package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/schmorrison/goshpanel/internal/store"
)

func testOrchestrator(t *testing.T) (*Service, *store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	paths := Paths{
		CaddyConfig: filepath.Join(dir, "Caddyfile"),
		CoreDNSDir:  filepath.Join(dir, "coredns"),
		MaddyConfig: filepath.Join(dir, "maddy.conf"),
		SystemdDir:  filepath.Join(dir, "systemd"),
	}
	return New(st, paths), st, dir
}

func TestApplyCaddyWritesConfig(t *testing.T) {
	orch, st, _ := testOrchestrator(t)
	if _, err := st.CreateDomain("example.com", "/srv/example", ""); err != nil {
		t.Fatal(err)
	}
	res := orch.ApplyCaddy(context.Background())
	if res.ConfigPath == "" {
		t.Fatal("missing config path")
	}
	data, err := os.ReadFile(res.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "example.com") {
		t.Errorf("Caddyfile missing domain:\n%s", data)
	}
	// caddy binary likely absent — message should mention that or reload failure.
	if res.OK && res.Message == "" {
		t.Error("expected message")
	}
}

func TestApplyCoreDNSWritesZones(t *testing.T) {
	orch, st, dir := testOrchestrator(t)
	d, _ := st.CreateDomain("example.com", "", "")
	if _, err := st.CreateDNSRecord(store.DNSRecord{DomainID: d.ID, Type: "A", Name: "@", Value: "192.0.2.1", TTL: 3600}); err != nil {
		t.Fatal(err)
	}
	res := orch.ApplyCoreDNS(context.Background())
	corefile := filepath.Join(dir, "coredns", "Corefile")
	data, err := os.ReadFile(corefile)
	if err != nil {
		t.Fatalf("Corefile: %v", err)
	}
	if !strings.Contains(string(data), "example.com") {
		t.Errorf("Corefile missing domain:\n%s", data)
	}
	zone := filepath.Join(dir, "coredns", "zones", "db.example.com")
	if _, err := os.Stat(zone); err != nil {
		t.Fatalf("zone file: %v", err)
	}
	_ = res
}

func TestApplyMaddyWritesConfig(t *testing.T) {
	orch, st, dir := testOrchestrator(t)
	if _, err := st.CreateMailbox("a@example.com", "hash", 0); err != nil {
		t.Fatal(err)
	}
	res := orch.ApplyMaddy(context.Background())
	data, err := os.ReadFile(filepath.Join(dir, "maddy.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "example.com") {
		t.Errorf("maddy config missing domain:\n%s", data)
	}
	_ = res
}

func TestApplySystemdWritesUnits(t *testing.T) {
	orch, st, dir := testOrchestrator(t)
	if _, err := st.CreateSystemdUnit("demo", "[Unit]\nDescription=Demo\n\n[Service]\nExecStart=/bin/true\n", true); err != nil {
		t.Fatal(err)
	}
	res := orch.ApplySystemd(context.Background())
	unitPath := filepath.Join(dir, "systemd", "demo.service")
	data, err := os.ReadFile(unitPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Description=Demo") {
		t.Errorf("unit file wrong:\n%s", data)
	}
	_ = res
}

func TestApplyAll(t *testing.T) {
	orch, st, _ := testOrchestrator(t)
	st.CreateDomain("x.com", "/srv/x", "")
	results := orch.ApplyAll(context.Background())
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}
	statuses, err := st.OrchestratorStatuses()
	if err != nil || len(statuses) == 0 {
		t.Fatalf("statuses: %v %d", err, len(statuses))
	}
}
