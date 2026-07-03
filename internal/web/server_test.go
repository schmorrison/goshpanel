package web

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/schmorrison/goshpanel/internal/config"
	"github.com/schmorrison/goshpanel/internal/store"
)

// newTestServer builds a fully wired server with temp dirs and a
// bootstrapped admin/testpassword account.
func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{
		Addr:                 ":0",
		DataDir:              dir,
		FilesRoot:            filepath.Join(dir, "workspace"),
		BootstrapUser:        "admin",
		BootstrapPassword:    "testpassword",
		SessionTTLMinutes:    60,
		CommandRunnerEnabled: true,
		OrchestratorEnabled:  true,
		CaddyConfigPath:      filepath.Join(dir, "Caddyfile"),
		CoreDNSConfigDir:     filepath.Join(dir, "coredns"),
		MaddyConfigPath:      filepath.Join(dir, "maddy.conf"),
		SystemdUnitDir:       filepath.Join(dir, "systemd"),
		FunctionsEnabled:     true,
		DockerEnabled:        true,
	}
	st, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	srv, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), st)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return srv.Handler()
}

// login performs the login flow and returns the session cookie and CSRF token.
func login(t *testing.T, h http.Handler) (*http.Cookie, string) {
	t.Helper()
	form := url.Values{"username": {"admin"}, "password": {"testpassword"}}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("login: code=%d loc=%q body=%s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("no session cookie set")
	}

	// Fetch the dashboard to extract the CSRF token from the logout form.
	req = httptest.NewRequest("GET", "/", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	marker := `name="csrf_token" value="`
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatal("csrf token not found in dashboard")
	}
	rest := body[i+len(marker):]
	return cookie, rest[:strings.Index(rest, `"`)]
}

func TestRedirectsToLoginWhenAnonymous(t *testing.T) {
	h := newTestServer(t)
	for _, path := range []string{"/", "/files", "/domains", "/databases", "/email", "/cron", "/backups", "/logs", "/security", "/terminal"} {
		req := httptest.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
			t.Errorf("%s: code=%d loc=%q", path, rec.Code, rec.Header().Get("Location"))
		}
	}
}

func TestLoginBadPassword(t *testing.T) {
	h := newTestServer(t)
	form := url.Values{"username": {"admin"}, "password": {"wrong"}}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/login?error=") {
		t.Errorf("bad login: code=%d loc=%q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestAuthenticatedPagesRender(t *testing.T) {
	h := newTestServer(t)
	cookie, _ := login(t, h)

	pages := map[string]string{
		"/":              "Server information",
		"/files":         "File Manager",
		"/domains":       "Add domain",
		"/databases":     "Add connection",
		"/email":         "Create mailbox",
		"/cron":          "Add cron job",
		"/backups":       "Create backup now",
		"/logs":          "Log sources",
		"/security":      "Panel users",
		"/terminal":      "Run command",
		"/orchestrator":  "Live orchestration",
		"/docker":        "Docker",
		"/functions":     "Create micro function",
	}
	for path, want := range pages {
		req := httptest.NewRequest("GET", path, nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: code=%d", path, rec.Code)
			continue
		}
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s: missing %q", path, want)
		}
	}
}

func TestCSRFRequired(t *testing.T) {
	h := newTestServer(t)
	cookie, csrf := login(t, h)

	// Without a CSRF token the mutation is rejected.
	form := url.Values{"schedule": {"* * * * *"}, "command": {"echo hi"}}
	req := httptest.NewRequest("POST", "/cron/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("missing CSRF: code=%d, want 403", rec.Code)
	}

	// With the token it succeeds.
	form.Set("csrf_token", csrf)
	req = httptest.NewRequest("POST", "/cron/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/cron?flash=") {
		t.Errorf("with CSRF: code=%d loc=%q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestFileManagerEndToEnd(t *testing.T) {
	h := newTestServer(t)
	cookie, csrf := login(t, h)

	post := func(path string, form url.Values) *httptest.ResponseRecorder {
		form.Set("csrf_token", csrf)
		req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	// Create folder, create file inside it, verify listing, delete.
	if rec := post("/files/mkdir", url.Values{"dir": {""}, "name": {"docs"}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("mkdir: %d %s", rec.Code, rec.Body.String())
	}
	if rec := post("/files/save", url.Values{"path": {"docs/note.txt"}, "content": {"hello world"}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("save: %d", rec.Code)
	}

	req := httptest.NewRequest("GET", "/files?dir=docs", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "note.txt") {
		t.Error("listing missing uploaded file")
	}

	req = httptest.NewRequest("GET", "/files/view?path=docs/note.txt", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "hello world") {
		t.Error("editor missing file content")
	}

	// Path traversal must not escape the sandbox.
	if rec := post("/files/save", url.Values{"path": {"../escape.txt"}, "content": {"x"}}); rec.Code != http.StatusSeeOther ||
		!strings.Contains(rec.Header().Get("Location"), "error=") {
		t.Errorf("traversal save: code=%d loc=%q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestIPBlockerBlocksPanel(t *testing.T) {
	h := newTestServer(t)
	cookie, csrf := login(t, h)

	form := url.Values{"cidr": {"192.0.2.0/24"}, "comment": {"test"}, "csrf_token": {csrf}}
	req := httptest.NewRequest("POST", "/security/ip/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("block create: %d", rec.Code)
	}

	req = httptest.NewRequest("GET", "/login", nil)
	req.RemoteAddr = "192.0.2.10:5555"
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("blocked IP got %d, want 403", rec.Code)
	}

	req = httptest.NewRequest("GET", "/login", nil)
	req.RemoteAddr = "198.51.100.1:5555"
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("unblocked IP got %d, want 200", rec.Code)
	}
}

func TestFunctionPublicInvoke(t *testing.T) {
	h := newTestServer(t)
	cookie, csrf := login(t, h)

	form := url.Values{
		"name": {"echo-test"}, "description": {"demo"},
		"script": {"echo public-invoke-ok"}, "timeout_sec": {"10"},
		"csrf_token": {csrf},
	}
	req := httptest.NewRequest("POST", "/functions/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create function: %d", rec.Code)
	}

	req = httptest.NewRequest("GET", "/functions", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "echo-test") {
		t.Fatal("function not listed")
	}

	req = httptest.NewRequest("GET", "/functions/1/edit", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	invokeMarker := "/fn/echo-test?token="
	if !strings.Contains(rec.Body.String(), invokeMarker) {
		t.Fatal("invoke URL missing")
	}
	rest := rec.Body.String()[strings.Index(rec.Body.String(), invokeMarker)+len(invokeMarker):]
	raw, _, _ := strings.Cut(rest, "\n")
	raw, _, _ = strings.Cut(raw, "<")
	token := strings.TrimSpace(raw)

	req = httptest.NewRequest("GET", "/fn/echo-test?token="+token, nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "public-invoke-ok") {
		t.Errorf("public invoke: code=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest("GET", "/fn/echo-test?token=wrong", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("bad token: code=%d", rec.Code)
	}
}

func TestTerminalRun(t *testing.T) {
	h := newTestServer(t)
	cookie, csrf := login(t, h)

	form := url.Values{"command": {"echo goshpanel-test-output"}, "csrf_token": {csrf}}
	req := httptest.NewRequest("POST", "/terminal/run", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "goshpanel-test-output") {
		t.Errorf("terminal run: code=%d", rec.Code)
	}
}
