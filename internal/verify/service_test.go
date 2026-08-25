package verify

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"sspu-verifier/internal/class"
	"sspu-verifier/internal/store"
)

type fakeMailer struct {
	sent    int
	lastTo  string
	lastCod string
	fail    bool
}

func (f *fakeMailer) SendCode(to, code string, ttl time.Duration) error {
	if f.fail {
		return errors.New("smtp down")
	}
	f.sent++
	f.lastTo = to
	f.lastCod = code
	return nil
}

func newTestService(t *testing.T, m Mailer) *Service {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return New(st, m, 10*time.Minute, 5, 3)
}

var october2026 = time.Date(2026, time.October, 15, 12, 0, 0, 0, time.UTC)

func TestStartAndConfirmHappyPath(t *testing.T) {
	fm := &fakeMailer{}
	svc := newTestService(t, fm)
	svc.Now = func() time.Time { return october2026 }
	ctx := context.Background()

	if err := svc.Start(ctx, "user1", "it2501@sspu-opava.cz"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if fm.sent != 1 || fm.lastTo != "it2501@sspu-opava.cz" {
		t.Fatalf("mailer got %+v", fm)
	}
	role, err := svc.Confirm(ctx, "user1", fm.lastCod)
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if role != "IT2" {
		t.Fatalf("role = %q, want IT2", role)
	}
	v, ok, err := svc.store.GetVerified(ctx, "user1")
	if err != nil || !ok {
		t.Fatalf("GetVerified: ok=%v err=%v", ok, err)
	}
	if v.Role != "IT2" || v.Email != "it2501@sspu-opava.cz" {
		t.Fatalf("verified user = %+v", v)
	}
}

func TestStartRejectsBadInput(t *testing.T) {
	fm := &fakeMailer{}
	svc := newTestService(t, fm)
	svc.Now = func() time.Time { return october2026 }
	ctx := context.Background()

	if err := svc.Start(ctx, "u", "it2601@gmail.com"); !errors.Is(err, class.ErrDomain) {
		t.Fatalf("wrong domain err = %v", err)
	}
	if err := svc.Start(ctx, "u", "zz1234@sspu-opava.cz"); !errors.Is(err, class.ErrFormat) {
		t.Fatalf("bad format err = %v", err)
	}
	if fm.sent != 0 {
		t.Fatalf("mailer should not have sent, sent=%d", fm.sent)
	}
}

func TestStartRejectsNotYetActiveMail(t *testing.T) {
	fm := &fakeMailer{}
	svc := newTestService(t, fm)
	june2026 := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	svc.Now = func() time.Time { return june2026 }

	if err := svc.Start(context.Background(), "u", "it2601@sspu-opava.cz"); !errors.Is(err, ErrNotActive) {
		t.Fatalf("err = %v, want ErrNotActive", err)
	}
	if fm.sent != 0 {
		t.Fatalf("mailer should not have sent")
	}
}

func TestRateLimit(t *testing.T) {
	fm := &fakeMailer{}
	svc := newTestService(t, fm)
	now := october2026
	svc.Now = func() time.Time { return now }
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := svc.Start(ctx, "user1", "it2501@sspu-opava.cz"); err != nil {
			t.Fatalf("Start #%d: %v", i, err)
		}
	}
	if err := svc.Start(ctx, "user1", "it2501@sspu-opava.cz"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v, want ErrRateLimited", err)
	}
	now = now.Add(2 * time.Hour)
	if err := svc.Start(ctx, "user1", "it2501@sspu-opava.cz"); err != nil {
		t.Fatalf("Start after window: %v", err)
	}
}

func TestWrongCodeAndAttempts(t *testing.T) {
	fm := &fakeMailer{}
	svc := newTestService(t, fm)
	svc.Now = func() time.Time { return october2026 }
	ctx := context.Background()

	if err := svc.Start(ctx, "user1", "it2501@sspu-opava.cz"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	rightCode := fm.lastCod
	for i := 1; i <= 4; i++ {
		_, err := svc.Confirm(ctx, "user1", "000000")
		var wc *WrongCodeError
		if !errors.As(err, &wc) {
			t.Fatalf("attempt %d err = %v, want WrongCodeError", i, err)
		}
		if wc.Remaining != 5-i {
			t.Fatalf("attempt %d remaining = %d, want %d", i, wc.Remaining, 5-i)
		}
	}
	if _, err := svc.Confirm(ctx, "user1", "000000"); !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("err = %v, want ErrTooManyAttempts", err)
	}
	if _, ok, _ := svc.store.GetPending(ctx, "user1"); ok {
		t.Fatalf("pending should be deleted after too many attempts")
	}
	if err := svc.Start(ctx, "user1", "it2501@sspu-opava.cz"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := svc.Confirm(ctx, "user1", rightCode); err == nil {
		t.Fatalf("old code must not work after re-request")
	}
}

func TestExpiredCode(t *testing.T) {
	fm := &fakeMailer{}
	svc := newTestService(t, fm)
	now := october2026
	svc.Now = func() time.Time { return now }
	ctx := context.Background()

	if err := svc.Start(ctx, "user1", "it2501@sspu-opava.cz"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	code := fm.lastCod
	now = now.Add(11 * time.Minute)
	if _, err := svc.Confirm(ctx, "user1", code); !errors.Is(err, ErrExpired) {
		t.Fatalf("err = %v, want ErrExpired", err)
	}
}

func TestConfirmWithoutPending(t *testing.T) {
	fm := &fakeMailer{}
	svc := newTestService(t, fm)
	svc.Now = func() time.Time { return october2026 }
	if _, err := svc.Confirm(context.Background(), "ghost", "123456"); !errors.Is(err, ErrNoPending) {
		t.Fatalf("err = %v, want ErrNoPending", err)
	}
}

func TestSendFailureKeepsStateConsistent(t *testing.T) {
	fm := &fakeMailer{fail: true}
	svc := newTestService(t, fm)
	svc.Now = func() time.Time { return october2026 }
	err := svc.Start(context.Background(), "user1", "it2501@sspu-opava.cz")
	if !errors.Is(err, ErrSendFailed) {
		t.Fatalf("err = %v, want ErrSendFailed", err)
	}
}
