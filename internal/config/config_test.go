package config

import "testing"

func lookupFrom(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(lookupFrom(nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != ":4674" {
		t.Errorf("Addr = %q, want :4674", cfg.Addr)
	}
	if cfg.SessionTTLMinutes != 720 {
		t.Errorf("SessionTTLMinutes = %d, want 720", cfg.SessionTTLMinutes)
	}
	if !cfg.CommandRunnerEnabled {
		t.Error("CommandRunnerEnabled should default to true")
	}
}

func TestLoadOverrides(t *testing.T) {
	cfg, err := Load(lookupFrom(map[string]string{
		"GOSHPANEL_ADDR":           ":9999",
		"GOSHPANEL_LOG_SOURCES":    "/var/log/a.log, /var/log/b.log",
		"GOSHPANEL_COMMAND_RUNNER": "false",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != ":9999" {
		t.Errorf("Addr = %q", cfg.Addr)
	}
	if len(cfg.LogSources) != 2 || cfg.LogSources[1] != "/var/log/b.log" {
		t.Errorf("LogSources = %v", cfg.LogSources)
	}
	if cfg.CommandRunnerEnabled {
		t.Error("CommandRunnerEnabled should be false")
	}
}

func TestLoadInvalidTTL(t *testing.T) {
	if _, err := Load(lookupFrom(map[string]string{"GOSHPANEL_SESSION_TTL_MINUTES": "nope"})); err == nil {
		t.Error("expected error for invalid TTL")
	}
}
