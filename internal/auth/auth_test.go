package auth

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/schmorrison/goshpanel/internal/store"
)

func newTestService(t *testing.T) (*Service, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return New(st, time.Hour), st
}

func TestBootstrapAndLogin(t *testing.T) {
	svc, _ := newTestService(t)

	if err := svc.Bootstrap("admin", "hunter2hunter2"); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	// Second bootstrap is a no-op.
	if err := svc.Bootstrap("admin", "different"); err != nil {
		t.Fatalf("second Bootstrap: %v", err)
	}

	sess, _, err := svc.Login("admin", "hunter2hunter2")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if sess.Token == "" || sess.CSRFToken == "" {
		t.Error("session missing tokens")
	}

	u, got, err := svc.Authenticate(sess.Token)
	if err != nil || u.Username != "admin" || got.Token != sess.Token {
		t.Fatalf("Authenticate: %v %+v", err, u)
	}

	if _, _, err := svc.Login("admin", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("wrong password: %v", err)
	}
	if _, _, err := svc.Login("ghost", "hunter2hunter2"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("unknown user: %v", err)
	}

	if err := svc.Logout(sess.Token); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, _, err := svc.Authenticate(sess.Token); err == nil {
		t.Error("session survived logout")
	}
}

func TestBootstrapRequiresPassword(t *testing.T) {
	svc, _ := newTestService(t)
	if err := svc.Bootstrap("admin", ""); err == nil {
		t.Error("empty bootstrap password accepted with no users")
	}
	if err := svc.Bootstrap("admin", "short"); err == nil {
		t.Error("short bootstrap password accepted with no users")
	}
}

func TestCreateUserValidation(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.CreateUser("x", "longenough", "user"); err == nil {
		t.Error("1-char username accepted")
	}
	if _, err := svc.CreateUser("ok", "short", "user"); err == nil {
		t.Error("short password accepted")
	}
	if _, err := svc.CreateUser("ok", "longenough", "superuser"); err == nil {
		t.Error("bogus role accepted")
	}
}

func TestChangePassword(t *testing.T) {
	svc, _ := newTestService(t)
	u, err := svc.CreateUser("carol", "originalpass", "user")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ChangePassword(u.ID, "newpassword"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if _, _, err := svc.Login("carol", "originalpass"); err == nil {
		t.Error("old password still works")
	}
	if _, _, err := svc.Login("carol", "newpassword"); err != nil {
		t.Errorf("new password rejected: %v", err)
	}
}
