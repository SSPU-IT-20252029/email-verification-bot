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

type VerifiedUser struct {
	DiscordID  string
	Email      string
	Role       string
	VerifiedAt time.Time
}

type Pending struct {
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
			return nil, fmt.Errorf("vytváření adresáře databáze: %w", err)
		}
	}
	db, err := sql.Open("sqlite", dsn+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("otevírání databáze: %w", err)
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
CREATE TABLE IF NOT EXISTS verified_users (
	discord_id  TEXT PRIMARY KEY,
	email       TEXT NOT NULL,
	role        TEXT NOT NULL,
	verified_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS pending_codes (
	discord_id TEXT PRIMARY KEY,
	email      TEXT NOT NULL,
	code_hash  TEXT NOT NULL,
	expires_at INTEGER NOT NULL,
	attempts   INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS send_log (
	discord_id TEXT NOT NULL,
	sent_at    INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_send_log_user_time ON send_log(discord_id, sent_at);
CREATE TABLE IF NOT EXISTS meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
`
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("migrace databáze: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) SetVerified(ctx context.Context, discordID, email, role string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO verified_users (discord_id, email, role, verified_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(discord_id) DO UPDATE SET email = excluded.email, role = excluded.role, verified_at = excluded.verified_at`,
		discordID, email, role, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("ukládání ověřeného uživatele: %w", err)
	}
	return nil
}

func (s *Store) GetVerified(ctx context.Context, discordID string) (VerifiedUser, bool, error) {
	var v VerifiedUser
	var at int64
	err := s.db.QueryRowContext(ctx,
		`SELECT discord_id, email, role, verified_at FROM verified_users WHERE discord_id = ?`, discordID).
		Scan(&v.DiscordID, &v.Email, &v.Role, &at)
	if err == sql.ErrNoRows {
		return VerifiedUser{}, false, nil
	}
	if err != nil {
		return VerifiedUser{}, false, fmt.Errorf("čtení ověřeného uživatele: %w", err)
	}
	v.VerifiedAt = time.Unix(at, 0)
	return v, true, nil
}

func (s *Store) GetVerifiedByEmail(ctx context.Context, email string) (VerifiedUser, bool, error) {
	var v VerifiedUser
	var at int64
	err := s.db.QueryRowContext(ctx,
		`SELECT discord_id, email, role, verified_at FROM verified_users WHERE email = ?`, email).
		Scan(&v.DiscordID, &v.Email, &v.Role, &at)
	if err == sql.ErrNoRows {
		return VerifiedUser{}, false, nil
	}
	if err != nil {
		return VerifiedUser{}, false, fmt.Errorf("čtení ověřeného uživatele podle e-mailu: %w", err)
	}
	v.VerifiedAt = time.Unix(at, 0)
	return v, true, nil
}

func (s *Store) ListVerified(ctx context.Context) ([]VerifiedUser, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT discord_id, email, role, verified_at FROM verified_users`)
	if err != nil {
		return nil, fmt.Errorf("výpis ověřených uživatelů: %w", err)
	}
	defer rows.Close()
	var out []VerifiedUser
	for rows.Next() {
		var v VerifiedUser
		var at int64
		if err := rows.Scan(&v.DiscordID, &v.Email, &v.Role, &at); err != nil {
			return nil, fmt.Errorf("čtení ověřeného uživatele: %w", err)
		}
		v.VerifiedAt = time.Unix(at, 0)
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) UpsertPending(ctx context.Context, p Pending) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO pending_codes (discord_id, email, code_hash, expires_at, attempts) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(discord_id) DO UPDATE SET email = excluded.email, code_hash = excluded.code_hash,
		   expires_at = excluded.expires_at, attempts = excluded.attempts`,
		p.DiscordID, p.Email, p.CodeHash, p.ExpiresAt.Unix(), p.Attempts)
	if err != nil {
		return fmt.Errorf("ukládání čekajícího kódu: %w", err)
	}
	return nil
}

func (s *Store) GetPending(ctx context.Context, discordID string) (Pending, bool, error) {
	var p Pending
	var exp int64
	err := s.db.QueryRowContext(ctx,
		`SELECT discord_id, email, code_hash, expires_at, attempts FROM pending_codes WHERE discord_id = ?`, discordID).
		Scan(&p.DiscordID, &p.Email, &p.CodeHash, &exp, &p.Attempts)
	if err == sql.ErrNoRows {
		return Pending{}, false, nil
	}
	if err != nil {
		return Pending{}, false, fmt.Errorf("čtení čekajícího kódu: %w", err)
	}
	p.ExpiresAt = time.Unix(exp, 0)
	return p, true, nil
}

func (s *Store) DeletePending(ctx context.Context, discordID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM pending_codes WHERE discord_id = ?`, discordID)
	if err != nil {
		return fmt.Errorf("mazání čekajícího kódu: %w", err)
	}
	return nil
}

func (s *Store) IncrementAttempts(ctx context.Context, discordID string) (int, error) {
	var attempts int
	err := s.db.QueryRowContext(ctx,
		`UPDATE pending_codes SET attempts = attempts + 1 WHERE discord_id = ? RETURNING attempts`, discordID).
		Scan(&attempts)
	if err != nil {
		return 0, fmt.Errorf("zvyšování počtu pokusů: %w", err)
	}
	return attempts, nil
}

func (s *Store) LogSend(ctx context.Context, discordID string, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO send_log (discord_id, sent_at) VALUES (?, ?)`, discordID, at.Unix())
	if err != nil {
		return fmt.Errorf("zápis do send_log: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM send_log WHERE sent_at < ?`, at.Add(-2*time.Hour).Unix())
	if err != nil {
		return fmt.Errorf("úklid send_log: %w", err)
	}
	return nil
}

func (s *Store) CountSendsSince(ctx context.Context, discordID string, since time.Time) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM send_log WHERE discord_id = ? AND sent_at >= ?`, discordID, since.Unix()).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("čtení send_log: %w", err)
	}
	return n, nil
}

func (s *Store) GetMeta(ctx context.Context, key string) (string, bool, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("čtení meta %q: %w", key, err)
	}
	return v, true, nil
}

func (s *Store) SetMeta(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO meta (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value)
	if err != nil {
		return fmt.Errorf("ukládání meta %q: %w", key, err)
	}
	return nil
}
