package config

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"

	"sspu-verifier/internal/class"
)

type Config struct {
	Discord      Discord      `yaml:"discord"`
	Email        Email        `yaml:"email"`
	Roles        Roles        `yaml:"roles"`
	Verification Verification `yaml:"verification"`
	Storage      Storage      `yaml:"storage"`
}

type Discord struct {
	Token           string `yaml:"token"`
	GuildID         string `yaml:"guild_id"`
	VerifyChannelID string `yaml:"verify_channel_id"`
}

type Email struct {
	APIKey  string `yaml:"api_key"`
	From    string `yaml:"from"`
	Subject string `yaml:"subject"`
}

type Roles struct {
	IDs map[string]string `yaml:"ids"`
}

type Verification struct {
	CodeTTL     time.Duration `yaml:"code_ttl"`
	MaxAttempts int           `yaml:"max_attempts"`
	HourlyLimit int           `yaml:"rate_limit_per_hour"`
}

type Storage struct {
	DSN string `yaml:"dsn"`
}

var envRe = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("čtení konfigurace: %w", err)
	}
	expanded := envRe.ReplaceAllFunc(raw, func(m []byte) []byte {
		name := envRe.FindSubmatch(m)[1]
		return []byte(os.Getenv(string(name)))
	})
	var cfg Config
	if err := yaml.Unmarshal(expanded, &cfg); err != nil {
		return nil, fmt.Errorf("parsování konfigurace: %w", err)
	}
	applyDefaults(&cfg)
	if err := validate(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Email.Subject == "" {
		cfg.Email.Subject = "Discord ověření — ověřovací kód"
	}
	if cfg.Verification.CodeTTL == 0 {
		cfg.Verification.CodeTTL = 10 * time.Minute
	}
	if cfg.Verification.MaxAttempts == 0 {
		cfg.Verification.MaxAttempts = 5
	}
	if cfg.Verification.HourlyLimit == 0 {
		cfg.Verification.HourlyLimit = 3
	}
	if cfg.Storage.DSN == "" {
		cfg.Storage.DSN = "./data/verifier.db"
	}
}

func validate(cfg *Config) error {
	if cfg.Discord.Token == "" {
		return fmt.Errorf("discord.token je povinný")
	}
	if cfg.Discord.GuildID == "" {
		return fmt.Errorf("discord.guild_id je povinný")
	}
	if cfg.Discord.VerifyChannelID == "" {
		return fmt.Errorf("discord.verify_channel_id je povinný")
	}
	if cfg.Email.APIKey == "" {
		return fmt.Errorf("email.api_key je povinný")
	}
	if cfg.Email.From == "" {
		return fmt.Errorf("email.from je povinný")
	}
	if cfg.Roles.IDs == nil {
		return fmt.Errorf("roles.ids je povinné")
	}
	for _, name := range class.AllRoleNames() {
		if cfg.Roles.IDs[name] == "" {
			return fmt.Errorf("roles.ids: chybí ID role %q", name)
		}
	}
	return nil
}
