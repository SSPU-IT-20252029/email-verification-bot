package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestGetVerifiedByEmail(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()

	// email not found
	if _, ok, err := s.GetVerifiedByEmail(ctx, "x@y.z"); err != nil || ok {
		t.Fatalf("neočekávaný výsledek: ok=%v err=%v", ok, err)
	}

	// insert
	if err := s.SetVerified(ctx, "d1", "a@b", "IT1"); err != nil {
		t.Fatal(err)
	}
	v, ok, err := s.GetVerifiedByEmail(ctx, "a@b")
	if err != nil || !ok || v.DiscordID != "d1" {
		t.Fatalf("GetVerifiedByEmail = %+v ok=%v err=%v", v, ok, err)
	}

	// other email still not found
	if _, ok, err := s.GetVerifiedByEmail(ctx, "c@d"); err != nil || ok {
		t.Fatalf("neočekávaný výsledek: ok=%v err=%v", ok, err)
	}
}

func TestMeta(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()

	if _, ok, err := s.GetMeta(ctx, "k"); err != nil || ok {
		t.Fatalf("chybějící klíč: ok=%v err=%v", ok, err)
	}
	if err := s.SetMeta(ctx, "k", "v1"); err != nil {
		t.Fatal(err)
	}
	v, ok, err := s.GetMeta(ctx, "k")
	if err != nil || !ok || v != "v1" {
		t.Fatalf("GetMeta = %q ok=%v err=%v", v, ok, err)
	}
	if err := s.SetMeta(ctx, "k", "v2"); err != nil {
		t.Fatal(err)
	}
	if v, _, _ := s.GetMeta(ctx, "k"); v != "v2" {
		t.Fatalf("přepsání selhalo, v = %q", v)
	}
}
