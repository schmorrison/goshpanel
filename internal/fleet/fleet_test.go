package fleet

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/schmorrison/goshpanel/internal/store"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestCollectorSnapshot(t *testing.T) {
	st := testStore(t)
	c := NewCollector(st, "test-node", t.TempDir())
	tel, err := c.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if tel.NodeName != "test-node" || tel.Version == "" {
		t.Errorf("telemetry = %+v", tel)
	}
}

func TestControllerPollAndIngest(t *testing.T) {
	st := testStore(t)
	agentToken := "agent-secret-token-123456"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/fleet/telemetry" {
			json.NewEncoder(w).Encode(Telemetry{NodeName: "remote", Version: "0.2.0"})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	node, err := st.CreateFleetNode("remote", srv.URL, agentToken)
	if err != nil {
		t.Fatal(err)
	}
	ctrl := NewController(st)
	if err := ctrl.PollNode(context.Background(), node); err != nil {
		t.Fatalf("PollNode: %v", err)
	}
	sample, err := st.LatestFleetTelemetry(node.ID)
	if err != nil {
		t.Fatalf("LatestFleetTelemetry: %v", err)
	}
	if !contains(sample.Payload, "remote") {
		t.Errorf("payload = %s", sample.Payload)
	}

	tel := Telemetry{NodeName: "remote", Version: "0.2.0"}
	if err := ctrl.IngestPush("remote", tel); err != nil {
		t.Fatalf("IngestPush: %v", err)
	}
}

func TestParseMode(t *testing.T) {
	if ParseMode("controller") != ModeController {
		t.Error("controller mode")
	}
	if !IsWorker(ModeBoth) || !IsController(ModeBoth) {
		t.Error("both mode")
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
