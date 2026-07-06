package files

import (
	"errors"
	"strings"
	"testing"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func TestSandboxEscapeRejected(t *testing.T) {
	s := newTestService(t)
	for _, p := range []string{"../outside", "a/../../outside", "..", "a/../.."} {
		if _, err := s.List(p); !errors.Is(err, ErrOutsideRoot) {
			t.Errorf("List(%q) = %v, want ErrOutsideRoot", p, err)
		}
	}
}

func TestWriteReadListDelete(t *testing.T) {
	s := newTestService(t)

	if err := s.Mkdir("sub"); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := s.Write("sub/hello.txt", []byte("hi there")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := s.Read("sub/hello.txt", 1024)
	if err != nil || string(data) != "hi there" {
		t.Fatalf("Read: %v %q", err, data)
	}

	if _, err := s.Read("sub/hello.txt", 3); err == nil {
		t.Error("expected size-limit error")
	}

	entries, err := s.List("sub")
	if err != nil || len(entries) != 1 || entries[0].Name != "hello.txt" {
		t.Fatalf("List: %v %+v", err, entries)
	}

	if err := s.Rename("sub/hello.txt", "sub/renamed.txt"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if err := s.Delete("sub"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if entries, _ := s.List(""); len(entries) != 0 {
		t.Errorf("root should be empty, got %+v", entries)
	}
}

func TestSaveUpload(t *testing.T) {
	s := newTestService(t)
	if err := s.Save("", "up.bin", strings.NewReader("payload")); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, _ := s.Read("up.bin", 1024)
	if string(data) != "payload" {
		t.Errorf("uploaded content = %q", data)
	}
	if err := s.Save("", "../evil", strings.NewReader("x")); err == nil {
		t.Error("path traversal in upload name accepted")
	}
}

func TestDeleteRootRefused(t *testing.T) {
	s := newTestService(t)
	if err := s.Delete(""); err == nil {
		t.Error("deleting sandbox root should fail")
	}
}
