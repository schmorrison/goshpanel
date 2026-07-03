// Package web wires every module into an HTTP server using only
// net/http (Go 1.22 pattern routing) and html/template.
package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/schmorrison/goshpanel/internal/auth"
	"github.com/schmorrison/goshpanel/internal/backups"
	"github.com/schmorrison/goshpanel/internal/config"
	"github.com/schmorrison/goshpanel/internal/docker"
	"github.com/schmorrison/goshpanel/internal/files"
	"github.com/schmorrison/goshpanel/internal/fleet"
	"github.com/schmorrison/goshpanel/internal/fn"
	"github.com/schmorrison/goshpanel/internal/logs"
	"github.com/schmorrison/goshpanel/internal/orchestrator"
	"github.com/schmorrison/goshpanel/internal/runner"
	"github.com/schmorrison/goshpanel/internal/security"
	"github.com/schmorrison/goshpanel/internal/store"
	"github.com/schmorrison/goshpanel/internal/system"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// Server holds every service and renders the panel UI.
type Server struct {
	cfg     config.Config
	log     *slog.Logger
	store   *store.Store
	auth    *auth.Service
	files   *files.Service
	backups *backups.Service
	logs    *logs.Service
	blocker *security.Blocker
	runner  *runner.Service
	orch    *orchestrator.Service
	docker  *docker.Service
	fns     *fn.Service
	fleet   *fleet.Controller
	collector *fleet.Collector

	tmpl *template.Template
	mux  *http.ServeMux
}

// New builds the server and its routes.
func New(cfg config.Config, logger *slog.Logger, st *store.Store) (*Server, error) {
	fileSvc, err := files.New(cfg.FilesRoot)
	if err != nil {
		return nil, err
	}
	backupSvc, err := backups.New(fileSvc.Root(), filepath.Join(cfg.DataDir, "backups"))
	if err != nil {
		return nil, err
	}
	blocker, err := security.NewBlocker(st)
	if err != nil {
		return nil, err
	}

	funcs := template.FuncMap{
		"humanKB":    system.HumanKB,
		"humanBytes": func(n int64) string { return system.HumanKB(uint64((n + 1023) / 1024)) },
		"timefmt":    func(t time.Time) string { return t.Format("2006-01-02 15:04") },
		"timefmtPtr": func(t *time.Time) string {
			if t == nil {
				return "—"
			}
			return t.Format("2006-01-02 15:04")
		},
		"durfmt":     formatDuration,
		"printf":     fmt.Sprintf,
		"nodeOnline": func(last *time.Time, interval int) bool {
			if last == nil {
				return false
			}
			return time.Since(*last) < time.Duration(interval*2)*time.Second
		},
	}
	tmpl, err := template.New("").Funcs(funcs).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	s := &Server{
		cfg:     cfg,
		log:     logger,
		store:   st,
		auth:    auth.New(st, time.Duration(cfg.SessionTTLMinutes)*time.Minute),
		files:   fileSvc,
		backups: backupSvc,
		logs:    logs.New(cfg.LogSources),
		blocker: blocker,
		runner:  runner.New(fileSvc.Root(), 60*time.Second, cfg.CommandRunnerEnabled),
		orch: orchestrator.New(st, orchestrator.Paths{
			CaddyConfig: cfg.CaddyConfigPath,
			CoreDNSDir:  cfg.CoreDNSConfigDir,
			MaddyConfig: cfg.MaddyConfigPath,
			SystemdDir:  cfg.SystemdUnitDir,
		}),
		tmpl: tmpl,
		mux:  http.NewServeMux(),
	}
	if cfg.DockerEnabled {
		if d, err := docker.New(); err == nil {
			s.docker = d
		} else {
			logger.Info("docker module unavailable", "err", err)
		}
	}
	if cfg.FunctionsEnabled {
		fnSvc, err := fn.New(st, filepath.Join(cfg.DataDir, "functions"))
		if err != nil {
			return nil, err
		}
		s.fns = fnSvc
	}
	s.collector = fleet.NewCollector(st, cfg.FleetNodeName, fileSvc.Root())
	if fleet.IsController(fleet.ParseMode(cfg.FleetMode)) {
		s.fleet = fleet.NewController(st)
	}

	if err := s.auth.Bootstrap(cfg.BootstrapUser, cfg.BootstrapPassword); err != nil {
		return nil, err
	}
	s.routes()
	return s, nil
}

