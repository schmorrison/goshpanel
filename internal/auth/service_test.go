package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/schmorrison/goshpanel/internal/store"
	"golang.org/x/crypto/bcrypt"
)

func TestLoginLogoutFlow(t *testing.T) {
	db := store.OpenTest(t)
	users := store.NewUserRepository(db)
	sessions := store.NewSessionStore(db)

	hash, err := bcrypt.GenerateFromPassword([]byte("secret-pass"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if _, err := users.Create(context.Background(), "admin", string(hash)); err != nil {
		t.Fatalf("create user: %v", err)
	}

	svc, err := New(users, sessions, "test-session-secret-value-32b", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	handler := svc.SessionMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			if err := svc.Login(r.Context(), w, r, "admin", "secret-pass"); err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case "/logout":
			if err := svc.Logout(r.Context()); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case "/me":
			user, ok, err := svc.CurrentUser(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(user.Username))
		default:
			http.NotFound(w, r)
		}
	}))

	loginReq := httptest.NewRequest(http.MethodGet, "/login", nil)
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusNoContent {
		t.Fatalf("login status = %d, want %d", loginRec.Code, http.StatusNoContent)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/me", nil)
	for _, cookie := range loginRec.Result().Cookies() {
		meReq.AddCookie(cookie)
	}
	meRec := httptest.NewRecorder()
	handler.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("me status = %d, want %d", meRec.Code, http.StatusOK)
	}
	if meRec.Body.String() != "admin" {
		t.Fatalf("me body = %q, want %q", meRec.Body.String(), "admin")
	}

	logoutReq := httptest.NewRequest(http.MethodGet, "/logout", nil)
	for _, cookie := range loginRec.Result().Cookies() {
		logoutReq.AddCookie(cookie)
	}
	logoutRec := httptest.NewRecorder()
	handler.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want %d", logoutRec.Code, http.StatusNoContent)
	}

	meAfterLogoutReq := httptest.NewRequest(http.MethodGet, "/me", nil)
	for _, cookie := range logoutRec.Result().Cookies() {
		meAfterLogoutReq.AddCookie(cookie)
	}
	meAfterLogoutRec := httptest.NewRecorder()
	handler.ServeHTTP(meAfterLogoutRec, meAfterLogoutReq)
	if meAfterLogoutRec.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout status = %d, want %d", meAfterLogoutRec.Code, http.StatusUnauthorized)
	}
}

func TestBootstrapCreatesFirstUser(t *testing.T) {
	db := store.OpenTest(t)
	users := store.NewUserRepository(db)
	sessions := store.NewSessionStore(db)

	svc, err := New(users, sessions, "test-session-secret-value-32b", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := svc.Bootstrap(context.Background(), "root", "bootstrap-pass"); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}

	count, err := users.Count(context.Background())
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("Count() = %d, want 1", count)
	}

	if err := svc.Bootstrap(context.Background(), "other", "ignored"); err != nil {
		t.Fatalf("second Bootstrap() error = %v", err)
	}
	count, err = users.Count(context.Background())
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("Count() after second bootstrap = %d, want 1", count)
	}
}
