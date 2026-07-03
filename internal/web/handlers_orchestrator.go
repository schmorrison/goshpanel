package web

import (
	"context"
	"net/http"
	"strconv"

	"github.com/schmorrison/goshpanel/internal/orchestrator"
	"github.com/schmorrison/goshpanel/internal/store"
)

type orchestratorData struct {
	Enabled  bool
	AutoApply bool
	Statuses []store.OrchestratorStatus
	Units    []store.SystemdUnit
	Results  []orchestrator.Result
}

func (s *Server) handleOrchestratorPage(w http.ResponseWriter, r *http.Request) {
	statuses, err := s.store.OrchestratorStatuses()
	if err != nil {
		redirectError(w, r, "/", err)
		return
	}
	units, err := s.store.SystemdUnits()
	if err != nil {
		redirectError(w, r, "/", err)
		return
	}
	s.render(w, r, "orchestrator.html", "Orchestrator", "orchestrator", orchestratorData{
		Enabled:   s.cfg.OrchestratorEnabled,
		AutoApply: s.cfg.AutoApply,
		Statuses:  statuses,
		Units:     units,
	})
}

func (s *Server) handleOrchestratorApply(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.OrchestratorEnabled {
		redirectError(w, r, "/orchestrator", moduleDisabled("orchestrator"))
		return
	}
	results := s.orch.ApplyAll(r.Context())
	s.audit(r, "orchestrator.apply_all", "all components")
	s.renderOrchestratorResults(w, r, results)
}

func (s *Server) handleOrchestratorApplyOne(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.OrchestratorEnabled {
		redirectError(w, r, "/orchestrator", moduleDisabled("orchestrator"))
		return
	}
	var res orchestrator.Result
	switch r.PathValue("component") {
	case "caddy":
		res = s.orch.ApplyCaddy(r.Context())
	case "coredns":
		res = s.orch.ApplyCoreDNS(r.Context())
	case "maddy":
		res = s.orch.ApplyMaddy(r.Context())
	case "systemd":
		res = s.orch.ApplySystemd(r.Context())
	default:
		redirectError(w, r, "/orchestrator", errBadComponent)
		return
	}
	s.audit(r, "orchestrator.apply", res.Component)
	s.renderOrchestratorResults(w, r, []orchestrator.Result{res})
}

func (s *Server) renderOrchestratorResults(w http.ResponseWriter, r *http.Request, results []orchestrator.Result) {
	statuses, _ := s.store.OrchestratorStatuses()
	units, _ := s.store.SystemdUnits()
	s.render(w, r, "orchestrator.html", "Orchestrator", "orchestrator", orchestratorData{
		Enabled:   s.cfg.OrchestratorEnabled,
		AutoApply: s.cfg.AutoApply,
		Statuses:  statuses,
		Units:     units,
		Results:   results,
	})
}

func (s *Server) handleSystemdCreate(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	content := r.FormValue("unit_content")
	if name == "" || content == "" {
		redirectError(w, r, "/orchestrator", errRequiredFields)
		return
	}
	if _, err := s.store.CreateSystemdUnit(name, content, true); err != nil {
		redirectError(w, r, "/orchestrator", err)
		return
	}
	s.audit(r, "orchestrator.systemd.create", name)
	redirectFlash(w, r, "/orchestrator", "Systemd unit added: "+name)
}

func (s *Server) handleSystemdDelete(w http.ResponseWriter, r *http.Request) {
	id, err := formID(r, "id")
	if err != nil {
		redirectError(w, r, "/orchestrator", err)
		return
	}
	if err := s.store.DeleteSystemdUnit(id); err != nil {
		redirectError(w, r, "/orchestrator", err)
		return
	}
	s.audit(r, "orchestrator.systemd.delete", strconv.FormatInt(id, 10))
	redirectFlash(w, r, "/orchestrator", "Systemd unit removed")
}

// maybeAutoApply reloads infrastructure when GOSHPANEL_AUTO_APPLY=true.
func (s *Server) maybeAutoApply(ctx context.Context) {
	if !s.cfg.AutoApply || !s.cfg.OrchestratorEnabled || s.orch == nil {
		return
	}
	go func() {
		_ = s.orch.ApplyAll(ctx)
	}()
}
