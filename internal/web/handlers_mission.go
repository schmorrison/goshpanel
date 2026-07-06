package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/schmorrison/goshpanel/internal/fleet"
	"github.com/schmorrison/goshpanel/internal/store"
	"github.com/schmorrison/goshpanel/internal/system"
)

type missionNode struct {
	Node      store.FleetNode
	Online    bool
	Telemetry *fleet.Telemetry
	Drift     []string
}

type missionData struct {
	Stats       system.Stats
	Alerts      []store.Alert
	Nodes       []missionNode
	Incident    bool
	FleetActive bool
}

func (s *Server) handleMissionPage(w http.ResponseWriter, r *http.Request) {
	stats := system.Snapshot(s.files.Root())
	alerts, _ := s.store.RecentAlerts(20)
	incident, _ := s.store.IncidentMode()
	data := missionData{Stats: stats, Alerts: alerts, Incident: incident, FleetActive: s.fleet != nil}

	if s.fleet != nil {
		nodes, _ := s.store.FleetNodes()
		for _, n := range nodes {
			row := missionNode{Node: n, Online: nodeOnline(n.LastSeenAt, s.cfg.FleetIntervalSeconds)}
			if sample, err := s.store.LatestFleetTelemetry(n.ID); err == nil {
				var t fleet.Telemetry
				if json.Unmarshal([]byte(sample.Payload), &t) == nil {
					row.Telemetry = &t
				}
			}
			localHash := s.localConfigHash()
			remote, _ := s.store.NodeConfigHashes(n.ID)
			for comp, h := range localHash {
				if rh, ok := remote[comp]; ok && rh != h {
					row.Drift = append(row.Drift, comp)
				}
			}
			data.Nodes = append(data.Nodes, row)
		}
	}
	s.render(w, r, "mission.html", "Mission Control", "mission", data)
}

func (s *Server) handleIncidentToggle(w http.ResponseWriter, r *http.Request) {
	on := r.FormValue("enable") == "1"
	if err := s.store.SetIncidentMode(on); err != nil {
		redirectError(w, r, "/mission", err)
		return
	}
	if on {
		if _, err := s.backups.Create(); err == nil {
			_ = s.store.CreateAlert("incident", "Incident mode: backup snapshot created")
		}
		_ = s.store.CreateAlert("incident", "Incident mode activated")
	} else {
		_ = s.store.CreateAlert("incident", "Incident mode cleared")
	}
	s.audit(r, "incident.toggle", strconv.FormatBool(on))
	redirectFlash(w, r, "/mission", fmt.Sprintf("Incident mode: %v", on))
}

func (s *Server) handleFleetRollingDeploy(w http.ResponseWriter, r *http.Request) {
	if s.fleet == nil {
		redirectError(w, r, "/mission", fmt.Errorf("fleet controller required"))
		return
	}
	action := r.FormValue("action")
	stack := r.FormValue("stack")
	nodes, _ := s.store.FleetNodes()
	var msgs []string
	for _, n := range nodes {
		req := fleet.ControlRequest{Action: action, Params: map[string]string{"stack": stack}}
		res, err := s.fleet.SendCommand(r.Context(), n.ID, req)
		if err != nil {
			msgs = append(msgs, n.Name+": "+err.Error())
			continue
		}
		msgs = append(msgs, n.Name+": "+res.Message)
	}
	redirectFlash(w, r, "/mission", fmt.Sprintf("Rolling deploy: %v", msgs))
}

func (s *Server) handleFleetBackupAll(w http.ResponseWriter, r *http.Request) {
	if s.fleet == nil {
		redirectError(w, r, "/mission", fmt.Errorf("fleet controller required"))
		return
	}
	nodes, _ := s.store.FleetNodes()
	for _, n := range nodes {
		_, _ = s.fleet.SendCommand(r.Context(), n.ID, fleet.ControlRequest{Action: fleet.ActionBackupCreate})
	}
	redirectFlash(w, r, "/mission", "Backup create sent to all nodes")
}

func (s *Server) handleFleetLogsPage(w http.ResponseWriter, r *http.Request) {
	nodeID, _ := strconv.ParseInt(r.URL.Query().Get("node"), 10, 64)
	logs, _ := s.store.FleetLogs(nodeID, 50)
	nodes, _ := s.store.FleetNodes()
	s.render(w, r, "fleet_logs.html", "Fleet Logs", "fleet", map[string]any{
		"Logs": logs, "Nodes": nodes, "Selected": nodeID,
	})
}

func (s *Server) localConfigHash() map[string]string {
	out := map[string]string{}
	if s.orch == nil {
		return out
	}
	if data, err := readFileIfExists(s.cfg.CaddyConfigPath); err == nil {
		out["caddy"] = hashBytes(data)
	}
	if data, err := readFileIfExists(s.cfg.MaddyConfigPath); err == nil {
		out["maddy"] = hashBytes(data)
	}
	return out
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func readFileIfExists(path string) ([]byte, error) {
	return os.ReadFile(path)
}
