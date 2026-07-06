package auth

import (
	"errors"
	"strings"
	"time"

	"github.com/schmorrison/goshpanel/internal/crypto"
	"github.com/schmorrison/goshpanel/internal/store"
)

// ErrMFARequired indicates password was valid but TOTP is still required.
var ErrMFARequired = errors.New("totp code required")

// PendingLogin is returned when password auth succeeded but 2FA is pending.
type PendingLogin struct {
	Token string
	User  store.User
}

// Login verifies credentials. When 2FA is enabled, returns PendingLogin instead of a session.
func (s *Service) Login(username, password string) (store.Session, *PendingLogin, error) {
	u, err := s.store.UserByUsername(strings.TrimSpace(username))
	if errors.Is(err, store.ErrNotFound) {
		crypto.VerifyPassword("argon2id$3$65536$4$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", password)
		return store.Session{}, nil, ErrInvalidCredentials
	}
	if err != nil {
		return store.Session{}, nil, err
	}
	if !crypto.VerifyPassword(u.PasswordHash, password) {
		return store.Session{}, nil, ErrInvalidCredentials
	}
	if u.TOTPEnabled {
		token, err := crypto.RandomToken()
		if err != nil {
			return store.Session{}, nil, err
		}
		if err := s.store.CreateLoginPending(token, u.ID, time.Now().Add(5*time.Minute)); err != nil {
			return store.Session{}, nil, err
		}
		return store.Session{}, &PendingLogin{Token: token, User: u}, ErrMFARequired
	}
	sess, err := s.createSession(u.ID)
	return sess, nil, err
}

// CompleteMFA verifies a pending login with a TOTP code.
func (s *Service) CompleteMFA(pendingToken, code string) (store.Session, error) {
	p, err := s.store.ConsumeLoginPending(strings.TrimSpace(pendingToken))
	if err != nil {
		return store.Session{}, ErrInvalidCredentials
	}
	u, err := s.store.UserByID(p.UserID)
	if err != nil {
		return store.Session{}, err
	}
	if !u.TOTPEnabled || !ValidateTOTP(u.TOTPSecret, code) {
		return store.Session{}, ErrInvalidCredentials
	}
	return s.createSession(u.ID)
}

// BeginTOTPSetup generates a secret for enrollment (not enabled until confirmed).
func (s *Service) BeginTOTPSetup(userID int64) (secret, uri string, err error) {
	secret, err = GenerateTOTPSecret()
	if err != nil {
		return "", "", err
	}
	u, err := s.store.UserByID(userID)
	if err != nil {
		return "", "", err
	}
	if err := s.store.SetUserTOTP(userID, secret, false); err != nil {
		return "", "", err
	}
	return secret, TOTPProvisioningURI(secret, u.Username, "GoshPanel"), nil
}

// ConfirmTOTPSetup enables 2FA after validating a code against the pending secret.
func (s *Service) ConfirmTOTPSetup(userID int64, code string) error {
	u, err := s.store.UserByID(userID)
	if err != nil {
		return err
	}
	if u.TOTPSecret == "" || !ValidateTOTP(u.TOTPSecret, code) {
		return errors.New("invalid authenticator code")
	}
	return s.store.SetUserTOTP(userID, u.TOTPSecret, true)
}

// DisableTOTP turns off 2FA after validating the current code.
func (s *Service) DisableTOTP(userID int64, code string) error {
	u, err := s.store.UserByID(userID)
	if err != nil {
		return err
	}
	if !u.TOTPEnabled || !ValidateTOTP(u.TOTPSecret, code) {
		return errors.New("invalid authenticator code")
	}
	return s.store.SetUserTOTP(userID, "", false)
}

func (s *Service) createSession(userID int64) (store.Session, error) {
	token, err := crypto.RandomToken()
	if err != nil {
		return store.Session{}, err
	}
	csrf, err := crypto.RandomToken()
	if err != nil {
		return store.Session{}, err
	}
	sess := store.Session{
		Token:     token,
		UserID:    userID,
		CSRFToken: csrf,
		ExpiresAt: time.Now().Add(s.sessionTTL),
	}
	if err := s.store.CreateSession(sess); err != nil {
		return store.Session{}, err
	}
	return sess, nil
}
