package files

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/schmorrison/goshpanel/internal/store"
)

func TestListAndUploadDelete(t *testing.T) {
	root := t.TempDir()
	db := store.OpenTest(t)
	audit := store.NewAuditRepository(db)
	svc, err := NewService(root, audit)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx := context.Background()
	entries, err := svc.List(ctx, 1, "admin", "")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("List() len = %d, want 0", len(entries))
	}

	if err := svc.Upload(ctx, 1, "admin", "", "notes.txt", strings.NewReader("hello")); err != nil {
		t.Fatalf("Upload() error = %v", err)
	}

	entries, err = svc.List(ctx, 1, "admin", "")
	if err != nil {
		t.Fatalf("List() after upload error = %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "notes.txt" {
		t.Fatalf("entries after upload = %+v", entries)
	}

	if err := svc.Delete(ctx, 1, "admin", "notes.txt"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	events, err := audit.Recent(ctx, 10)
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	if len(events) < 3 {
		t.Fatalf("audit events = %d, want at least 3", len(events))
	}
}

func TestResolveRejectsPathEscape(t *testing.T) {
	root := t.TempDir()
	svc, err := NewService(root, nil)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if err := svc.Delete(context.Background(), 1, "admin", ".."); !errorsIs(err, ErrPathEscape) {
		t.Fatalf("Delete(..) error = %v, want ErrPathEscape", err)
	}
}

func errorsIs(err, target error) bool {
	return err == target
}

func TestCannotDeleteSandboxRoot(t *testing.T) {
	root := t.TempDir()
	svc, err := NewService(root, nil)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if err := svc.Delete(context.Background(), 1, "admin", ""); err == nil {
		t.Fatal("Delete() error = nil, want error")
	}
}

func TestUploadCreatesNestedPath(t *testing.T) {
	root := t.TempDir()
	svc, err := NewService(root, nil)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	if err := svc.Upload(context.Background(), 1, "admin", "docs", "readme.md", strings.NewReader("# hi")); err != nil {
		t.Fatalf("Upload() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "docs", "readme.md")); err != nil {
		t.Fatalf("uploaded file missing: %v", err)
	}
}
