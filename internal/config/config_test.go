package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Host != defaultHost {
		t.Errorf("Host = %q, want %q", cfg.Server.Host, defaultHost)
	}
	if cfg.Server.Port != defaultPort {
		t.Errorf("Port = %d, want %d", cfg.Server.Port, defaultPort)
	}
	if cfg.Addr() != "0.0.0.0:4674" {
		t.Errorf("Addr() = %q, want %q", cfg.Addr(), "0.0.0.0:4674")
	}
}

func TestLoadFromEnvironment(t *testing.T) {
	t.Setenv("GOSHPANEL_SERVER_HOST", "127.0.0.1")
	t.Setenv("GOSHPANEL_SERVER_PORT", "8080")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Host = %q, want %q", cfg.Server.Host, "127.0.0.1")
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Port = %d, want %d", cfg.Server.Port, 8080)
	}
}

func TestLoadIgnoresMissingConfigFile(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	if _, err := Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
}
