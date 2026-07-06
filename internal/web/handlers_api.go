package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/schmorrison/goshpanel/internal/api"
	"github.com/schmorrison/goshpanel/internal/crypto"
	"github.com/schmorrison/goshpanel/internal/fleet"
	"github.com/schmorrison/goshpanel/internal/system"
)

func (s *Server) apiAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authz := r.Header.Get("Authorization")
		if !strings.HasPrefix(authz, "Bearer ") {
			api.WriteError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		token := strings.TrimPrefix(authz, "Bearer ")
		hash := api.HashToken(token)
		tok, err := s.store.APITokenByHash(hash)
		if err != nil {
			api.WriteError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		_ = s.store.TouchAPIToken(tok.ID)
		next(w, r)
	}
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	api.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if _, err := s.store.CountUsers(); err != nil {
		api.WriteError(w, http.StatusServiceUnavailable, "store unavailable")
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) handleAPIPage(w http.ResponseWriter, r *http.Request) {
	tokens, _ := s.store.APITokens()
	s.render(w, r, "api.html", "API & Explorer", "api", map[string]any{
		"Tokens": tokens,
		"Routes": apiRouteCatalog(),
	})
}

func (s *Server) handleAPITokenCreate(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		redirectError(w, r, "/api", fmt.Errorf("name required"))
		return
	}
	raw, err := crypto.RandomToken()
	if err != nil {
		redirectError(w, r, "/api", err)
		return
	}
	if _, err := s.store.CreateAPIToken(name, api.HashToken(raw), r.FormValue("role")); err != nil {
		redirectError(w, r, "/api", err)
		return
	}
	s.audit(r, "api.token.create", name)
	redirectFlash(w, r, "/api?flash_token="+raw, "Token created — copy it now")
}

func (s *Server) handleAPITokenDelete(w http.ResponseWriter, r *http.Request) {
	id, err := formID(r, "id")
	if err != nil {
		redirectError(w, r, "/api", err)
		return
	}
	if err := s.store.DeleteAPIToken(id); err != nil {
		redirectError(w, r, "/api", err)
		return
	}
	redirectFlash(w, r, "/api", "Token revoked")
}

func (s *Server) handleAPIDomains(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.Domains()
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	api.WriteJSON(w, http.StatusOK, list)
}

func (s *Server) handleAPIMetrics(w http.ResponseWriter, r *http.Request) {
	stats := system.Snapshot(s.files.Root())
	api.WriteJSON(w, http.StatusOK, stats)
}

func (s *Server) handleAPIMetricsStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		api.WriteError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	for i := 0; i < 30; i++ {
		stats := system.Snapshot(s.files.Root())
		fmt.Fprintf(w, "data: load=%.2f mem=%.0f disk=%.0f\n\n", stats.Load1, stats.MemUsedPct, stats.DiskUsedPct)
		flusher.Flush()
		select {
		case <-r.Context().Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func (s *Server) handleAPIBackupCreate(w http.ResponseWriter, r *http.Request) {
	name, err := s.backups.Create()
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]string{"name": name})
}

func (s *Server) handleAPIFleetNodes(w http.ResponseWriter, r *http.Request) {
	if s.fleet == nil {
		api.WriteError(w, http.StatusNotFound, "fleet disabled")
		return
	}
	nodes, err := s.store.FleetNodes()
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	api.WriteJSON(w, http.StatusOK, nodes)
}

func (s *Server) handleAPIFleetCommand(w http.ResponseWriter, r *http.Request) {
	if s.fleet == nil {
		api.WriteError(w, http.StatusNotFound, "fleet disabled")
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var req fleet.ControlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := s.fleet.SendCommand(r.Context(), id, req)
	if err != nil {
		api.WriteError(w, http.StatusBadGateway, err.Error())
		return
	}
	api.WriteJSON(w, http.StatusOK, res)
}

func (s *Server) handleFleetLogsIngest(w http.ResponseWriter, r *http.Request) {
	nodeName := r.Header.Get("X-Fleet-Node")
	if nodeName == "" {
		api.WriteError(w, http.StatusBadRequest, "X-Fleet-Node required")
		return
	}
	node, err := s.store.FleetNodeByName(nodeName)
	if err != nil {
		api.WriteError(w, http.StatusNotFound, "unknown node")
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err := s.store.AppendFleetLog(node.ID, r.URL.Query().Get("source"), string(body)); err != nil {
		api.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func apiRouteCatalog() []map[string]string {
	return []map[string]string{
		{"method": "GET", "path": "/healthz", "auth": "none", "desc": "Liveness probe"},
		{"method": "GET", "path": "/readyz", "auth": "none", "desc": "Readiness probe"},
		{"method": "GET", "path": "/api/v1/domains", "auth": "bearer", "desc": "List domains"},
		{"method": "GET", "path": "/api/v1/metrics", "auth": "bearer", "desc": "Current system stats"},
		{"method": "GET", "path": "/api/v1/metrics/stream", "auth": "bearer", "desc": "SSE metrics ticker (60s)"},
		{"method": "POST", "path": "/api/v1/backups", "auth": "bearer", "desc": "Create backup"},
		{"method": "GET", "path": "/api/v1/fleet/nodes", "auth": "bearer", "desc": "List fleet nodes"},
		{"method": "POST", "path": "/api/v1/fleet/nodes/{id}/command", "auth": "bearer", "desc": "Remote fleet command"},
	}
}

// HashConfig returns SHA-256 of config bytes for drift detection.
func HashConfig(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
