package web

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/schmorrison/goshpanel/internal/fleet"
	"github.com/schmorrison/goshpanel/internal/orchestrator"
)

func (s *Server) fleetAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.FleetToken == "" {
			http.Error(w, "fleet API disabled: set GOSHPANEL_FLEET_TOKEN", http.StatusServiceUnavailable)
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.FleetToken)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
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
	res := s.executeFleetControl(r, req.Action)
	if !res.OK {
		writeJSON(w, http.StatusInternalServerError, res)
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

func (s *Server) executeFleetControl(r *http.Request, action string) fleet.ControlResponse {
	switch action {
	case fleet.ActionPing:
		return fleet.ControlResponse{OK: true, Message: "pong"}
	case fleet.ActionApplyAll:
		if s.orch == nil {
			return fleet.ControlResponse{Message: "orchestrator disabled"}
		}
		for _, res := range s.orch.ApplyAll(r.Context()) {
			if !res.OK {
				return fleet.ControlResponse{Message: res.Component + ": " + res.Message}
			}
		}
		return fleet.ControlResponse{OK: true, Message: "applied all components"}
	case fleet.ActionApplyCaddy:
		if s.orch == nil {
			return fleet.ControlResponse{Message: "orchestrator disabled"}
		}
		return toControlRes(s.orch.ApplyCaddy(r.Context()))
	case fleet.ActionApplyDNS:
		if s.orch == nil {
			return fleet.ControlResponse{Message: "orchestrator disabled"}
		}
		return toControlRes(s.orch.ApplyCoreDNS(r.Context()))
	case fleet.ActionApplyMaddy:
		if s.orch == nil {
			return fleet.ControlResponse{Message: "orchestrator disabled"}
		}
		return toControlRes(s.orch.ApplyMaddy(r.Context()))
	case fleet.ActionApplySystemd:
		if s.orch == nil {
			return fleet.ControlResponse{Message: "orchestrator disabled"}
		}
		return toControlRes(s.orch.ApplySystemd(r.Context()))
	default:
		return fleet.ControlResponse{Message: "unknown action " + action}
	}
}

func toControlRes(res orchestrator.Result) fleet.ControlResponse {
	return fleet.ControlResponse{OK: res.OK, Message: res.Message}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
