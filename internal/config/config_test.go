package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func baseConfig() string {
	return `
discord:
  token: "tok"
  guild_id: "123"
  verify_channel_id: "456"
email:
  api_key: "re_test_123"
  from: "bot@sspu-opava.cz"
`
}

func TestLoadValid(t *testing.T) {
	cfg, err := Load(writeConfig(t, baseConfig()))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Verification.CodeTTL.Minutes() != 10 || cfg.Verification.MaxAttempts != 5 || cfg.Verification.HourlyLimit != 3 {
		t.Fatalf("defaults not applied: %+v", cfg.Verification)
	}
	if cfg.Storage.DSN == "" || cfg.Email.APIKey == "" {
		t.Fatalf("defaults not applied: %+v", cfg)
	}
}

func TestLoadEnvExpansion(t *testing.T) {
	t.Setenv("DISCORD_TOKEN", "secret-token")
	path := writeConfig(t, strings.Replace(baseConfig(), "\"tok\"", "${DISCORD_TOKEN}", 1))
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Discord.Token != "secret-token" {
		t.Fatalf("token = %q", cfg.Discord.Token)
	}
}

func TestLoadMissingChannel(t *testing.T) {
	path := writeConfig(t, `
discord:
  token: "tok"
  guild_id: "1"
email:
  api_key: "re_x"
  from: "a@b.cz"
`)
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "verify_channel_id") {
		t.Fatalf("err = %v, want missing verify_channel_id", err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yml")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestDefaultRoleNames(t *testing.T) {
	cfg, err := Load(writeConfig(t, baseConfig()))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Roles.Absolvent != "Absolvent" {
		t.Fatalf("absolvent = %q", cfg.Roles.Absolvent)
	}
	if got := cfg.Roles.DisplayName("IT2"); got != "IT2" {
		t.Fatalf("DisplayName(IT2) = %q", got)
	}
	if got := cfg.Roles.DisplayName("Sv3B"); got != "Sv3B" {
		t.Fatalf("DisplayName(Sv3B) = %q", got)
	}
}

func TestCustomRoleNames(t *testing.T) {
	path := writeConfig(t, baseConfig()+`
roles:
  absolvent: "Maturanti"
  names:
    it: ["Prima IT", "Sekunda IT", "Tercie IT", "Kvarta IT"]
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Roles.DisplayName("IT1"); got != "Prima IT" {
		t.Fatalf("DisplayName(IT1) = %q", got)
	}
	if got := cfg.Roles.DisplayName("IT4"); got != "Kvarta IT" {
		t.Fatalf("DisplayName(IT4) = %q", got)
	}
	if got := cfg.Roles.DisplayName("Absolvent"); got != "Maturanti" {
		t.Fatalf("DisplayName(Absolvent) = %q", got)
	}
	if got := cfg.Roles.DisplayName("Uo1"); got != "Uo1" {
		t.Fatalf("nezměněný obor: DisplayName(Uo1) = %q", got)
	}
}

func TestDuplicateRoleNamesRejected(t *testing.T) {
	path := writeConfig(t, baseConfig()+`
roles:
  names:
    it: ["X", "Y", "Z", "W"]
    uo: ["X", "Uo2", "Uo3", "Uo4"]
`)
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "duplicitní") {
		t.Fatalf("err = %v, want duplicitní název", err)
	}
}

func TestWrongRoleNameCountRejected(t *testing.T) {
	path := writeConfig(t, baseConfig()+`
roles:
  names:
    sva: ["Sv1A", "Sv2A", "Sv3A"]
`)
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "4 názvy") {
		t.Fatalf("err = %v, want 4 názvy", err)
	}
}
