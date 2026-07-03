package fleet

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/schmorrison/goshpanel/internal/config"
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

func TestEnrollJWT(t *testing.T) {
	secret := "enroll-secret-key"
	token, err := SignEnrollToken(secret, time.Hour, "worker-east")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := VerifyEnrollToken(secret, token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Name != "worker-east" {
		t.Errorf("name = %q", claims.Name)
	}
	if _, err := VerifyEnrollToken("wrong-secret", token); err == nil {
		t.Fatal("expected wrong secret to fail")
	}
}

func TestEnrollControllerAndWorker(t *testing.T) {
	st := testStore(t)
	secret := "controller-enroll-secret"
	token, err := SignEnrollToken(secret, time.Hour, "auto-worker")
	if err != nil {
		t.Fatal(err)
	}

	var enrolledToken string
	var controllerURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/fleet/enroll" {
			var req EnrollRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			res, err := EnrollController(st, secret, controllerURL, req)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}
			enrolledToken = res.NodeToken
			json.NewEncoder(w).Encode(res)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	controllerURL = srv.URL

	cfg := config.Config{
		DataDir:            t.TempDir(),
		FleetMode:          "worker",
		FleetNodeName:      "auto-worker",
		FleetControllerURL: srv.URL,
		FleetEnrollToken:   token,
		FleetPublicURL:     "http://worker.example:4674",
		Addr:               ":4674",
	}
	creds, err := EnrollWorker(context.Background(), cfg, NewClient())
	if err != nil {
		t.Fatalf("EnrollWorker: %v", err)
	}
	if creds.NodeToken == "" || creds.NodeName != "auto-worker" {
		t.Fatalf("creds = %+v", creds)
	}
	if enrolledToken != creds.NodeToken {
		t.Fatalf("token mismatch: %q vs %q", enrolledToken, creds.NodeToken)
	}

	loaded, err := LoadAgentCredentials(cfg.DataDir)
	if err != nil {
		t.Fatalf("LoadAgentCredentials: %v", err)
	}
	if loaded.NodeToken != creds.NodeToken {
		t.Errorf("persisted token = %q", loaded.NodeToken)
	}

	node, err := st.FleetNodeByName("auto-worker")
	if err != nil {
		t.Fatalf("FleetNodeByName: %v", err)
	}
	if node.BaseURL != "http://worker.example:4674" {
		t.Errorf("base URL = %q", node.BaseURL)
	}
}

func TestUpsertFleetNode(t *testing.T) {
	st := testStore(t)
	if _, err := st.UpsertFleetNode("n1", "http://a", "tok1"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertFleetNode("n1", "http://b", "tok2"); err != nil {
		t.Fatal(err)
	}
	node, err := st.FleetNodeByName("n1")
	if err != nil {
		t.Fatal(err)
	}
	if node.BaseURL != "http://b" || node.Token != "tok2" {
		t.Errorf("node = %+v", node)
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
