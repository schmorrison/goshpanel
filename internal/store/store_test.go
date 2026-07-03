package store

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestUserCRUD(t *testing.T) {
	s := openTestStore(t)

	u, err := s.CreateUser("alice", "hash1", "admin")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.ID == 0 {
		t.Error("expected assigned ID")
	}

	got, err := s.UserByUsername("alice")
	if err != nil || got.ID != u.ID {
		t.Fatalf("UserByUsername: %v %+v", err, got)
	}

	if _, err := s.CreateUser("alice", "hash2", "user"); err == nil {
		t.Error("duplicate username should fail")
	}

	if err := s.UpdateUserPassword(u.ID, "hash3"); err != nil {
		t.Fatalf("UpdateUserPassword: %v", err)
	}
	got, _ = s.UserByID(u.ID)
	if got.PasswordHash != "hash3" {
		t.Errorf("PasswordHash = %q", got.PasswordHash)
	}

	if err := s.DeleteUser(u.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if _, err := s.UserByID(u.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSessions(t *testing.T) {
	s := openTestStore(t)
	u, _ := s.CreateUser("bob", "h", "user")

	sess := Session{Token: "tok1", UserID: u.ID, CSRFToken: "csrf1", ExpiresAt: time.Now().Add(time.Hour)}
	if err := s.CreateSession(sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	got, err := s.SessionByToken("tok1")
	if err != nil || got.UserID != u.ID {
		t.Fatalf("SessionByToken: %v %+v", err, got)
	}

	expired := Session{Token: "tok2", UserID: u.ID, CSRFToken: "csrf2", ExpiresAt: time.Now().Add(-time.Hour)}
	if err := s.CreateSession(expired); err != nil {
		t.Fatalf("CreateSession expired: %v", err)
	}
	if _, err := s.SessionByToken("tok2"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired session should be ErrNotFound, got %v", err)
	}

	// Deleting the user cascades to sessions.
	if err := s.DeleteUser(u.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if _, err := s.SessionByToken("tok1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("cascade delete failed, got %v", err)
	}
}

func TestDomainsAndDNS(t *testing.T) {
	s := openTestStore(t)

	d, err := s.CreateDomain("example.com", "/srv/example", "")
	if err != nil {
		t.Fatalf("CreateDomain: %v", err)
	}
	rec, err := s.CreateDNSRecord(DNSRecord{DomainID: d.ID, Type: "A", Name: "@", Value: "192.0.2.1", TTL: 3600})
	if err != nil {
		t.Fatalf("CreateDNSRecord: %v", err)
	}
	recs, err := s.DNSRecords(d.ID)
	if err != nil || len(recs) != 1 || recs[0].ID != rec.ID {
		t.Fatalf("DNSRecords: %v %+v", err, recs)
	}

	// Cascade: deleting domain removes records.
	if err := s.DeleteDomain(d.ID); err != nil {
		t.Fatalf("DeleteDomain: %v", err)
	}
	recs, _ = s.DNSRecords(d.ID)
	if len(recs) != 0 {
		t.Errorf("expected cascade delete of DNS records, got %d", len(recs))
	}
}

func TestMailboxesForwardersCronDatabases(t *testing.T) {
	s := openTestStore(t)

	if _, err := s.CreateMailbox("a@x.com", "h", 100); err != nil {
		t.Fatalf("CreateMailbox: %v", err)
	}
	boxes, _ := s.Mailboxes()
	if len(boxes) != 1 {
		t.Errorf("Mailboxes = %d", len(boxes))
	}

	f, err := s.CreateForwarder("b@x.com", "c@y.com")
	if err != nil {
		t.Fatalf("CreateForwarder: %v", err)
	}
	if err := s.DeleteForwarder(f.ID); err != nil {
		t.Fatalf("DeleteForwarder: %v", err)
	}

	j, err := s.CreateCronJob("* * * * *", "echo hi", "test")
	if err != nil {
		t.Fatalf("CreateCronJob: %v", err)
	}
	jobs, _ := s.CronJobs()
	if len(jobs) != 1 || jobs[0].ID != j.ID {
		t.Errorf("CronJobs = %+v", jobs)
	}

	c, err := s.CreateDatabaseConn("local", "sqlite", "/tmp/x.db")
	if err != nil {
		t.Fatalf("CreateDatabaseConn: %v", err)
	}
	got, err := s.DatabaseConnByID(c.ID)
	if err != nil || got.Driver != "sqlite" {
		t.Fatalf("DatabaseConnByID: %v %+v", err, got)
	}
}

func TestFunctionsDockerSystemdOrchestrator(t *testing.T) {
	s := openTestStore(t)

	fn, err := s.CreateMicroFunction("ping", "health", "echo pong", "tok", 15)
	if err != nil {
		t.Fatalf("CreateMicroFunction: %v", err)
	}
	got, err := s.MicroFunctionByName("ping")
	if err != nil || got.ID != fn.ID {
		t.Fatalf("MicroFunctionByName: %v", err)
	}

	stack, err := s.CreateDockerStack("web", "services:\n  web:\n    image: nginx\n")
	if err != nil {
		t.Fatalf("CreateDockerStack: %v", err)
	}
	stacks, _ := s.DockerStacks()
	if len(stacks) != 1 || stacks[0].ID != stack.ID {
		t.Errorf("DockerStacks = %+v", stacks)
	}

	unit, err := s.CreateSystemdUnit("demo", "[Service]\nExecStart=/bin/true\n", true)
	if err != nil {
		t.Fatalf("CreateSystemdUnit: %v", err)
	}
	units, _ := s.SystemdUnits()
	if len(units) != 1 || units[0].ID != unit.ID {
		t.Errorf("SystemdUnits = %+v", units)
	}

	if err := s.SetOrchestratorStatus("caddy", "/tmp/Caddyfile", "", now()); err != nil {
		t.Fatalf("SetOrchestratorStatus: %v", err)
	}
	statuses, err := s.OrchestratorStatuses()
	if err != nil || len(statuses) != 1 || statuses[0].Component != "caddy" {
		t.Fatalf("OrchestratorStatuses: %v %+v", err, statuses)
	}
}

func TestAuditAndIPRules(t *testing.T) {
	s := openTestStore(t)

	if err := s.AppendAudit("alice", "test.action", "detail"); err != nil {
		t.Fatalf("AppendAudit: %v", err)
	}
	entries, err := s.AuditEntries(10)
	if err != nil || len(entries) != 1 || entries[0].Action != "test.action" {
		t.Fatalf("AuditEntries: %v %+v", err, entries)
	}

	r, err := s.CreateIPRule("203.0.113.0/24", "abuse")
	if err != nil {
		t.Fatalf("CreateIPRule: %v", err)
	}
	rules, _ := s.IPRules()
	if len(rules) != 1 {
		t.Errorf("IPRules = %d", len(rules))
	}
	if err := s.DeleteIPRule(r.ID); err != nil {
		t.Fatalf("DeleteIPRule: %v", err)
	}
	if err := s.DeleteIPRule(r.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("double delete should be ErrNotFound, got %v", err)
	}
}
