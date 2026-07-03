package runner

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunEcho(t *testing.T) {
	s := New(t.TempDir(), 10*time.Second, true)
	res, err := s.Run(context.Background(), "echo hello")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode != 0 || !strings.Contains(res.Output, "hello") {
		t.Errorf("res = %+v", res)
	}
}

func TestRunNonZeroExit(t *testing.T) {
	s := New(t.TempDir(), 10*time.Second, true)
	res, err := s.Run(context.Background(), "exit 3")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", res.ExitCode)
	}
}

func TestRunTimeout(t *testing.T) {
	s := New(t.TempDir(), 200*time.Millisecond, true)
	res, _ := s.Run(context.Background(), "sleep 5")
	if !res.TimedOut {
		t.Errorf("expected timeout, res = %+v", res)
	}
}

func TestDisabled(t *testing.T) {
	s := New(t.TempDir(), time.Second, false)
	if _, err := s.Run(context.Background(), "echo hi"); err == nil {
		t.Error("disabled runner executed a command")
	}
}
