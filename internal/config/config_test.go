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
	if cfg.Database.Path != defaultDatabasePath {
		t.Errorf("Database.Path = %q, want %q", cfg.Database.Path, defaultDatabasePath)
	}
	if cfg.Auth.BootstrapUsername != defaultBootstrapUser {
		t.Errorf("BootstrapUsername = %q, want %q", cfg.Auth.BootstrapUsername, defaultBootstrapUser)
	}
	if cfg.Files.Root != defaultFilesRoot {
		t.Errorf("Files.Root = %q, want %q", cfg.Files.Root, defaultFilesRoot)
	}
}

func TestLoadFromEnvironment(t *testing.T) {
	t.Setenv("GOSHPANEL_SERVER_HOST", "127.0.0.1")
	t.Setenv("GOSHPANEL_SERVER_PORT", "8080")
	t.Setenv("GOSHPANEL_DATABASE_PATH", "/tmp/goshpanel.db")
	t.Setenv("GOSHPANEL_AUTH_BOOTSTRAP_PASSWORD", "from-env")

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
	if cfg.Database.Path != "/tmp/goshpanel.db" {
		t.Errorf("Database.Path = %q, want %q", cfg.Database.Path, "/tmp/goshpanel.db")
	}
	if cfg.Auth.BootstrapPassword != "from-env" {
		t.Errorf("BootstrapPassword = %q, want %q", cfg.Auth.BootstrapPassword, "from-env")
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

func TestSessionSecretUsesConfiguredValue(t *testing.T) {
	cfg := Config{
		Auth: AuthConfig{SessionSecret: "configured-secret"},
	}

	secret, generated, err := cfg.SessionSecret()
	if err != nil {
		t.Fatalf("SessionSecret() error = %v", err)
	}
	if generated {
		t.Fatal("SessionSecret() generated = true, want false")
	}
	if secret != "configured-secret" {
		t.Fatalf("SessionSecret() = %q, want %q", secret, "configured-secret")
	}
}

func TestSessionSecretGeneratesWhenMissing(t *testing.T) {
	secret, generated, err := Config{}.SessionSecret()
	if err != nil {
		t.Fatalf("SessionSecret() error = %v", err)
	}
	if !generated {
		t.Fatal("SessionSecret() generated = false, want true")
	}
	if len(secret) != 64 {
		t.Fatalf("generated secret length = %d, want 64", len(secret))
	}
}
