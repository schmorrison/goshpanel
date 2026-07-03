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
CREATE TABLE IF NOT EXISTS micro_functions (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	name        TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL DEFAULT '',
	script      TEXT NOT NULL,
	token       TEXT NOT NULL DEFAULT '',
	enabled     INTEGER NOT NULL DEFAULT 1,
	timeout_sec INTEGER NOT NULL DEFAULT 30,
	created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS docker_stacks (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	name         TEXT NOT NULL UNIQUE,
	compose_yaml TEXT NOT NULL,
	created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS systemd_units (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	name         TEXT NOT NULL UNIQUE,
	unit_content TEXT NOT NULL,
	enabled      INTEGER NOT NULL DEFAULT 1,
	created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS orchestrator_status (
	component      TEXT PRIMARY KEY,
	config_path    TEXT NOT NULL DEFAULT '',
	last_applied   TIMESTAMP,
	last_error     TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS fleet_nodes (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	name          TEXT NOT NULL UNIQUE,
	base_url      TEXT NOT NULL,
	token         TEXT NOT NULL,
	enabled       INTEGER NOT NULL DEFAULT 1,
	last_seen_at  TIMESTAMP,
	last_error    TEXT NOT NULL DEFAULT '',
	created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS fleet_telemetry (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	node_id    INTEGER NOT NULL REFERENCES fleet_nodes(id) ON DELETE CASCADE,
	payload    TEXT NOT NULL,
	recorded_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS fleet_commands (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	node_id    INTEGER NOT NULL REFERENCES fleet_nodes(id) ON DELETE CASCADE,
	action     TEXT NOT NULL,
	status     TEXT NOT NULL DEFAULT 'pending',
	result     TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	completed_at TIMESTAMP
);
CREATE TABLE IF NOT EXISTS metric_samples (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	load1         REAL NOT NULL,
	mem_used_pct  REAL NOT NULL,
	disk_used_pct REAL NOT NULL,
	recorded_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
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
	st := &Store{db: db}
	if err := st.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return st, nil
}

// Close closes the underlying database.
func (s *Store) Close() error { return s.db.Close() }

// DB exposes the raw handle for tests.
func (s *Store) DB() *sql.DB { return s.db }

// now returns the current UTC time truncated to seconds for stable storage.
func now() time.Time { return time.Now().UTC().Truncate(time.Second) }
