package fn

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/schmorrison/goshpanel/internal/store"
)

func testFnService(t *testing.T) (*Service, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	svc, err := New(st, filepath.Join(dir, "scripts"))
	if err != nil {
		t.Fatal(err)
	}
	return svc, st
}

func TestCreateValidateInvoke(t *testing.T) {
	svc, _ := testFnService(t)
	f, err := svc.Create("hello", "greeting", `echo "hi from fn"`, 10)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if f.Token == "" {
		t.Error("expected token")
	}
	res, err := svc.Invoke(context.Background(), "hello", f.Token)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if res.ExitCode != 0 || !strings.Contains(res.Output, "hi from fn") {
		t.Errorf("result = %+v", res)
	}
}

func TestInvokeBadToken(t *testing.T) {
	svc, _ := testFnService(t)
	f, _ := svc.Create("secret", "", "echo ok", 10)
	if _, err := svc.Invoke(context.Background(), "secret", "wrong"); err != ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
	_, err := svc.Invoke(context.Background(), "secret", f.Token)
	if err != nil {
		t.Errorf("valid token rejected: %v", err)
	}
}

func TestValidateName(t *testing.T) {
	for _, name := range []string{"hello", "my-fn", "a1"} {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) = %v", name, err)
		}
	}
	for _, name := range []string{"", "Hello", "1bad", "has space"} {
		if err := ValidateName(name); err == nil {
			t.Errorf("ValidateName(%q) accepted", name)
		}
	}
}

func TestUpdateDisable(t *testing.T) {
	svc, _ := testFnService(t)
	f, _ := svc.Create("toggle", "", "echo on", 10)
	if err := svc.Update(f.ID, "", "echo on", false, 10); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Invoke(context.Background(), "toggle", f.Token); err != ErrDisabled {
		t.Errorf("disabled function ran: %v", err)
	}
}

func TestDelete(t *testing.T) {
	svc, st := testFnService(t)
	f, _ := svc.Create("gone", "", "echo x", 10)
	if err := svc.Delete(f.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.MicroFunctionByName("gone"); err == nil {
		t.Error("function still in store")
	}
}
