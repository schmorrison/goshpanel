// Package auth implements login sessions, CSRF protection and user
// management on top of the store.
package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/schmorrison/goshpanel/internal/crypto"
	"github.com/schmorrison/goshpanel/internal/store"
)

// ErrInvalidCredentials is returned for a bad username/password pair.
var ErrInvalidCredentials = errors.New("invalid username or password")

// Service provides authentication operations.
type Service struct {
	store      *store.Store
	sessionTTL time.Duration
}

// New creates an auth service.
func New(st *store.Store, sessionTTL time.Duration) *Service {
	return &Service{store: st, sessionTTL: sessionTTL}
}

// Bootstrap creates the initial admin user when no users exist.
// The second return value reports whether a new user was created.
func (s *Service) Bootstrap(username, password string) (bool, error) {
	n, err := s.store.CountUsers()
	if err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	if password == "" {
		return false, errors.New("no users exist: set GOSHPANEL_BOOTSTRAP_PASSWORD to create the first admin")
	}
	if len(password) < 8 {
		return false, errors.New("GOSHPANEL_BOOTSTRAP_PASSWORD must be at least 8 characters")
	}
	if _, err := s.CreateUser(username, password, "admin"); err != nil {
		return false, fmt.Errorf("bootstrap admin: %w", err)
	}
	return true, nil
}

// ResetBootstrapPassword updates the bootstrap user's password when local
// recovery is explicitly requested via GOSHPANEL_BOOTSTRAP_RESET=true.
func (s *Service) ResetBootstrapPassword(username, password string, reset bool) (bool, error) {
	if !reset {
		return false, nil
	}
	if password == "" {
		return false, errors.New("GOSHPANEL_BOOTSTRAP_RESET requires GOSHPANEL_BOOTSTRAP_PASSWORD")
	}
	if len(password) < 8 {
		return false, errors.New("GOSHPANEL_BOOTSTRAP_PASSWORD must be at least 8 characters")
	}
	u, err := s.store.UserByUsername(strings.TrimSpace(username))
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := s.ChangePassword(u.ID, password); err != nil {
		return false, fmt.Errorf("reset bootstrap password: %w", err)
	}
	return true, nil
}

// CreateUser validates and creates an account.
func (s *Service) CreateUser(username, password, role string) (store.User, error) {
	username = strings.TrimSpace(username)
	if len(username) < 2 {
		return store.User{}, errors.New("username must be at least 2 characters")
	}
	if len(password) < 8 {
		return store.User{}, errors.New("password must be at least 8 characters")
	}
	if role != "admin" && role != "user" {
		return store.User{}, fmt.Errorf("invalid role %q", role)
	}
	hash, err := crypto.HashPassword(password)
	if err != nil {
		return store.User{}, err
	}
	return s.store.CreateUser(username, hash, role)
}

// Authenticate resolves a session token to its user.
func (s *Service) Authenticate(token string) (store.User, store.Session, error) {
	sess, err := s.store.SessionByToken(token)
	if err != nil {
		return store.User{}, store.Session{}, err
	}
	u, err := s.store.UserByID(sess.UserID)
	if err != nil {
		return store.User{}, store.Session{}, err
	}
	return u, sess, nil
}

// Logout deletes the session for token.
func (s *Service) Logout(token string) error { return s.store.DeleteSession(token) }

// ChangePassword sets a new password for the user.
func (s *Service) ChangePassword(userID int64, newPassword string) error {
	if len(newPassword) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	hash, err := crypto.HashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.store.UpdateUserPassword(userID, hash)
}
