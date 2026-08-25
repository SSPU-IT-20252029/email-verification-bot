package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sspu-verifier/internal/class"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

var validRoles = func() string {
	var b strings.Builder
	b.WriteString("roles:\n  ids:\n")
	for _, name := range class.AllRoleNames() {
		b.WriteString("    " + name + ": \"111\"\n")
	}
	return b.String()
}()

func baseConfig() string {
	return `
discord:
  token: "tok"
  guild_id: "123"
  verify_channel_id: "456"
email:
  api_key: "re_test_123"
  from: "bot@sspu-opava.cz"
` + validRoles
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

func TestLoadMissingRole(t *testing.T) {
	path := writeConfig(t, `
discord:
  token: "tok"
  guild_id: "1"
  verify_channel_id: "2"
email:
  api_key: "re_test_123"
  from: "a@b.cz"
roles:
  ids:
    IT1: "111"
`)
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "Uo1") {
		t.Fatalf("err = %v, want missing Uo1", err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yml")); err == nil {
		t.Fatal("expected error for missing file")
	}
}
