package web

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/schmorrison/goshpanel/internal/fleet"
)

func (s *Server) fleetAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		expected := s.workerFleetToken()
		if expected == "" {
			http.Error(w, "fleet API disabled: set GOSHPANEL_FLEET_TOKEN or enroll as a worker", http.StatusServiceUnavailable)
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *Server) workerFleetToken() string {
	if s.cfg.FleetToken != "" {
		return s.cfg.FleetToken
	}
	if creds, err := fleet.LoadAgentCredentials(s.cfg.DataDir); err == nil {
		return creds.NodeToken
	}
	return ""
}

func (s *Server) handleFleetTelemetryAPI(w http.ResponseWriter, r *http.Request) {
	t, err := s.collector.Snapshot()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleFleetControlAPI(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	req, err := fleet.ParseControlRequest(body)
	if err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	res := s.executeFleetControl(r, req)
	if !res.OK {
		writeJSON(w, http.StatusInternalServerError, res)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleFleetEnrollAPI(w http.ResponseWriter, r *http.Request) {
	if s.fleet == nil {
		http.Error(w, "not a fleet controller", http.StatusForbidden)
		return
	}
	secret := fleet.EnrollSecret(s.cfg)
	if secret == "" {
		http.Error(w, "enrollment disabled: set GOSHPANEL_FLEET_ENROLL_SECRET or GOSHPANEL_FLEET_TOKEN", http.StatusServiceUnavailable)
		return
	}
	var req fleet.EnrollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	res, err := fleet.EnrollController(s.store, secret, requestBaseURL(r), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleFleetIngestAPI(w http.ResponseWriter, r *http.Request) {
	if s.fleet == nil {
		http.Error(w, "not a fleet controller", http.StatusForbidden)
		return
	}
	var req fleet.IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	node, err := s.store.FleetNodeByName(req.NodeName)
	if err != nil {
		http.Error(w, "unknown node", http.StatusUnauthorized)
		return
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(node.Token)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := s.fleet.IngestPush(req.NodeName, req.Telemetry); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
