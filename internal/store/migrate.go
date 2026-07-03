package store

import (
	"fmt"
	"strings"
)

func (s *Store) migrate() error {
	stmts := []string{
		`ALTER TABLE users ADD COLUMN totp_secret TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN totp_enabled INTEGER NOT NULL DEFAULT 0`,
		`CREATE TABLE IF NOT EXISTS login_pending (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at TIMESTAMP NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS access_log_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			recorded_at TIMESTAMP NOT NULL,
			host TEXT NOT NULL DEFAULT '',
			method TEXT NOT NULL DEFAULT '',
			path TEXT NOT NULL DEFAULT '',
			status INTEGER NOT NULL DEFAULT 0,
			bytes INTEGER NOT NULL DEFAULT 0,
			remote_ip TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS analytics_state (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS panel_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT ''
		)`,
		`ALTER TABLE users ADD COLUMN files_subdir TEXT NOT NULL DEFAULT ''`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("migrate %q: %w", stmt, err)
		}
	}
	return nil
}
