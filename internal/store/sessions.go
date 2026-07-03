package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Session is a login session keyed by an opaque token.
type Session struct {
	Token     string
	UserID    int64
	CSRFToken string
	ExpiresAt time.Time
}

// CreateSession stores a new session.
func (s *Store) CreateSession(sess Session) error {
	_, err := s.db.Exec(
		`INSERT INTO sessions (token, user_id, csrf_token, expires_at) VALUES (?, ?, ?, ?)`,
		sess.Token, sess.UserID, sess.CSRFToken, sess.ExpiresAt.UTC())
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// SessionByToken returns a non-expired session for token.
func (s *Store) SessionByToken(token string) (Session, error) {
	var sess Session
	err := s.db.QueryRow(
		`SELECT token, user_id, csrf_token, expires_at FROM sessions WHERE token = ? AND expires_at > ?`,
		token, now()).Scan(&sess.Token, &sess.UserID, &sess.CSRFToken, &sess.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	return sess, err
}

// DeleteSession removes a session (logout).
func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// PruneSessions deletes expired sessions.
func (s *Store) PruneSessions() error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, now())
	return err
}
