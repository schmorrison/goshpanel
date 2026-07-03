package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/schmorrison/goshpanel/internal/fleet"
	"github.com/schmorrison/goshpanel/internal/metricsviz"
	"github.com/schmorrison/goshpanel/internal/store"
)

type fleetNodeRow struct {
	Node      store.FleetNode
	Telemetry *fleet.Telemetry
	Commands  []store.FleetCommand
	LoadChart metricsviz.Chart
	MemChart  metricsviz.Chart
	DiskChart metricsviz.Chart
	HasCharts bool
}

type fleetData struct {
	Mode             string
	NodeName         string
	Nodes            []fleetNodeRow
	Selected         int64
	PushURL          string
	Interval         int
	EnrollToken      string
	EnrollEnvBlock   string
	EnrollEnabled    bool
	WorkerEnrolled   bool
	WorkerController string
}

func (s *Server) handleFleetPage(w http.ResponseWriter, r *http.Request) {
	mode := fleet.ParseMode(s.cfg.FleetMode)
	data := fleetData{
		Mode:          string(mode),
		NodeName:      s.cfg.FleetNodeName,
		Interval:      s.cfg.FleetIntervalSeconds,
		EnrollEnabled: fleet.EnrollSecret(s.cfg) != "",
		EnrollToken:   r.URL.Query().Get("enroll_token"),
	}

	if fleet.IsWorker(mode) {
		rt := fleet.ResolveWorkerRuntime(s.cfg)
		data.WorkerEnrolled = rt.Enrolled || (rt.NodeToken != "" && rt.ControllerURL != "")
		data.WorkerController = rt.ControllerURL
		data.PushURL = rt.ControllerURL
	}

	if data.EnrollToken != "" {
		controllerURL := data.WorkerController
		if controllerURL == "" {
			controllerURL = requestBaseURL(r)
		}
		data.EnrollEnvBlock = workerEnvBlock(controllerURL, data.EnrollToken, s.cfg.FleetNodeName)
	}

	if s.fleet == nil {
		s.render(w, r, "fleet.html", "Fleet", "fleet", data)
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
		if history, err := s.store.FleetTelemetryHistory(n.ID, 60); err == nil && len(history) > 1 {
			row.LoadChart, row.MemChart, row.DiskChart = metricsviz.ChartsFromFleetHistory(history, 0)
			row.HasCharts = true
		}
		rows = append(rows, row)
	}
	sel, _ := strconv.ParseInt(r.URL.Query().Get("node"), 10, 64)
	data.Nodes = rows
	data.Selected = sel
	data.PushURL = s.cfg.FleetControllerURL
	if data.EnrollToken != "" && data.EnrollEnvBlock == "" {
		data.EnrollEnvBlock = workerEnvBlock(requestBaseURL(r), data.EnrollToken, "")
	}
	s.render(w, r, "fleet.html", "Fleet", "fleet", data)
}

func (s *Server) handleFleetEnrollTokenGenerate(w http.ResponseWriter, r *http.Request) {
	if s.fleet == nil {
		redirectError(w, r, "/fleet", moduleDisabled("fleet controller"))
		return
	}
	secret := fleet.EnrollSecret(s.cfg)
	if secret == "" {
		redirectError(w, r, "/fleet", fmt.Errorf("set GOSHPANEL_FLEET_ENROLL_SECRET or GOSHPANEL_FLEET_TOKEN to generate enroll tokens"))
		return
	}
	ttlHours, _ := strconv.Atoi(r.FormValue("ttl_hours"))
	if ttlHours <= 0 {
		ttlHours = int(fleet.DefaultEnrollTTL / time.Hour)
	}
	token, err := fleet.SignEnrollToken(secret, time.Duration(ttlHours)*time.Hour, r.FormValue("suggested_name"))
	if err != nil {
		redirectError(w, r, "/fleet", err)
		return
	}
	s.audit(r, "fleet.enroll_token", strconv.Itoa(ttlHours)+"h")
	http.Redirect(w, r, "/fleet?enroll_token="+token, http.StatusSeeOther)
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

func workerEnvBlock(controllerURL, enrollToken, nodeName string) string {
	block := fmt.Sprintf("GOSHPANEL_FLEET_MODE=worker\nGOSHPANEL_FLEET_CONTROLLER_URL=%s\nGOSHPANEL_FLEET_ENROLL_TOKEN=%s",
		controllerURL, enrollToken)
	if nodeName != "" {
		block += "\nGOSHPANEL_FLEET_NODE_NAME=" + nodeName
	}
	block += "\nGOSHPANEL_FLEET_PUBLIC_URL=http://your-worker-host:4674"
	return block
}

// nodeOnline reports whether a node was seen within 2x the fleet interval.
func nodeOnline(lastSeen *time.Time, intervalSec int) bool {
	if lastSeen == nil {
		return false
	}
	grace := time.Duration(intervalSec*2) * time.Second
	return time.Since(*lastSeen) < grace
}
