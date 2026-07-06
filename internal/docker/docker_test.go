package docker

import (
	"context"
	"testing"
)

func TestAvailable(t *testing.T) {
	_ = Available()
}

func TestIntegration(t *testing.T) {
	if !Available() {
		t.Skip("docker not installed")
	}
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Ping(context.Background()); err != nil {
		t.Skip("docker daemon not running:", err)
	}
	if _, err := s.Containers(context.Background()); err != nil {
		t.Errorf("Containers: %v", err)
	}
	if _, err := s.Images(context.Background()); err != nil {
		t.Errorf("Images: %v", err)
	}
}

func TestShortID(t *testing.T) {
	if got := ShortID("sha256:abcdef1234567890"); got != "abcdef123456" {
		t.Errorf("ShortID = %q", got)
	}
}

func TestStackWorkdir(t *testing.T) {
	got := StackWorkdir("/data", "my-stack")
	if got != "/data/docker-stacks/my-stack" {
		t.Errorf("StackWorkdir = %q", got)
	}
}
