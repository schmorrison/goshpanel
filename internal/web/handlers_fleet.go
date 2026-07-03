package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/schmorrison/goshpanel/internal/fleet"
	"github.com/schmorrison/goshpanel/internal/store"
)

type fleetNodeRow struct {
	Node      store.FleetNode
	Telemetry *fleet.Telemetry
	Commands  []store.FleetCommand
}

type fleetData struct {
	Mode      string
	NodeName  string
	Nodes     []fleetNodeRow
	Selected  int64
	PushURL   string
	Interval  int
}

func (s *Server) handleFleetPage(w http.ResponseWriter, r *http.Request) {
	if s.fleet == nil {
		s.render(w, r, "fleet.html", "Fleet", "fleet", fleetData{
			Mode: string(fleet.ParseMode(s.cfg.FleetMode)),
		})
		return
	}
	nodes, err := s.store.FleetNodes()
	if err != nil {
		redirectError(w, r, "/", err)
		return
	}
	rows := make([]fleetNodeRow, 0, len(nodes))
	for _, n := range nodes {
		row := fleetNodeRow{Node: n}
		if sample, err := s.store.LatestFleetTelemetry(n.ID); err == nil {
			var t fleet.Telemetry
			if json.Unmarshal([]byte(sample.Payload), &t) == nil {
				row.Telemetry = &t
			}
		}
		if cmds, err := s.store.FleetCommands(n.ID, 5); err == nil {
			row.Commands = cmds
		}
		rows = append(rows, row)
	}
	sel, _ := strconv.ParseInt(r.URL.Query().Get("node"), 10, 64)
	s.render(w, r, "fleet.html", "Fleet", "fleet", fleetData{
		Mode:     string(fleet.ParseMode(s.cfg.FleetMode)),
		NodeName: s.cfg.FleetNodeName,
		Nodes:    rows,
		Selected: sel,
		PushURL:  s.cfg.FleetControllerURL,
		Interval: s.cfg.FleetIntervalSeconds,
	})
}

func (s *Server) handleFleetNodeCreate(w http.ResponseWriter, r *http.Request) {
	if s.fleet == nil {
		redirectError(w, r, "/fleet", moduleDisabled("fleet controller"))
		return
	}
	name := r.FormValue("name")
	url := r.FormValue("base_url")
	token := r.FormValue("token")
	if name == "" || url == "" || token == "" {
		redirectError(w, r, "/fleet", errRequiredFields)
		return
	}
	if _, err := s.store.CreateFleetNode(name, url, token); err != nil {
		redirectError(w, r, "/fleet", err)
		return
	}
	s.audit(r, "fleet.node.create", name)
	redirectFlash(w, r, "/fleet", "Node registered: "+name)
}

func (s *Server) handleFleetNodeDelete(w http.ResponseWriter, r *http.Request) {
	id, err := formID(r, "id")
	if err != nil {
		redirectError(w, r, "/fleet", err)
		return
	}
	if err := s.store.DeleteFleetNode(id); err != nil {
		redirectError(w, r, "/fleet", err)
		return
	}
	s.audit(r, "fleet.node.delete", strconv.FormatInt(id, 10))
	redirectFlash(w, r, "/fleet", "Node removed")
}

func (s *Server) handleFleetPoll(w http.ResponseWriter, r *http.Request) {
	if s.fleet == nil {
		redirectError(w, r, "/fleet", moduleDisabled("fleet controller"))
		return
	}
	if id, err := formID(r, "id"); err == nil {
		node, err := s.store.FleetNodeByID(id)
		if err != nil {
			redirectError(w, r, "/fleet", err)
			return
		}
		if err := s.fleet.PollNode(r.Context(), node); err != nil {
			redirectError(w, r, "/fleet", err)
			return
		}
	} else {
		for _, err := range s.fleet.PollAll(r.Context()) {
			if err != nil {
				redirectError(w, r, "/fleet", err)
				return
			}
		}
	}
	s.audit(r, "fleet.poll", "manual")
	redirectFlash(w, r, "/fleet", "Telemetry updated")
}

func (s *Server) handleFleetCommand(w http.ResponseWriter, r *http.Request) {
	if s.fleet == nil {
		redirectError(w, r, "/fleet", moduleDisabled("fleet controller"))
		return
	}
	id, err := formID(r, "node_id")
	if err != nil {
		redirectError(w, r, "/fleet", err)
		return
	}
	action := r.FormValue("action")
	res, err := s.fleet.SendCommand(r.Context(), id, action)
	if err != nil {
		redirectError(w, r, "/fleet?node="+strconv.FormatInt(id, 10), err)
		return
	}
	s.audit(r, "fleet.command", action)
	redirectFlash(w, r, "/fleet?node="+strconv.FormatInt(id, 10), res.Message)
}

// nodeOnline reports whether a node was seen within 2x the fleet interval.
func nodeOnline(lastSeen *time.Time, intervalSec int) bool {
	if lastSeen == nil {
		return false
	}
	grace := time.Duration(intervalSec*2) * time.Second
	return time.Since(*lastSeen) < grace
}
