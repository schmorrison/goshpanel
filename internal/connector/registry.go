// Package connector bridges GoshPanel to external infrastructure: Caddy in
// Docker, remote Docker daemons, Postgres, MariaDB/MySQL, and similar targets.
package connector

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/schmorrison/goshpanel/internal/config"
	"github.com/schmorrison/goshpanel/internal/store"
)

// Kind identifies a connector type.
type Kind string

const (
	KindCaddy    Kind = "caddy"
	KindDocker   Kind = "docker"
	KindPostgres Kind = "postgres"
	KindMySQL    Kind = "mysql"
)

// Mode describes how the panel reaches the service.
type Mode string

const (
	ModeLocal  Mode = "local"
	ModeDocker Mode = "docker"
)

// CaddyConfig is stored in ServiceConnector.ConfigJSON for kind=caddy.
type CaddyConfig struct {
	Container           string `json:"container"`
	ConfigPathContainer string `json:"config_path_container"`
	ConfigPathHost      string `json:"config_path_host"`
	AccessLogPath       string `json:"access_log_path"`
	DockerHost          string `json:"docker_host"`
}

// DockerConfig is stored for kind=docker.
type DockerConfig struct {
	DockerHost string `json:"docker_host"`
}

// DatabaseConfig is stored for kind=postgres or kind=mysql.
type DatabaseConfig struct {
	Host            string `json:"host"`
	Port            int    `json:"port"`
	AdminUser       string `json:"admin_user"`
	AdminPassword   string `json:"admin_password"`
	DefaultDatabase string `json:"default_database"`
	SSLMode         string `json:"ssl_mode"`
}

// Registry resolves connectors and builds clients.
type Registry struct {
	store  *store.Store
	cfg    config.Config
	docker *dockerFactory
}

// NewRegistry creates a connector registry.
func NewRegistry(st *store.Store, cfg config.Config) *Registry {
	return &Registry{store: st, cfg: cfg, docker: &dockerFactory{}}
}

// List returns all connectors.
func (r *Registry) List() ([]store.ServiceConnector, error) {
	return r.store.ServiceConnectors()
}

// ByID loads a connector by id.
func (r *Registry) ByID(id int64) (store.ServiceConnector, error) {
	return r.store.ServiceConnectorByID(id)
}

// Default returns the default connector for a kind, or ErrNotFound.
func (r *Registry) Default(kind Kind) (store.ServiceConnector, error) {
	return r.store.DefaultServiceConnector(string(kind))
}

// Caddy builds a Caddy client for the connector (or local defaults when id=0).
func (r *Registry) Caddy(c store.ServiceConnector, localPaths CaddyLocalPaths) (*CaddyClient, error) {
	return newCaddyClient(c, localPaths, r.docker)
}

// CaddyLocalPaths supplies on-disk paths for local / fallback mode.
type CaddyLocalPaths struct {
	ConfigPath     string
	AccessLogPath  string
}

// LocalCaddyPaths builds paths from panel config.
func LocalCaddyPaths(cfg config.Config) CaddyLocalPaths {
	return CaddyLocalPaths{
		ConfigPath:    cfg.CaddyConfigPath,
		AccessLogPath: cfg.CaddyAccessLog,
	}
}

// DockerService returns a docker CLI wrapper for the connector.
func (r *Registry) DockerService(c store.ServiceConnector) (*DockerCLI, error) {
	return r.docker.For(c)
}

// DatabaseDSN builds driver + DSN for postgres/mysql connectors.
func DatabaseDSN(kind Kind, cfg DatabaseConfig) (driver, dsn string, err error) {
	switch kind {
	case KindPostgres:
		if cfg.Port == 0 {
			cfg.Port = 5432
		}
		db := cfg.DefaultDatabase
		if db == "" {
			db = "postgres"
		}
		ssl := cfg.SSLMode
		if ssl == "" {
			ssl = "disable"
		}
		return "postgres", fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
			cfg.AdminUser, cfg.AdminPassword, cfg.Host, cfg.Port, db, ssl), nil
	case KindMySQL:
		if cfg.Port == 0 {
			cfg.Port = 3306
		}
		return "mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/",
			cfg.AdminUser, cfg.AdminPassword, cfg.Host, cfg.Port), nil
	default:
		return "", "", fmt.Errorf("not a database connector kind %q", kind)
	}
}

// ParseCaddyConfig unmarshals connector JSON.
func ParseCaddyConfig(raw string) (CaddyConfig, error) {
	var c CaddyConfig
	if raw == "" {
		raw = "{}"
	}
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return CaddyConfig{}, err
	}
	if c.ConfigPathContainer == "" {
		c.ConfigPathContainer = "/etc/caddy/Caddyfile"
	}
	return c, nil
}

// ParseDockerConfig unmarshals docker connector JSON.
func ParseDockerConfig(raw string) (DockerConfig, error) {
	var c DockerConfig
	if raw == "" {
		raw = "{}"
	}
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return DockerConfig{}, err
	}
	return c, nil
}

// ParseDatabaseConfig unmarshals database connector JSON.
func ParseDatabaseConfig(raw string) (DatabaseConfig, error) {
	var c DatabaseConfig
	if raw == "" {
		raw = "{}"
	}
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return DatabaseConfig{}, err
	}
	return c, nil
}

// Ping checks connectivity for a connector and updates store health.
func (r *Registry) Ping(ctx context.Context, c store.ServiceConnector) error {
	var err error
	switch Kind(c.Kind) {
	case KindCaddy:
		client, e := r.Caddy(c, LocalCaddyPaths(r.cfg))
		if e != nil {
			err = e
			break
		}
		_, err = client.Read(ctx)
	case KindDocker:
		svc, e := r.DockerService(c)
		if e != nil {
			err = e
			break
		}
		err = svc.Ping(ctx)
	case KindPostgres, KindMySQL:
		cfg, e := ParseDatabaseConfig(c.ConfigJSON)
		if e != nil {
			err = e
			break
		}
		driver, dsn, e := DatabaseDSN(Kind(c.Kind), cfg)
		if e != nil {
			err = e
			break
		}
		err = pingDB(ctx, driver, dsn)
	default:
		err = fmt.Errorf("unsupported kind %q", c.Kind)
	}
	if err != nil {
		_ = r.store.TouchServiceConnector(c.ID, false, err.Error())
		return err
	}
	_ = r.store.TouchServiceConnector(c.ID, true, "")
	return nil
}

// EffectiveCaddy returns the default caddy connector or a synthetic local one.
func (r *Registry) EffectiveCaddy() (store.ServiceConnector, CaddyLocalPaths, error) {
	paths := LocalCaddyPaths(r.cfg)
	c, err := r.Default(KindCaddy)
	if err == nil {
		return c, paths, nil
	}
	if err != store.ErrNotFound {
		return store.ServiceConnector{}, paths, err
	}
	return store.ServiceConnector{
		Name:  "local",
		Kind:  string(KindCaddy),
		Mode:  string(ModeLocal),
		Enabled: true,
	}, paths, nil
}