func (s *Server) routes() {
	static, _ := fs.Sub(staticFS, "static")
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))

	s.mux.HandleFunc("GET /login", s.handleLoginPage)
	s.mux.HandleFunc("POST /login", s.handleLogin)
	s.mux.HandleFunc("POST /logout", s.requireAuth(s.handleLogout))

	s.mux.HandleFunc("GET /{$}", s.requireAuth(s.handleDashboard))

	s.mux.HandleFunc("GET /files", s.requireAuth(s.handleFilesPage))
	s.mux.HandleFunc("GET /files/view", s.requireAuth(s.handleFileView))
	s.mux.HandleFunc("GET /files/download", s.requireAuth(s.handleFileDownload))
	s.mux.HandleFunc("POST /files/save", s.requireAuth(s.handleFileSave))
	s.mux.HandleFunc("POST /files/upload", s.requireAuth(s.handleFileUpload))
	s.mux.HandleFunc("POST /files/mkdir", s.requireAuth(s.handleFileMkdir))
	s.mux.HandleFunc("POST /files/rename", s.requireAuth(s.handleFileRename))
	s.mux.HandleFunc("POST /files/delete", s.requireAuth(s.handleFileDelete))

	s.mux.HandleFunc("GET /domains", s.requireAuth(s.handleDomainsPage))
	s.mux.HandleFunc("POST /domains/create", s.requireAuth(s.handleDomainCreate))
	s.mux.HandleFunc("POST /domains/delete", s.requireAuth(s.handleDomainDelete))
	s.mux.HandleFunc("GET /domains/caddyfile", s.requireAuth(s.handleCaddyfile))
	s.mux.HandleFunc("GET /domains/{id}/zone", s.requireAuth(s.handleZoneFile))
	s.mux.HandleFunc("POST /dns/create", s.requireAuth(s.handleDNSCreate))
	s.mux.HandleFunc("POST /dns/delete", s.requireAuth(s.handleDNSDelete))

	s.mux.HandleFunc("GET /databases", s.requireAuth(s.handleDatabasesPage))
	s.mux.HandleFunc("POST /databases/create", s.requireAuth(s.handleDatabaseCreate))
	s.mux.HandleFunc("POST /databases/delete", s.requireAuth(s.handleDatabaseDelete))
	s.mux.HandleFunc("POST /databases/query", s.requireAuth(s.handleDatabaseQuery))

	s.mux.HandleFunc("GET /email", s.requireAuth(s.handleEmailPage))
	s.mux.HandleFunc("POST /email/mailboxes/create", s.requireAuth(s.handleMailboxCreate))
	s.mux.HandleFunc("POST /email/mailboxes/delete", s.requireAuth(s.handleMailboxDelete))
	s.mux.HandleFunc("POST /email/forwarders/create", s.requireAuth(s.handleForwarderCreate))
	s.mux.HandleFunc("POST /email/forwarders/delete", s.requireAuth(s.handleForwarderDelete))
	s.mux.HandleFunc("GET /email/maddy.conf", s.requireAuth(s.handleMaddyConfig))

	s.mux.HandleFunc("GET /cron", s.requireAuth(s.handleCronPage))
	s.mux.HandleFunc("POST /cron/create", s.requireAuth(s.handleCronCreate))
	s.mux.HandleFunc("POST /cron/delete", s.requireAuth(s.handleCronDelete))
	s.mux.HandleFunc("GET /cron/crontab", s.requireAuth(s.handleCrontab))
	s.mux.HandleFunc("POST /cron/apply", s.requireAuth(s.handleCronApply))

	s.mux.HandleFunc("GET /backups", s.requireAuth(s.handleBackupsPage))
	s.mux.HandleFunc("POST /backups/create", s.requireAuth(s.handleBackupCreate))
	s.mux.HandleFunc("POST /backups/restore", s.requireAuth(s.handleBackupRestore))
	s.mux.HandleFunc("POST /backups/delete", s.requireAuth(s.handleBackupDelete))
	s.mux.HandleFunc("GET /backups/download", s.requireAuth(s.handleBackupDownload))

	s.mux.HandleFunc("GET /logs", s.requireAuth(s.handleLogsPage))

	s.mux.HandleFunc("GET /security", s.requireAuth(s.handleSecurityPage))
	s.mux.HandleFunc("POST /security/ip/create", s.requireAuth(s.handleIPRuleCreate))
	s.mux.HandleFunc("POST /security/ip/delete", s.requireAuth(s.handleIPRuleDelete))
	s.mux.HandleFunc("POST /security/users/create", s.requireAuth(s.handleUserCreate))
	s.mux.HandleFunc("POST /security/users/delete", s.requireAuth(s.handleUserDelete))
	s.mux.HandleFunc("POST /security/password", s.requireAuth(s.handlePasswordChange))

	s.mux.HandleFunc("GET /terminal", s.requireAuth(s.handleTerminalPage))
	s.mux.HandleFunc("POST /terminal/run", s.requireAuth(s.handleTerminalRun))

	s.mux.HandleFunc("GET /orchestrator", s.requireAuth(s.handleOrchestratorPage))
	s.mux.HandleFunc("POST /orchestrator/apply", s.requireAuth(s.handleOrchestratorApply))
	s.mux.HandleFunc("POST /orchestrator/apply/{component}", s.requireAuth(s.handleOrchestratorApplyOne))
	s.mux.HandleFunc("POST /orchestrator/systemd/create", s.requireAuth(s.handleSystemdCreate))
	s.mux.HandleFunc("POST /orchestrator/systemd/delete", s.requireAuth(s.handleSystemdDelete))

	s.mux.HandleFunc("GET /docker", s.requireAuth(s.handleDockerPage))
	s.mux.HandleFunc("POST /docker/run", s.requireAuth(s.handleDockerRun))
	s.mux.HandleFunc("POST /docker/{action}", s.requireAuth(s.handleDockerAction))
	s.mux.HandleFunc("GET /docker/logs", s.requireAuth(s.handleDockerLogs))
	s.mux.HandleFunc("POST /docker/stacks/create", s.requireAuth(s.handleDockerStackCreate))
	s.mux.HandleFunc("POST /docker/stacks/up", s.requireAuth(s.handleDockerStackUp))
	s.mux.HandleFunc("POST /docker/stacks/down", s.requireAuth(s.handleDockerStackDown))
	s.mux.HandleFunc("POST /docker/stacks/delete", s.requireAuth(s.handleDockerStackDelete))

	s.mux.HandleFunc("GET /functions", s.requireAuth(s.handleFunctionsPage))
	s.mux.HandleFunc("POST /functions/create", s.requireAuth(s.handleFunctionCreate))
	s.mux.HandleFunc("GET /functions/{id}/edit", s.requireAuth(s.handleFunctionEditPage))
	s.mux.HandleFunc("POST /functions/{id}/save", s.requireAuth(s.handleFunctionSave))
	s.mux.HandleFunc("POST /functions/{id}/delete", s.requireAuth(s.handleFunctionDelete))
	s.mux.HandleFunc("POST /functions/{id}/invoke", s.requireAuth(s.handleFunctionInvokePanel))

	// Public function invoke — token auth, no panel session required.
	s.mux.HandleFunc("GET /fn/{name}", s.handleFunctionInvokePublic)
	s.mux.HandleFunc("POST /fn/{name}", s.handleFunctionInvokePublic)

	// Fleet agent API (Bearer GOSHPANEL_FLEET_TOKEN).
	s.mux.HandleFunc("GET /api/v1/fleet/telemetry", s.fleetAuth(s.handleFleetTelemetryAPI))
	s.mux.HandleFunc("POST /api/v1/fleet/control", s.fleetAuth(s.handleFleetControlAPI))
	s.mux.HandleFunc("POST /api/v1/fleet/enroll", s.handleFleetEnrollAPI)
	s.mux.HandleFunc("POST /api/v1/fleet/ingest", s.handleFleetIngestAPI)

	s.mux.HandleFunc("GET /fleet", s.requireAuth(s.handleFleetPage))
	s.mux.HandleFunc("POST /fleet/enroll-token", s.requireAuth(s.handleFleetEnrollTokenGenerate))
	s.mux.HandleFunc("POST /fleet/nodes/create", s.requireAuth(s.handleFleetNodeCreate))
	s.mux.HandleFunc("POST /fleet/nodes/delete", s.requireAuth(s.handleFleetNodeDelete))
	s.mux.HandleFunc("POST /fleet/poll", s.requireAuth(s.handleFleetPoll))
	s.mux.HandleFunc("POST /fleet/command", s.requireAuth(s.handleFleetCommand))

	s.mux.HandleFunc("GET /metrics", s.requireAuth(s.handleMetricsPage))
	s.mux.HandleFunc("GET /ssl", s.requireAuth(s.handleSSLPage))
}

// Handler returns the fully wrapped HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.withBlocker(s.withSecurityHeaders(s.mux))
}

// formatDuration renders a duration like "3d 4h 5m".
func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	days := d / (24 * time.Hour)
	d -= days * 24 * time.Hour
	hours := d / time.Hour
	mins := (d - hours*time.Hour) / time.Minute
	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	parts = append(parts, fmt.Sprintf("%dm", mins))
	return strings.Join(parts, " ")
}
