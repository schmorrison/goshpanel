package logs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTailAllowlist(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")
	var b strings.Builder
	for i := 1; i <= 50; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	if err := os.WriteFile(logPath, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	s := New([]string{logPath})

	lines, err := s.Tail(logPath, 10)
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}
	if len(lines) != 10 || lines[9] != "line 50" || lines[0] != "line 41" {
		t.Errorf("lines = %v", lines)
	}

	if _, err := s.Tail(filepath.Join(dir, "other.log"), 10); !errors.Is(err, ErrNotAllowed) {
		t.Errorf("non-allowlisted path: %v, want ErrNotAllowed", err)
	}
}

func TestSources(t *testing.T) {
	dir := t.TempDir()
	exists := filepath.Join(dir, "a.log")
	os.WriteFile(exists, []byte("x\n"), 0o644)
	missing := filepath.Join(dir, "missing.log")

	s := New([]string{exists, missing})
	srcs := s.Sources()
	if len(srcs) != 2 || !srcs[0].Exists || srcs[1].Exists {
		t.Errorf("Sources = %+v", srcs)
	}
}
