package webhooks

import "testing"

func TestMatchesEvent(t *testing.T) {
	if !matchesEvent("*", "domains.create") {
		t.Fatal("wildcard should match")
	}
	if !matchesEvent("domains.create,backups.create", "backups.create") {
		t.Fatal("comma list should match")
	}
	if matchesEvent("domains.create", "backups.create") {
		t.Fatal("non-matching event")
	}
}
