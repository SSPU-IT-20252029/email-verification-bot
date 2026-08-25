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

type Verification struct {
	CodeTTL     time.Duration `yaml:"code_ttl"`
	MaxAttempts int           `yaml:"max_attempts"`
	HourlyLimit int           `yaml:"rate_limit_per_hour"`
}

type Storage struct {
	DSN string `yaml:"dsn"`
}

// Roles umožňuje přejmenovat Discord role. Sekce je nepovinná —
// bez ní se používají výchozí názvy (IT1…, Sv1A…, Absolvent).
type Roles struct {
	Absolvent string              `yaml:"absolvent"`
	Names     map[string][]string `yaml:"names"` // klíč it/uo/sva/svb → 4 názvy od 1. ročníku
}

// DisplayName vrací zobrazovaný název role pro kanonický název
// ("IT2" → nastavený název 2. ročníku oboru it).
func (r Roles) DisplayName(canonical string) string {
	for _, t := range class.ChainTypes() {
		for g := 1; g <= 4; g++ {
			if canonical == class.RoleName(t, g) {
				return r.Names[string(t)][g-1]
			}
		}
	}
	if canonical == class.RoleAbsolvent {
		return r.Absolvent
	}
	return canonical
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
	if cfg.Roles.Absolvent == "" {
		cfg.Roles.Absolvent = class.RoleAbsolvent
	}
	if cfg.Roles.Names == nil {
		cfg.Roles.Names = make(map[string][]string, len(class.ChainTypes()))
	}
	for _, t := range class.ChainTypes() {
		if len(cfg.Roles.Names[string(t)]) == 0 {
			names := make([]string, 4)
			for g := 1; g <= 4; g++ {
				names[g-1] = class.RoleName(t, g)
			}
			cfg.Roles.Names[string(t)] = names
		}
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
	seen := map[string]string{cfg.Roles.Absolvent: "roles.absolvent"}
	for _, t := range class.ChainTypes() {
		names := cfg.Roles.Names[string(t)]
		if len(names) != 4 {
			return fmt.Errorf("roles.names.%s: očekávám 4 názvy rolí (1.–4. ročník)", t)
		}
		for g, name := range names {
			if name == "" {
				return fmt.Errorf("roles.names.%s: %d. ročník má prázdný název", t, g+1)
			}
			if prev, dup := seen[name]; dup {
				return fmt.Errorf("roles.names: duplicitní název role %q (%s a %s)", name, prev, t)
			}
			seen[name] = string(t)
		}
	}
	return nil
}
