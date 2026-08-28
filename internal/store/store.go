package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type GuildConfig struct {
	GuildID           string
	VerifyChannelID   string
	Domain            string
	Mode              string // 'REGEX' or 'CSV'
	Subject           string
	CodeTTL           time.Duration
	MaxAttempts       int
	RateLimitPerHour  int
}

type RegexRule struct {
	ID       int
	GuildID  string
	Pattern  string
	RoleID   string
	Priority int
}

type VerifiedUser struct {
	GuildID    string
	DiscordID  string
	Email      string
	RoleID     string
	VerifiedAt time.Time
}

type Pending struct {
	GuildID   string
	DiscordID string
	Email     string
	CodeHash  string
	ExpiresAt time.Time
	Attempts  int
}

type Store struct {
	db *sql.DB
}

func Open(dsn string) (*Store, error) {
	if dir := filepath.Dir(dsn); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("creating database directory: %w", err)
		}
	}
	db, err := sql.Open("sqlite", dsn+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS guilds (
	guild_id            TEXT PRIMARY KEY,
	verify_channel_id   TEXT,
	domain              TEXT,
	mode                TEXT,
	subject             TEXT,
	code_ttl            INTEGER,
	max_attempts        INTEGER,
	rate_limit_per_hour INTEGER
);
CREATE TABLE IF NOT EXISTS regex_rules (
	id       INTEGER PRIMARY KEY AUTOINCREMENT,
	guild_id TEXT REFERENCES guilds(guild_id) ON DELETE CASCADE,
	pattern  TEXT NOT NULL,
	role_id  TEXT NOT NULL,
	priority INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS csv_mappings (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	guild_id   TEXT REFERENCES guilds(guild_id) ON DELETE CASCADE,
	class_name TEXT NOT NULL,
	role_id    TEXT NOT NULL,
	UNIQUE(guild_id, class_name)
);
CREATE TABLE IF NOT EXISTS csv_emails (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	guild_id   TEXT REFERENCES guilds(guild_id) ON DELETE CASCADE,
	email      TEXT NOT NULL,
	class_name TEXT NOT NULL,
	UNIQUE(guild_id, email)
);
CREATE TABLE IF NOT EXISTS verified_users (
	guild_id    TEXT NOT NULL,
	discord_id  TEXT NOT NULL,
	email       TEXT NOT NULL,
	role_id     TEXT NOT NULL,
	verified_at INTEGER NOT NULL,
	PRIMARY KEY (guild_id, discord_id),
	UNIQUE (guild_id, email)
);
CREATE TABLE IF NOT EXISTS pending_codes (
	guild_id   TEXT NOT NULL,
	discord_id TEXT NOT NULL,
	email      TEXT NOT NULL,
	code_hash  TEXT NOT NULL,
	expires_at INTEGER NOT NULL,
	attempts   INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (guild_id, discord_id)
);
CREATE TABLE IF NOT EXISTS send_log (
	guild_id   TEXT NOT NULL,
	discord_id TEXT NOT NULL,
	sent_at    INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_send_log_user_time ON send_log(guild_id, discord_id, sent_at);
`
	// Enable foreign keys
	_, err := s.db.Exec("PRAGMA foreign_keys = ON;")
	if err != nil {
		return err
	}
	_, err = s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("database migration: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// Guild Config
func (s *Store) SaveGuildConfig(ctx context.Context, g GuildConfig) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO guilds (guild_id, verify_channel_id, domain, mode, subject, code_ttl, max_attempts, rate_limit_per_hour)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(guild_id) DO UPDATE SET 
		 verify_channel_id=excluded.verify_channel_id,
		 domain=excluded.domain,
		 mode=excluded.mode,
		 subject=excluded.subject,
		 code_ttl=excluded.code_ttl,
		 max_attempts=excluded.max_attempts,
		 rate_limit_per_hour=excluded.rate_limit_per_hour`,
		g.GuildID, g.VerifyChannelID, g.Domain, g.Mode, g.Subject, int64(g.CodeTTL), g.MaxAttempts, g.RateLimitPerHour)
	return err
}

func (s *Store) GetGuildConfig(ctx context.Context, guildID string) (GuildConfig, bool, error) {
	var g GuildConfig
	var ttl int64
	err := s.db.QueryRowContext(ctx,
		`SELECT guild_id, verify_channel_id, domain, mode, subject, code_ttl, max_attempts, rate_limit_per_hour
		 FROM guilds WHERE guild_id = ?`, guildID).
		Scan(&g.GuildID, &g.VerifyChannelID, &g.Domain, &g.Mode, &g.Subject, &ttl, &g.MaxAttempts, &g.RateLimitPerHour)
	if err == sql.ErrNoRows {
		return GuildConfig{}, false, nil
	}
	if err != nil {
		return GuildConfig{}, false, err
	}
	g.CodeTTL = time.Duration(ttl)
	return g, true, nil
}

func (s *Store) ListGuildConfigs(ctx context.Context) ([]GuildConfig, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT guild_id, verify_channel_id, domain, mode, subject, code_ttl, max_attempts, rate_limit_per_hour FROM guilds`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GuildConfig
	for rows.Next() {
		var g GuildConfig
		var ttl int64
		if err := rows.Scan(&g.GuildID, &g.VerifyChannelID, &g.Domain, &g.Mode, &g.Subject, &ttl, &g.MaxAttempts, &g.RateLimitPerHour); err != nil {
			return nil, err
		}
		g.CodeTTL = time.Duration(ttl)
		out = append(out, g)
	}
	return out, nil
}

// Regex Rules
func (s *Store) AddRegexRule(ctx context.Context, r RegexRule) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO regex_rules (guild_id, pattern, role_id, priority) VALUES (?, ?, ?, ?)`,
		r.GuildID, r.Pattern, r.RoleID, r.Priority)
	return err
}

