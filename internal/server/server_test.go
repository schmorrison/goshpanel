package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/schmorrison/goshpanel/internal/auth"
	"github.com/schmorrison/goshpanel/internal/config"
	"github.com/schmorrison/goshpanel/internal/files"
	"github.com/schmorrison/goshpanel/internal/store"
	"github.com/schmorrison/goshpanel/internal/ui"
	"golang.org/x/crypto/bcrypt"
)

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	healthHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status field = %q, want %q", body["status"], "ok")
	}
}

func TestHomeRoute(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Sign in to dashboard") {
		t.Fatalf("body = %q, want public homepage content", body)
	}
}

func TestLoginAndDashboardFlow(t *testing.T) {
	srv := newTestServer(t)

	loginPageReq := httptest.NewRequest(http.MethodGet, "/login", nil)
	loginPageRec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(loginPageRec, loginPageReq)
	if loginPageRec.Code != http.StatusOK {
		t.Fatalf("login page status = %d, want %d", loginPageRec.Code, http.StatusOK)
	}

	csrfToken := extractCSRFToken(t, loginPageRec.Body.String())
	form := url.Values{}
	form.Set("csrf_token", csrfToken)
	form.Set("username", "admin")
	form.Set("password", "secret-pass")

	loginReq := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range loginPageRec.Result().Cookies() {
		loginReq.AddCookie(cookie)
	}
	loginRec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusSeeOther {
		t.Fatalf("login status = %d, want %d", loginRec.Code, http.StatusSeeOther)
	}

	dashboardReq := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	for _, cookie := range loginRec.Result().Cookies() {
		dashboardReq.AddCookie(cookie)
	}
	dashboardRec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(dashboardRec, dashboardReq)
	if dashboardRec.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d, want %d", dashboardRec.Code, http.StatusOK)
	}
	if !strings.Contains(dashboardRec.Body.String(), "Signed in as admin") {
		t.Fatalf("dashboard body missing signed-in user")
	}

	protectedReq := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	protectedRec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(protectedRec, protectedReq)
	if protectedRec.Code != http.StatusSeeOther {
		t.Fatalf("unauthenticated dashboard status = %d, want %d", protectedRec.Code, http.StatusSeeOther)
	}
}

func TestFilesRouteRequiresAuth(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/files", nil)
	rec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("unauthenticated files status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
}

func TestAuthenticatedFilesPage(t *testing.T) {
	srv := newTestServer(t)
	cookies := login(t, srv)

	req := httptest.NewRequest(http.MethodGet, "/files", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("files status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "This directory is empty") {
		t.Fatalf("files body missing empty directory message")
	}
}

func login(t *testing.T, srv *Server) []*http.Cookie {
	t.Helper()

	loginPageReq := httptest.NewRequest(http.MethodGet, "/login", nil)
	loginPageRec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(loginPageRec, loginPageReq)

	csrfToken := extractCSRFToken(t, loginPageRec.Body.String())
	form := url.Values{}
	form.Set("csrf_token", csrfToken)
	form.Set("username", "admin")
	form.Set("password", "secret-pass")

	loginReq := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range loginPageRec.Result().Cookies() {
		loginReq.AddCookie(cookie)
	}
	loginRec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusSeeOther {
		t.Fatalf("login status = %d, want %d", loginRec.Code, http.StatusSeeOther)
	}
	return loginRec.Result().Cookies()
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

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

	authService, err := auth.New(users, sessions, "test-session-secret-value-32b", nil)
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	filesService, err := files.NewService(t.TempDir(), store.NewAuditRepository(db))
	if err != nil {
		t.Fatalf("files.NewService() error = %v", err)
	}

	uiHandler := ui.NewHandler(authService, filesService)
	return New(config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)), authService, uiHandler)
}

func extractCSRFToken(t *testing.T, html string) string {
	t.Helper()

	const prefix = `name="csrf_token" value="`
	start := strings.Index(html, prefix)
	if start == -1 {
		t.Fatal("csrf token not found in login page")
	}
	start += len(prefix)
	end := strings.Index(html[start:], `"`)
	if end == -1 {
		t.Fatal("csrf token value terminator not found")
	}
	return html[start : start+end]
}
