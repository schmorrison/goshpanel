package web

import (
	"errors"
	"html/template"
	"net/http"

	"github.com/schmorrison/goshpanel/internal/auth"
	"github.com/schmorrison/goshpanel/internal/metricsviz"
	"github.com/schmorrison/goshpanel/internal/system"
)

// --- login / logout ---

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "login.html", "Sign in", "", nil)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	sess, err := s.auth.Login(r.FormValue("username"), r.FormValue("password"))
	if err != nil {
		if !errors.Is(err, auth.ErrInvalidCredentials) {
			s.log.Error("login failed", "err", err)
		}
		redirectError(w, r, "/login", auth.ErrInvalidCredentials)
		return
	}
	s.setSessionCookie(w, sess)
	if err := s.store.AppendAudit(r.FormValue("username"), "login", "signed in"); err != nil {
		s.log.Error("audit append failed", "err", err)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		if err := s.auth.Logout(cookie.Value); err != nil {
			s.log.Error("logout failed", "err", err)
		}
	}
	s.clearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// --- dashboard ---

type dashboardData struct {
	Stats      system.Stats
	Counts     map[string]int
	FilesRoot  string
	LoadSpark  template.HTML
	MemSpark   template.HTML
	DiskSpark  template.HTML
	HasHistory bool
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	counts := map[string]int{}
	if domains, err := s.store.Domains(); err == nil {
		counts["domains"] = len(domains)
	}
	if boxes, err := s.store.Mailboxes(); err == nil {
		counts["mailboxes"] = len(boxes)
	}
	if jobs, err := s.store.CronJobs(); err == nil {
		counts["cron"] = len(jobs)
	}
	if conns, err := s.store.DatabaseConns(); err == nil {
		counts["databases"] = len(conns)
	}

	stats := system.Snapshot(s.files.Root())
	data := dashboardData{
		Stats:     stats,
		Counts:    counts,
		FilesRoot: s.files.Root(),
	}

	if samples, err := s.store.MetricSamples(48); err == nil && len(samples) > 1 {
		data.HasHistory = true
		loadYMax := metricsviz.LoadYMax(samples, stats.NumCPU)
		data.LoadSpark = metricsviz.Sparkline(metricsviz.SamplesLoad(samples), loadYMax, 280, 64, "#0f766e")
		data.MemSpark = metricsviz.Sparkline(metricsviz.SamplesMem(samples), 100, 280, 64, "#2563eb")
		data.DiskSpark = metricsviz.Sparkline(metricsviz.SamplesDisk(samples), 100, 280, 64, "#7c3aed")
	}

	s.render(w, r, "dashboard.html", "Dashboard", "dashboard", data)
}
