package verify

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"sspu-verifier/internal/class"
	"sspu-verifier/internal/store"
)

var (
	ErrNotActive        = errors.New("verify: e-mail zatím není aktivní, školní rok pro něj ještě nezačal")
	ErrRateLimited      = errors.New("verify: překročen limit odeslaných kódů, zkus to později")
	ErrNoPending        = errors.New("verify: žádný čekající kód, použij nejdřív /verify")
	ErrExpired          = errors.New("verify: kód vypršel")
	ErrTooManyAttempts  = errors.New("verify: příliš mnoho pokusů")
	ErrSendFailed       = errors.New("verify: odeslání e-mailu selhalo")
	ErrEmailAlreadyUsed = errors.New("verify: e-mail je přiřazen k jinému uživateli")
)

type WrongCodeError struct {
	Remaining int
}

func (e *WrongCodeError) Error() string {
	return "verify: nesprávný kód"
}

type Mailer interface {
	SendCode(to, code string, ttl time.Duration) error
}

type Service struct {
	store       *store.Store
	mailer      Mailer
	ttl         time.Duration
	maxAttempts int
	hourlyLimit int
	Now         func() time.Time
}

func New(st *store.Store, m Mailer, ttl time.Duration, maxAttempts, hourlyLimit int) *Service {
	return &Service{
		store:       st,
		mailer:      m,
		ttl:         ttl,
		maxAttempts: maxAttempts,
		hourlyLimit: hourlyLimit,
		Now:         time.Now,
	}
}

func (s *Service) Start(ctx context.Context, discordID, email string) error {
	mail, err := class.Parse(email)
	if err != nil {
		return err
	}
	now := s.Now()
	if _, ok := mail.Role(now); !ok {
		return ErrNotActive
	}
	if existing, ok, err := s.store.GetVerifiedByEmail(ctx, mail.String()); err == nil && ok && existing.DiscordID != discordID {
		return ErrEmailAlreadyUsed
	}
	sent, err := s.store.CountSendsSince(ctx, discordID, now.Add(-time.Hour))
	if err != nil {
		return err
	}
	if sent >= s.hourlyLimit {
		return ErrRateLimited
	}
	code, err := generateCode()
	if err != nil {
		return err
	}
	err = s.store.UpsertPending(ctx, store.Pending{
		DiscordID: discordID,
		Email:     mail.String(),
		CodeHash:  hash(code),
		ExpiresAt: now.Add(s.ttl),
		Attempts:  0,
	})
	if err != nil {
		return err
	}
	if err := s.mailer.SendCode(mail.String(), code, s.ttl); err != nil {
		return errors.Join(ErrSendFailed, err)
	}
	return s.store.LogSend(ctx, discordID, now)
}

func (s *Service) Confirm(ctx context.Context, discordID, code string) (string, error) {
	pending, ok, err := s.store.GetPending(ctx, discordID)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNoPending
	}
	now := s.Now()
	if now.After(pending.ExpiresAt) {
		if err := s.store.DeletePending(ctx, discordID); err != nil {
			return "", err
		}
		return "", ErrExpired
	}
	if subtle.ConstantTimeCompare([]byte(hash(normalizeCode(code))), []byte(pending.CodeHash)) != 1 {
		attempts, err := s.store.IncrementAttempts(ctx, discordID)
		if err != nil {
			return "", err
		}
		if attempts >= s.maxAttempts {
			if err := s.store.DeletePending(ctx, discordID); err != nil {
				return "", err
			}
			return "", ErrTooManyAttempts
		}
		return "", &WrongCodeError{Remaining: s.maxAttempts - attempts}
	}
	mail, err := class.Parse(pending.Email)
	if err != nil {
		return "", err
	}
	role, ok := mail.Role(now)
	if !ok {
		if err := s.store.DeletePending(ctx, discordID); err != nil {
			return "", err
		}
		return "", ErrNotActive
	}
	if err := s.store.SetVerified(ctx, discordID, pending.Email, role); err != nil {
		return "", err
	}
	if err := s.store.DeletePending(ctx, discordID); err != nil {
		return "", err
	}
	return role, nil
}

func generateCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", fmt.Errorf("generování kódu: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func normalizeCode(code string) string {
	out := make([]rune, 0, len(code))
	for _, r := range code {
		if r != ' ' {
			out = append(out, r)
		}
	}
	return string(out)
}

func hash(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}