func (s *Store) RemoveRegexRule(ctx context.Context, id int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM regex_rules WHERE id = ?`, id)
	return err
}

func (s *Store) ListRegexRules(ctx context.Context, guildID string) ([]RegexRule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, guild_id, pattern, role_id, priority FROM regex_rules WHERE guild_id = ? ORDER BY priority DESC`, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RegexRule
	for rows.Next() {
		var r RegexRule
		if err := rows.Scan(&r.ID, &r.GuildID, &r.Pattern, &r.RoleID, &r.Priority); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// CSV Data
func (s *Store) ClearCSVEmails(ctx context.Context, guildID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM csv_emails WHERE guild_id = ?`, guildID)
	return err
}

func (s *Store) InsertCSVEmail(ctx context.Context, guildID, email, className string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO csv_emails (guild_id, email, class_name) VALUES (?, ?, ?)
		 ON CONFLICT(guild_id, email) DO UPDATE SET class_name=excluded.class_name`,
		guildID, email, className)
	return err
}

func (s *Store) MapCSVClass(ctx context.Context, guildID, className, roleID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO csv_mappings (guild_id, class_name, role_id) VALUES (?, ?, ?)
		 ON CONFLICT(guild_id, class_name) DO UPDATE SET role_id=excluded.role_id`,
		guildID, className, roleID)
	return err
}

func (s *Store) UnmapCSVClass(ctx context.Context, guildID, className string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM csv_mappings WHERE guild_id = ? AND class_name = ?`, guildID, className)
	return err
}

func (s *Store) GetRoleByCSVEmail(ctx context.Context, guildID, email string) (string, bool, error) {
	var roleID string
	err := s.db.QueryRowContext(ctx,
		`SELECT m.role_id 
		 FROM csv_emails e 
		 JOIN csv_mappings m ON e.guild_id = m.guild_id AND e.class_name = m.class_name
		 WHERE e.guild_id = ? AND e.email = ?`, guildID, email).Scan(&roleID)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return roleID, true, nil
}

// Verified Users
func (s *Store) SetVerified(ctx context.Context, guildID, discordID, email, roleID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO verified_users (guild_id, discord_id, email, role_id, verified_at) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(guild_id, discord_id) DO UPDATE SET email = excluded.email, role_id = excluded.role_id, verified_at = excluded.verified_at`,
		guildID, discordID, email, roleID, time.Now().Unix())
	return err
}

func (s *Store) GetVerifiedByEmail(ctx context.Context, guildID, email string) (VerifiedUser, bool, error) {
	var v VerifiedUser
	var at int64
	err := s.db.QueryRowContext(ctx,
		`SELECT guild_id, discord_id, email, role_id, verified_at FROM verified_users WHERE guild_id = ? AND email = ?`, guildID, email).
		Scan(&v.GuildID, &v.DiscordID, &v.Email, &v.RoleID, &at)
	if err == sql.ErrNoRows {
		return VerifiedUser{}, false, nil
	}
	if err != nil {
		return VerifiedUser{}, false, err
	}
	v.VerifiedAt = time.Unix(at, 0)
	return v, true, nil
}

// Pending Codes
func (s *Store) UpsertPending(ctx context.Context, p Pending) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO pending_codes (guild_id, discord_id, email, code_hash, expires_at, attempts) VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(guild_id, discord_id) DO UPDATE SET email = excluded.email, code_hash = excluded.code_hash,
		   expires_at = excluded.expires_at, attempts = excluded.attempts`,
		p.GuildID, p.DiscordID, p.Email, p.CodeHash, p.ExpiresAt.Unix(), p.Attempts)
	return err
}

func (s *Store) GetPending(ctx context.Context, guildID, discordID string) (Pending, bool, error) {
	var p Pending
	var exp int64
	err := s.db.QueryRowContext(ctx,
		`SELECT guild_id, discord_id, email, code_hash, expires_at, attempts FROM pending_codes WHERE guild_id = ? AND discord_id = ?`, guildID, discordID).
		Scan(&p.GuildID, &p.DiscordID, &p.Email, &p.CodeHash, &exp, &p.Attempts)
	if err == sql.ErrNoRows {
		return Pending{}, false, nil
	}
	if err != nil {
		return Pending{}, false, err
	}
	p.ExpiresAt = time.Unix(exp, 0)
	return p, true, nil
}

func (s *Store) DeletePending(ctx context.Context, guildID, discordID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM pending_codes WHERE guild_id = ? AND discord_id = ?`, guildID, discordID)
	return err
}

func (s *Store) IncrementAttempts(ctx context.Context, guildID, discordID string) (int, error) {
	var attempts int
	err := s.db.QueryRowContext(ctx,
		`UPDATE pending_codes SET attempts = attempts + 1 WHERE guild_id = ? AND discord_id = ? RETURNING attempts`, guildID, discordID).
		Scan(&attempts)
	if err != nil {
		return 0, err
	}
	return attempts, nil
}

// Rate Limiting
func (s *Store) LogSend(ctx context.Context, guildID, discordID string, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO send_log (guild_id, discord_id, sent_at) VALUES (?, ?, ?)`, guildID, discordID, at.Unix())
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM send_log WHERE sent_at < ?`, at.Add(-2*time.Hour).Unix())
	return err
}

func (s *Store) CountSendsSince(ctx context.Context, guildID, discordID string, since time.Time) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM send_log WHERE guild_id = ? AND discord_id = ? AND sent_at >= ?`, guildID, discordID, since.Unix()).Scan(&n)
	return n, err
}
