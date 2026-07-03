// Package store persists panel state in SQLite via modernc.org/sqlite,
// a pure-Go driver (no cgo). All panel modules share this single database.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("record not found")

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	username      TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	role          TEXT NOT NULL DEFAULT 'user',
	created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS sessions (
	token      TEXT PRIMARY KEY,
	user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	csrf_token TEXT NOT NULL,
	expires_at TIMESTAMP NOT NULL
);
CREATE TABLE IF NOT EXISTS domains (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	name       TEXT NOT NULL UNIQUE,
	root       TEXT NOT NULL DEFAULT '',
	upstream   TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS dns_records (
	id        INTEGER PRIMARY KEY AUTOINCREMENT,
	domain_id INTEGER NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
	type      TEXT NOT NULL,
	name      TEXT NOT NULL,
	value     TEXT NOT NULL,
	ttl       INTEGER NOT NULL DEFAULT 3600,
	priority  INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS mailboxes (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	address       TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	quota_mb      INTEGER NOT NULL DEFAULT 0,
	created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS forwarders (
	id        INTEGER PRIMARY KEY AUTOINCREMENT,
	from_addr TEXT NOT NULL,
	to_addr   TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS cron_jobs (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	schedule   TEXT NOT NULL,
	command    TEXT NOT NULL,
	comment    TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS db_connections (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	name       TEXT NOT NULL UNIQUE,
	driver     TEXT NOT NULL,
	dsn        TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS audit_log (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	username   TEXT NOT NULL,
	action     TEXT NOT NULL,
	detail     TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS ip_rules (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	cidr       TEXT NOT NULL UNIQUE,
	comment    TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

// Store wraps the SQLite database shared by all panel modules.
type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the SQLite database at path and applies
// the schema.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// modernc.org/sqlite serializes writes; a single connection avoids
	// SQLITE_BUSY under concurrent handlers.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the underlying database.
func (s *Store) Close() error { return s.db.Close() }

// DB exposes the raw handle for tests.
func (s *Store) DB() *sql.DB { return s.db }

// now returns the current UTC time truncated to seconds for stable storage.
func now() time.Time { return time.Now().UTC().Truncate(time.Second) }
