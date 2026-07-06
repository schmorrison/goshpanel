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
	"github.com/schmorrison/goshpanel/internal/connector"
	"github.com/schmorrison/goshpanel/internal/docker"
	"github.com/schmorrison/goshpanel/internal/files"
	"github.com/schmorrison/goshpanel/internal/fleet"
	"github.com/schmorrison/goshpanel/internal/fn"
	"github.com/schmorrison/goshpanel/internal/logs"
	"github.com/schmorrison/goshpanel/internal/metricsviz"
	"github.com/schmorrison/goshpanel/internal/orchestrator"
	"github.com/schmorrison/goshpanel/internal/runner"
	"github.com/schmorrison/goshpanel/internal/secrets"
	"github.com/schmorrison/goshpanel/internal/security"
	"github.com/schmorrison/goshpanel/internal/store"
	"github.com/schmorrison/goshpanel/internal/system"
	"github.com/schmorrison/goshpanel/internal/webhooks"
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
	connect *connector.Registry
	docker  *docker.Service
	fns     *fn.Service
	fleet   *fleet.Controller
	collector *fleet.Collector
	vault   *secrets.Vault
	hooks   *webhooks.Dispatcher

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
		"gaugeLevel": metricsviz.GaugeLevel,
		"loadUtil":   metricsviz.LoadUtilPct,
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
		connect: connector.NewRegistry(st, cfg),
		tmpl: tmpl,
		mux:  http.NewServeMux(),
	}
	s.orch = orchestrator.New(st, orchestrator.Paths{
		CaddyConfig:    cfg.CaddyConfigPath,
		CaddyAccessLog: cfg.CaddyAccessLog,
		CoreDNSDir:     cfg.CoreDNSConfigDir,
		MaddyConfig:    cfg.MaddyConfigPath,
		SystemdDir:     cfg.SystemdUnitDir,
	}, s.connect)
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

	created, err := s.auth.Bootstrap(cfg.BootstrapUser, cfg.BootstrapPassword)
	if err != nil {
		return nil, err
	}
	if created {
		logger.Info("bootstrap admin created", "username", cfg.BootstrapUser)
	} else {
		reset, err := s.auth.ResetBootstrapPassword(cfg.BootstrapUser, cfg.BootstrapPassword, cfg.BootstrapReset)
		if err != nil {
			return nil, err
		}
		if reset {
			logger.Warn("bootstrap admin password reset", "username", cfg.BootstrapUser)
		} else if cfg.BootstrapPassword != "" {
			logger.Info("bootstrap skipped: users already exist; GOSHPANEL_BOOTSTRAP_PASSWORD is ignored (set GOSHPANEL_BOOTSTRAP_RESET=true to update the admin password, or delete the database to start fresh)")
		}
	}
	if cfg.SecretsKey != "" {
		if v, err := secrets.NewVault(cfg.SecretsKey); err == nil {
			s.vault = v
			s.connect.SetVault(v)
		}
	}
	s.hooks = webhooks.NewDispatcher(st, logger)
	s.routes()
	return s, nil
}

