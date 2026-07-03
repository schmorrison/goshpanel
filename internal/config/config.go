// Package config loads GoshPanel configuration from environment variables
// with sane defaults. GoshPanel deliberately avoids config-file dependencies:
// every setting is an environment variable prefixed with GOSHPANEL_.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds all runtime settings for the panel.
type Config struct {
	// Addr is the listen address for the HTTP server, e.g. ":4674".
	Addr string
	// DataDir holds the JSON state store, backups and generated configs.
	DataDir string
	// FilesRoot is the sandbox root for the file manager.
	FilesRoot string
	// LogSources is an allowlist of log files exposed by the log viewer.
	LogSources []string
	// BootstrapUser and BootstrapPassword create the first admin account
	// when the store contains no users.
	BootstrapUser     string
	BootstrapPassword string
	// SessionTTLMinutes controls how long a login session stays valid.
	SessionTTLMinutes int
	// CommandRunnerEnabled toggles the /terminal command runner module.
	CommandRunnerEnabled bool

	// Orchestrator paths — configs are written here and reloaded via process calls.
	OrchestratorEnabled bool
	CaddyConfigPath     string
	CoreDNSConfigDir    string
	MaddyConfigPath     string
	SystemdUnitDir      string
	// AutoApply triggers orchestrator reload after domain/DNS/email changes.
	AutoApply bool

	// DockerEnabled toggles the Docker management module (uses docker CLI).
	DockerEnabled bool

	// FunctionsEnabled toggles HTTP-triggered micro functions.
	FunctionsEnabled bool

	// FleetMode: standalone, controller, worker, or both.
	FleetMode            string
	FleetToken           string // agent API bearer token for this instance
	FleetNodeName        string // identity reported in telemetry
	FleetControllerURL   string // worker push target (http://controller:4674)
	FleetEnrollSecret    string // controller HMAC secret for enrollment JWTs
	FleetEnrollToken     string // worker enrollment JWT (one-time join)
	FleetPublicURL       string // worker URL reported during enrollment
	FleetIntervalSeconds int

	// MetricsEnabled records local metric samples for /metrics history.
	MetricsEnabled bool

	// SSLCertDir is scanned by the SSL/TLS status page.
	SSLCertDir string

	// AnalyticsEnabled ingests Caddy JSON access logs for /analytics.
	AnalyticsEnabled bool
	CaddyAccessLog   string

	// SFTPEnabled starts a sandboxed SFTP server for panel users.
	SFTPEnabled    bool
	SFTPAddr       string
	SFTPHostKeyPath string
}

// Load reads configuration from the given environment lookup function.
// Pass os.LookupEnv in production; tests can inject their own.
func Load(lookup func(string) (string, bool)) (Config, error) {
	get := func(key, def string) string {
		if v, ok := lookup("GOSHPANEL_" + key); ok && v != "" {
			return v
		}
		return def
	}

	cfg := Config{
		Addr:              get("ADDR", ":4674"),
		DataDir:           get("DATA_DIR", "data"),
		FilesRoot:         get("FILES_ROOT", filepath.Join("data", "workspace")),
		BootstrapUser:     get("BOOTSTRAP_USER", "admin"),
		BootstrapPassword: get("BOOTSTRAP_PASSWORD", ""),
	}

	ttl := get("SESSION_TTL_MINUTES", "720")
	n, err := strconv.Atoi(ttl)
	if err != nil || n <= 0 {
		return Config{}, fmt.Errorf("invalid GOSHPANEL_SESSION_TTL_MINUTES %q", ttl)
	}
	cfg.SessionTTLMinutes = n

	if raw := get("LOG_SOURCES", ""); raw != "" {
		for _, p := range strings.Split(raw, ",") {
			if p = strings.TrimSpace(p); p != "" {
				cfg.LogSources = append(cfg.LogSources, p)
			}
		}
	}

	runner := get("COMMAND_RUNNER", "true")
	b, err := strconv.ParseBool(runner)
	if err != nil {
		return Config{}, fmt.Errorf("invalid GOSHPANEL_COMMAND_RUNNER %q", runner)
	}
	cfg.CommandRunnerEnabled = b

	cfg.OrchestratorEnabled = parseBool(get("ORCHESTRATOR", "true"), "ORCHESTRATOR", &err)
	cfg.CaddyConfigPath = get("CADDY_CONFIG", filepath.Join(cfg.DataDir, "generated", "Caddyfile"))
	cfg.CoreDNSConfigDir = get("COREDNS_DIR", filepath.Join(cfg.DataDir, "generated", "coredns"))
	cfg.MaddyConfigPath = get("MADDY_CONFIG", filepath.Join(cfg.DataDir, "generated", "maddy.conf"))
	cfg.SystemdUnitDir = get("SYSTEMD_UNIT_DIR", filepath.Join(cfg.DataDir, "generated", "systemd"))
	cfg.AutoApply = parseBool(get("AUTO_APPLY", "false"), "AUTO_APPLY", &err)

	cfg.DockerEnabled = parseBool(get("DOCKER", "true"), "DOCKER", &err)
	cfg.FunctionsEnabled = parseBool(get("FUNCTIONS", "true"), "FUNCTIONS", &err)

	cfg.FleetMode = strings.ToLower(get("FLEET_MODE", "standalone"))
	cfg.FleetToken = get("FLEET_TOKEN", "")
	cfg.FleetNodeName = get("FLEET_NODE_NAME", "")
	if cfg.FleetNodeName == "" {
		cfg.FleetNodeName, _ = os.Hostname()
	}
	cfg.FleetControllerURL = get("FLEET_CONTROLLER_URL", "")
	cfg.FleetEnrollSecret = get("FLEET_ENROLL_SECRET", "")
	cfg.FleetEnrollToken = get("FLEET_ENROLL_TOKEN", "")
	cfg.FleetPublicURL = get("FLEET_PUBLIC_URL", "")
	intervalStr := get("FLEET_INTERVAL_SECONDS", "60")
	nInterval, aerr := strconv.Atoi(intervalStr)
	if aerr != nil || nInterval <= 0 {
		err = fmt.Errorf("invalid GOSHPANEL_FLEET_INTERVAL_SECONDS %q", intervalStr)
	} else {
		cfg.FleetIntervalSeconds = nInterval
	}
	cfg.MetricsEnabled = parseBool(get("METRICS", "true"), "METRICS", &err)
	cfg.SSLCertDir = get("SSL_CERT_DIR", filepath.Join(cfg.DataDir, "certs"))
	cfg.AnalyticsEnabled = parseBool(get("ANALYTICS", "true"), "ANALYTICS", &err)
	cfg.CaddyAccessLog = get("CADDY_ACCESS_LOG", filepath.Join(cfg.DataDir, "generated", "caddy", "access.log"))
	cfg.SFTPEnabled = parseBool(get("SFTP", "false"), "SFTP", &err)
	cfg.SFTPAddr = get("SFTP_ADDR", ":2222")
	cfg.SFTPHostKeyPath = get("SFTP_HOST_KEY", filepath.Join(cfg.DataDir, "sftp_host_key"))

	if err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func parseBool(raw, key string, err *error) bool {
	if *err != nil {
		return false
	}
	b, e := strconv.ParseBool(raw)
	if e != nil {
		*err = fmt.Errorf("invalid GOSHPANEL_%s %q", key, raw)
	}
	return b
}

// LoadFromEnv is a convenience wrapper over Load(os.LookupEnv).
func LoadFromEnv() (Config, error) { return Load(os.LookupEnv) }
