package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/schmorrison/goshpanel/internal/store"
	"golang.org/x/crypto/bcrypt"
)

const (
	sessionUserIDKey = "user_id"
	csrfSessionKey   = "csrf_token"
)

// ErrInvalidCredentials is returned when login details do not match a user.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrInvalidCSRF is returned when a CSRF token does not match the session.
var ErrInvalidCSRF = errors.New("invalid csrf token")

// Service handles authentication and sessions.
type Service struct {
	users   *store.UserRepository
	session *scs.SessionManager
	logger  *slog.Logger
}

// New creates an authentication service.
func New(users *store.UserRepository, sessionStore scs.Store, secret string, logger *slog.Logger) (*Service, error) {
	if secret == "" {
		return nil, errors.New("session secret is required")
	}
	_ = secret // reserved for future signed-cookie configuration
	if logger == nil {
		logger = slog.Default()
	}

	session := scs.New()
	session.Store = sessionStore
	session.Lifetime = 24 * time.Hour
	session.IdleTimeout = 2 * time.Hour
	session.Cookie.Name = "goshpanel_session"
	session.Cookie.HttpOnly = true
	session.Cookie.SameSite = http.SameSiteLaxMode
	session.Cookie.Secure = false

	return &Service{
		users:   users,
		session: session,
		logger:  logger,
	}, nil
}

// SessionMiddleware returns session loading middleware.
func (s *Service) SessionMiddleware() func(http.Handler) http.Handler {
	return s.session.LoadAndSave
}

// CSRFToken returns the CSRF token for the current session.
func (s *Service) CSRFToken(ctx context.Context) string {
	if token, ok := s.session.Get(ctx, csrfSessionKey).(string); ok && token != "" {
		return token
	}

	token := randomToken()
	s.session.Put(ctx, csrfSessionKey, token)
	return token
}

// ValidateCSRF validates a submitted CSRF token.
func (s *Service) ValidateCSRF(ctx context.Context, token string) error {
	expected, ok := s.session.Get(ctx, csrfSessionKey).(string)
	if !ok || expected == "" || token == "" || expected != token {
		return ErrInvalidCSRF
	}
	return nil
}

// Login authenticates a user and starts a session.
func (s *Service) Login(ctx context.Context, w http.ResponseWriter, r *http.Request, username, password string) error {
	user, err := s.users.FindByUsername(ctx, username)
	if err != nil {
		return ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}

	s.session.Put(ctx, sessionUserIDKey, user.ID)
	if err := s.session.RenewToken(ctx); err != nil {
		return fmt.Errorf("renew session token: %w", err)
	}
	return nil
}

// Logout ends the current session.
func (s *Service) Logout(ctx context.Context) error {
	return s.session.Destroy(ctx)
}

// CurrentUser returns the authenticated user, if any.
func (s *Service) CurrentUser(ctx context.Context) (store.User, bool, error) {
	userID, ok := s.session.Get(ctx, sessionUserIDKey).(int64)
	if !ok || userID == 0 {
		return store.User{}, false, nil
	}

	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return store.User{}, false, err
	}

	return user, true, nil
}

// Bootstrap creates the first admin user when the database is empty.
func (s *Service) Bootstrap(ctx context.Context, username, password string) error {
	count, err := s.users.Count(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	if username == "" {
		username = "admin"
	}
	if password == "" {
		password = randomPassword()
		s.logger.Warn("created bootstrap admin user; change this password after first login",
			"username", username,
			"password", password,
		)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash bootstrap password: %w", err)
	}

	if _, err := s.users.Create(ctx, username, string(hash)); err != nil {
		return fmt.Errorf("create bootstrap user: %w", err)
	}

	return nil
}

// RequireAuth redirects unauthenticated users to /login.
func (s *Service) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok, err := s.CurrentUser(r.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RedirectIfAuthenticated sends logged-in users away from public auth pages.
func (s *Service) RedirectIfAuthenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok, err := s.CurrentUser(r.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if ok {
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func randomPassword() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return hex.EncodeToString([]byte("goshpanel-bootstrap"))
	}
	return hex.EncodeToString(buf)
}

func randomToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "fallback-csrf-token"
	}
	return hex.EncodeToString(buf)
}
