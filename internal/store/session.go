package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/alexedwards/scs/v2"
)

// SessionStore persists session data in SQLite.
type SessionStore struct {
	db *sql.DB
}

// NewSessionStore returns a SQLite-backed scs store.
func NewSessionStore(db *sql.DB) *SessionStore {
	return &SessionStore{db: db}
}

// Delete implements scs.Store.
func (s *SessionStore) Delete(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// Find implements scs.Store.
func (s *SessionStore) Find(token string) ([]byte, bool, error) {
	var data []byte
	var expiry string
	err := s.db.QueryRow(`
		SELECT data, expiry
		FROM sessions
		WHERE token = ?
	`, token).Scan(&data, &expiry)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	expiresAt, err := time.Parse(time.RFC3339Nano, expiry)
	if err != nil {
		return nil, false, err
	}
	if time.Now().After(expiresAt) {
		_ = s.Delete(token)
		return nil, false, nil
	}

	return data, true, nil
}

// Commit implements scs.Store.
func (s *SessionStore) Commit(token string, data []byte, expiry time.Time) error {
	_, err := s.db.Exec(`
		INSERT INTO sessions (token, data, expiry)
		VALUES (?, ?, ?)
		ON CONFLICT(token) DO UPDATE SET data = excluded.data, expiry = excluded.expiry
	`, token, data, expiry.UTC().Format(time.RFC3339Nano))
	return err
}

// DeleteAll implements scs.Store.
func (s *SessionStore) DeleteAll() error {
	_, err := s.db.Exec(`DELETE FROM sessions`)
	return err
}

// All implements scs.Store.
func (s *SessionStore) All() (map[string][]byte, error) {
	rows, err := s.db.Query(`
		SELECT token, data, expiry
		FROM sessions
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := make(map[string][]byte)
	now := time.Now()
	for rows.Next() {
		var token string
		var data []byte
		var expiry string
		if err := rows.Scan(&token, &data, &expiry); err != nil {
			return nil, err
		}

		expiresAt, err := time.Parse(time.RFC3339Nano, expiry)
		if err != nil {
			return nil, err
		}
		if now.After(expiresAt) {
			_ = s.Delete(token)
			continue
		}

		sessions[token] = data
	}

	return sessions, rows.Err()
}

// PurgeExpired removes expired sessions from the database.
func (s *SessionStore) PurgeExpired(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT token, expiry FROM sessions`)
	if err != nil {
		return err
	}
	defer rows.Close()

	now := time.Now()
	for rows.Next() {
		var token string
		var expiry string
		if err := rows.Scan(&token, &expiry); err != nil {
			return err
		}

		expiresAt, err := time.Parse(time.RFC3339Nano, expiry)
		if err != nil {
			return err
		}
		if now.After(expiresAt) {
			if err := s.Delete(token); err != nil {
				return err
			}
		}
	}

	return rows.Err()
}

var _ scs.Store = (*SessionStore)(nil)