func (s *Server) routes() {
	static, _ := fs.Sub(staticFS, "static")
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))

	s.mux.HandleFunc("GET /login", s.handleLoginPage)
	s.mux.HandleFunc("POST /login", s.handleLogin)
	s.mux.HandleFunc("POST /login/totp", s.handleLoginTOTP)
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
	s.mux.HandleFunc("POST /domains/{id}/deliverability", s.requireAuth(s.handleDeliverability))
	s.mux.HandleFunc("POST /domains/{id}/dkim", s.requireAuth(s.handleDKIMGenerate))

	s.mux.HandleFunc("GET /webmail", s.requireAuth(s.handleWebmailPage))
	s.mux.HandleFunc("POST /webmail/save", s.requireAuth(s.handleWebmailSave))

	s.mux.HandleFunc("GET /installers", s.requireAuth(s.handleInstallersPage))
	s.mux.HandleFunc("POST /installers/{id}/install", s.requireAuth(s.handleInstallerRun))
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
	s.mux.HandleFunc("POST /security/users/files-subdir", s.requireAuth(s.handleUserFilesSubdir))
	s.mux.HandleFunc("POST /security/password", s.requireAuth(s.handlePasswordChange))
	s.mux.HandleFunc("POST /security/2fa/setup", s.requireAuth(s.handle2FASetup))
	s.mux.HandleFunc("POST /security/2fa/enable", s.requireAuth(s.handle2FAEnable))
	s.mux.HandleFunc("POST /security/2fa/disable", s.requireAuth(s.handle2FADisable))

	s.mux.HandleFunc("GET /sftp", s.requireAuth(s.handleSFTPPage))

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
	s.mux.HandleFunc("GET /analytics", s.requireAuth(s.handleAnalyticsPage))
	s.mux.HandleFunc("GET /ssl", s.requireAuth(s.handleSSLPage))

	// Health probes (no auth)
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /readyz", s.handleReadyz)

	// JSON API (bearer token)
	s.mux.HandleFunc("GET /api", s.requireAuth(s.handleAPIPage))
	s.mux.HandleFunc("POST /api/tokens/create", s.requireAuth(s.handleAPITokenCreate))
	s.mux.HandleFunc("POST /api/tokens/delete", s.requireAuth(s.handleAPITokenDelete))
	s.mux.HandleFunc("GET /api/v1/domains", s.apiAuth(s.handleAPIDomains))
	s.mux.HandleFunc("GET /api/v1/metrics", s.apiAuth(s.handleAPIMetrics))
	s.mux.HandleFunc("GET /api/v1/metrics/stream", s.apiAuth(s.handleAPIMetricsStream))
	s.mux.HandleFunc("POST /api/v1/backups", s.apiAuth(s.handleAPIBackupCreate))
	s.mux.HandleFunc("GET /api/v1/fleet/nodes", s.apiAuth(s.handleAPIFleetNodes))
	s.mux.HandleFunc("POST /api/v1/fleet/nodes/{id}/command", s.apiAuth(s.handleAPIFleetCommand))
	s.mux.HandleFunc("POST /api/v1/fleet/logs", s.fleetAuth(s.handleFleetLogsIngest))

	s.mux.HandleFunc("GET /api/v1/short-links", s.apiAuth(s.handleAPIShortLinks))
	s.mux.HandleFunc("POST /api/v1/short-links", s.apiAuth(s.handleAPIShortLinks))
	s.mux.HandleFunc("GET /api/v1/short-links/{id}", s.apiAuth(s.handleAPIShortLinkByID))
	s.mux.HandleFunc("DELETE /api/v1/short-links/{id}", s.apiAuth(s.handleAPIShortLinkByID))
	s.mux.HandleFunc("GET /api/v1/webhooks/outbound", s.apiAuth(s.handleAPIWebhookOutbound))
	s.mux.HandleFunc("POST /api/v1/webhooks/outbound", s.apiAuth(s.handleAPIWebhookOutbound))
	s.mux.HandleFunc("GET /api/v1/webhooks/outbound/{id}", s.apiAuth(s.handleAPIWebhookOutboundByID))
	s.mux.HandleFunc("DELETE /api/v1/webhooks/outbound/{id}", s.apiAuth(s.handleAPIWebhookOutboundByID))
	s.mux.HandleFunc("GET /api/v1/webhooks/outbound/{id}/deliveries", s.apiAuth(s.handleAPIWebhookDeliveries))
	s.mux.HandleFunc("GET /api/v1/webhooks/inbound", s.apiAuth(s.handleAPIWebhookInbound))

	// Service connectors
	s.mux.HandleFunc("GET /connectors", s.requireAuth(s.handleConnectorsPage))
	s.mux.HandleFunc("POST /connectors/create", s.requireAuth(s.handleConnectorCreate))
	s.mux.HandleFunc("POST /connectors/delete", s.requireAuth(s.handleConnectorDelete))
	s.mux.HandleFunc("POST /connectors/ping", s.requireAuth(s.handleConnectorPing))
	s.mux.HandleFunc("POST /connectors/default", s.requireAuth(s.handleConnectorDefault))
	s.mux.HandleFunc("GET /connectors/{id}/edit", s.requireAuth(s.handleConnectorEditPage))
	s.mux.HandleFunc("POST /connectors/{id}/save", s.requireAuth(s.handleConnectorSave))
	s.mux.HandleFunc("GET /connectors/caddy", s.requireAuth(s.handleCaddyConnectorPage))
	s.mux.HandleFunc("POST /connectors/caddy/save", s.requireAuth(s.handleCaddyConnectorSave))
	s.mux.HandleFunc("POST /connectors/caddy/reload", s.requireAuth(s.handleCaddyConnectorReload))
	s.mux.HandleFunc("GET /connectors/coredns", s.requireAuth(s.handleCoreDNSConnectorPage))
	s.mux.HandleFunc("POST /connectors/coredns/save", s.requireAuth(s.handleCoreDNSConnectorSave))
	s.mux.HandleFunc("POST /connectors/coredns/reload", s.requireAuth(s.handleCoreDNSConnectorReload))
	s.mux.HandleFunc("GET /connectors/maddy", s.requireAuth(s.handleMaddyConnectorPage))
	s.mux.HandleFunc("POST /connectors/maddy/save", s.requireAuth(s.handleMaddyConnectorSave))
	s.mux.HandleFunc("POST /connectors/maddy/reload", s.requireAuth(s.handleMaddyConnectorReload))
	s.mux.HandleFunc("POST /databases/provision", s.requireAuth(s.handleDatabaseProvision))

	// URL shortener & webhooks
	s.mux.HandleFunc("GET /short", s.requireAuth(s.handleShortLinksPage))
	s.mux.HandleFunc("POST /short/create", s.requireAuth(s.handleShortLinkCreate))
	s.mux.HandleFunc("POST /short/update", s.requireAuth(s.handleShortLinkUpdate))
	s.mux.HandleFunc("POST /short/delete", s.requireAuth(s.handleShortLinkDelete))
	s.mux.HandleFunc("GET /s/{code}", s.handleShortRedirect)
	s.mux.HandleFunc("GET /webhooks", s.requireAuth(s.handleWebhooksPage))
	s.mux.HandleFunc("POST /webhooks/outbound/create", s.requireAuth(s.handleWebhookOutboundCreate))
	s.mux.HandleFunc("POST /webhooks/outbound/delete", s.requireAuth(s.handleWebhookOutboundDelete))
	s.mux.HandleFunc("POST /webhooks/outbound/test", s.requireAuth(s.handleWebhookOutboundTest))
	s.mux.HandleFunc("POST /webhooks/inbound/create", s.requireAuth(s.handleWebhookInboundCreate))
	s.mux.HandleFunc("POST /webhooks/inbound/delete", s.requireAuth(s.handleWebhookInboundDelete))
	s.mux.HandleFunc("POST /hooks/in/{token}", s.handleInboundWebhook)

	// Tools hub & helpers
	s.mux.HandleFunc("GET /tools", s.requireAuth(s.handleToolsPage))
	s.mux.HandleFunc("GET /jump", s.requireAuth(s.handleJumpPage))
	s.mux.HandleFunc("GET /http", s.requireAuth(s.handleHTTPPage))
	s.mux.HandleFunc("POST /http/collections/create", s.requireAuth(s.handleHTTPCollectionCreate))
	s.mux.HandleFunc("POST /http/requests/save", s.requireAuth(s.handleHTTPRequestSave))
	s.mux.HandleFunc("POST /http/requests/run", s.requireAuth(s.handleHTTPRequestRun))
	s.mux.HandleFunc("GET /bandwidth", s.requireAuth(s.handleBandwidthPage))
	s.mux.HandleFunc("GET /redirects", s.requireAuth(s.handleRedirectsPage))
	s.mux.HandleFunc("POST /redirects/create", s.requireAuth(s.handleRedirectCreate))
	s.mux.HandleFunc("POST /redirects/delete", s.requireAuth(s.handleRedirectDelete))
	s.mux.HandleFunc("POST /aliases/create", s.requireAuth(s.handleAliasCreate))
	s.mux.HandleFunc("POST /aliases/delete", s.requireAuth(s.handleAliasDelete))
	s.mux.HandleFunc("GET /firewall", s.requireAuth(s.handleFirewallPage))
	s.mux.HandleFunc("POST /firewall/create", s.requireAuth(s.handleFirewallCreate))
	s.mux.HandleFunc("POST /firewall/delete", s.requireAuth(s.handleFirewallDelete))
	s.mux.HandleFunc("GET /firewall/download", s.requireAuth(s.handleFirewallDownload))
	s.mux.HandleFunc("GET /ssh", s.requireAuth(s.handleSSHPage))
	s.mux.HandleFunc("POST /ssh/keys/create", s.requireAuth(s.handleSSHKeyCreate))
	s.mux.HandleFunc("POST /ssh/keys/delete", s.requireAuth(s.handleSSHKeyDelete))
	s.mux.HandleFunc("GET /health-checks", s.requireAuth(s.handleHealthChecksPage))
	s.mux.HandleFunc("POST /health-checks/create", s.requireAuth(s.handleHealthCheckCreate))
	s.mux.HandleFunc("POST /health-checks/delete", s.requireAuth(s.handleHealthCheckDelete))
	s.mux.HandleFunc("GET /secrets", s.requireAuth(s.handleSecretsPage))
	s.mux.HandleFunc("POST /secrets/save", s.requireAuth(s.handleSecretSave))
	s.mux.HandleFunc("POST /secrets/delete", s.requireAuth(s.handleSecretDelete))
	s.mux.HandleFunc("GET /git-deploy", s.requireAuth(s.handleGitDeployPage))
	s.mux.HandleFunc("POST /git-deploy/create", s.requireAuth(s.handleGitRepoCreate))
	s.mux.HandleFunc("POST /hooks/git/{id}", s.handleGitWebhook)
	s.mux.HandleFunc("GET /ddns", s.requireAuth(s.handleDDNSPage))
	s.mux.HandleFunc("POST /ddns/create", s.requireAuth(s.handleDDNSCreate))
	s.mux.HandleFunc("POST /ddns/delete", s.requireAuth(s.handleDDNSDelete))
	s.mux.HandleFunc("GET /waf", s.requireAuth(s.handleWAFPage))
	s.mux.HandleFunc("POST /waf/create", s.requireAuth(s.handleWAFCreate))
	s.mux.HandleFunc("POST /waf/delete", s.requireAuth(s.handleWAFDelete))
	s.mux.HandleFunc("GET /theme", s.requireAuth(s.handleThemePage))
	s.mux.HandleFunc("POST /theme/save", s.requireAuth(s.handleThemeSave))
	s.mux.HandleFunc("GET /onboarding", s.requireAuth(s.handleOnboardingPage))
	s.mux.HandleFunc("POST /onboarding/complete", s.requireAuth(s.handleOnboardingComplete))
	s.mux.HandleFunc("GET /webdav", s.requireAuth(s.handleWebDAVPage))
	s.mux.HandleFunc("POST /logs/summary", s.requireAuth(s.handleLogSummary))
	s.mux.HandleFunc("POST /backups/schedules/create", s.requireAuth(s.handleBackupScheduleCreate))
	s.mux.HandleFunc("POST /databases/grants/create", s.requireAuth(s.handleDBGrantCreate))
	s.mux.HandleFunc("POST /functions/schedules/create", s.requireAuth(s.handleFunctionScheduleCreate))

	// Mission control
	s.mux.HandleFunc("GET /mission", s.requireAuth(s.handleMissionPage))
	s.mux.HandleFunc("POST /mission/incident", s.requireAuth(s.handleIncidentToggle))
	s.mux.HandleFunc("POST /mission/fleet/rolling", s.requireAuth(s.handleFleetRollingDeploy))
	s.mux.HandleFunc("POST /mission/fleet/backup-all", s.requireAuth(s.handleFleetBackupAll))
	s.mux.HandleFunc("GET /fleet/logs", s.requireAuth(s.handleFleetLogsPage))
}

// Handler returns the fully wrapped HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.withBlocker(s.withSecurityHeaders(s.mux))
}

// Backups exposes the backup service for background jobs.
func (s *Server) Backups() *backups.Service { return s.backups }

// Functions exposes the micro-functions service for background jobs.
func (s *Server) Functions() *fn.Service { return s.fns }

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
