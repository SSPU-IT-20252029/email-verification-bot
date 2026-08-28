package config

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Discord Discord `yaml:"discord"`
	Email   Email   `yaml:"email"`
	Storage Storage `yaml:"storage"`
}

type Discord struct {
	Token string `yaml:"token"`
}

type Email struct {
	APIKey string `yaml:"api_key"`
	From   string `yaml:"from"`
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
	if cfg.Storage.DSN == "" {
		cfg.Storage.DSN = "./data/verifier.db"
	}
}

func validate(cfg *Config) error {
	if cfg.Discord.Token == "" {
		return fmt.Errorf("discord.token je povinný")
	}
	if cfg.Email.APIKey == "" {
		return fmt.Errorf("email.api_key je povinný")
	}
	if cfg.Email.From == "" {
		return fmt.Errorf("email.from je povinný")
	}
	return nil
}
