package backups

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateListRestoreDelete(t *testing.T) {
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := New(src, t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	name, err := s.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	list, err := s.List()
	if err != nil || len(list) != 1 || list[0].Name != name {
		t.Fatalf("List: %v %+v", err, list)
	}

	// Wipe the source, restore, verify content returns.
	if err := os.RemoveAll(filepath.Join(src, "sub")); err != nil {
		t.Fatal(err)
	}
	if err := s.Restore(name); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(src, "sub", "file.txt"))
	if err != nil || string(data) != "data" {
		t.Fatalf("restored file: %v %q", err, data)
	}

	if err := s.Delete(name); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	list, _ = s.List()
	if len(list) != 0 {
		t.Errorf("expected empty list, got %+v", list)
	}
}

func TestInvalidNamesRejected(t *testing.T) {
	s, err := New(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../etc/passwd.tar.gz", "x/y.tar.gz", "plain.txt"} {
		if err := s.Restore(name); err == nil {
			t.Errorf("Restore(%q) accepted", name)
		}
		if err := s.Delete(name); err == nil {
			t.Errorf("Delete(%q) accepted", name)
		}
	}
}
