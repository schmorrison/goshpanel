package store

import (
	"database/sql"
	"errors"
	"time"
)

// LoginPending is a short-lived token between password and TOTP verification.
type LoginPending struct {
	Token     string
	UserID    int64
	ExpiresAt time.Time
}

// CreateLoginPending stores a pending MFA login.
func (s *Store) CreateLoginPending(token string, userID int64, expiresAt time.Time) error {
	_, err := s.db.Exec(`INSERT INTO login_pending (token, user_id, expires_at) VALUES (?, ?, ?)`, token, userID, expiresAt.UTC())
	return err
}

// ConsumeLoginPending returns and deletes a pending login if valid.
func (s *Store) ConsumeLoginPending(token string) (LoginPending, error) {
	var p LoginPending
	err := s.db.QueryRow(`SELECT token, user_id, expires_at FROM login_pending WHERE token = ?`, token).
		Scan(&p.Token, &p.UserID, &p.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return LoginPending{}, ErrNotFound
	}
	if err != nil {
		return LoginPending{}, err
	}
	_, _ = s.db.Exec(`DELETE FROM login_pending WHERE token = ?`, token)
	if time.Now().UTC().After(p.ExpiresAt) {
		return LoginPending{}, ErrNotFound
	}
	return p, nil
}

// PruneLoginPending removes expired pending logins.
func (s *Store) PruneLoginPending() error {
	_, err := s.db.Exec(`DELETE FROM login_pending WHERE expires_at < ?`, now())
	return err
}
