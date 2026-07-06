package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/schmorrison/goshpanel/internal/store"
)

// CoreDNSConfig is stored in ServiceConnector.ConfigJSON for kind=coredns.
type CoreDNSConfig struct {
	Container             string `json:"container"`
	CorefilePathContainer string `json:"corefile_path_container"`
	CorefilePathHost      string `json:"corefile_path_host"`
	ZonesDirContainer     string `json:"zones_dir_container"`
	ZonesDirHost          string `json:"zones_dir_host"`
	DockerHost            string `json:"docker_host"`
}

// CoreDNSBundle is a generated Corefile plus zone files.
type CoreDNSBundle struct {
	Corefile string
	Zones    map[string]string // filename -> content
}

// CoreDNSLocalPaths supplies on-disk paths for local / fallback mode.
type CoreDNSLocalPaths struct {
	ConfigDir string
}

// CoreDNSClient reads/writes CoreDNS config (host or container).
type CoreDNSClient struct {
	conn   store.ServiceConnector
	cfg    CoreDNSConfig
	local  CoreDNSLocalPaths
	docker *dockerFactory
}

func newCoreDNSClient(c store.ServiceConnector, local CoreDNSLocalPaths, df *dockerFactory) (*CoreDNSClient, error) {
	cfg, err := ParseCoreDNSConfig(c.ConfigJSON)
	if err != nil {
		return nil, err
	}
	return &CoreDNSClient{conn: c, cfg: cfg, local: local, docker: df}, nil
}

// ParseCoreDNSConfig unmarshals connector JSON.
func ParseCoreDNSConfig(raw string) (CoreDNSConfig, error) {
	var c CoreDNSConfig
	if raw == "" {
		raw = "{}"
	}
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return CoreDNSConfig{}, err
	}
	if c.CorefilePathContainer == "" {
		c.CorefilePathContainer = "/etc/coredns/Corefile"
	}
	if c.ZonesDirContainer == "" {
		c.ZonesDirContainer = "/etc/coredns/zones"
	}
	return c, nil
}

// ConfigPath returns the display path for the Corefile.
func (c *CoreDNSClient) ConfigPath() string {
	if c.conn.Mode == string(ModeDocker) && c.cfg.CorefilePathHost != "" {
		return c.cfg.CorefilePathHost
	}
	if c.conn.Mode == string(ModeDocker) && c.cfg.Container != "" {
		return c.cfg.Container + ":" + c.cfg.CorefilePathContainer
	}
	return filepath.Join(c.local.ConfigDir, "Corefile")
}

// ReadCorefile returns the current Corefile contents.
func (c *CoreDNSClient) ReadCorefile(ctx context.Context) (string, error) {
	return readSingleFile(ctx, c.conn, c.cfg.Container, c.cfg.CorefilePathHost, c.cfg.CorefilePathContainer, filepath.Join(c.local.ConfigDir, "Corefile"), c.docker)
}

// WriteBundle writes Corefile and zone files, then reloads.
func (c *CoreDNSClient) WriteBundle(ctx context.Context, bundle CoreDNSBundle) error {
	if err := c.writeCorefile(ctx, bundle.Corefile); err != nil {
		return err
	}
	for name, content := range bundle.Zones {
		if err := c.writeZone(ctx, name, content); err != nil {
			return err
		}
	}
	return c.Reload(ctx)
}

func (c *CoreDNSClient) writeCorefile(ctx context.Context, content string) error {
	return writeSingleFile(ctx, c.conn, c.cfg.Container, c.cfg.CorefilePathHost, c.cfg.CorefilePathContainer, filepath.Join(c.local.ConfigDir, "Corefile"), content, c.docker)
}

func (c *CoreDNSClient) writeZone(ctx context.Context, filename, content string) error {
	if c.conn.Mode == string(ModeDocker) && c.cfg.Container != "" {
		if c.cfg.ZonesDirHost != "" {
			path := filepath.Join(c.cfg.ZonesDirHost, filename)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			return os.WriteFile(path, []byte(content), 0o644)
		}
		cli, err := c.docker.For(c.conn)
		if err != nil {
			return err
		}
		tmp := filepath.Join(os.TempDir(), "goshpanel-zone-"+filename)
		if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
			return err
		}
		defer os.Remove(tmp)
		dest := c.cfg.Container + ":" + filepath.Join(c.cfg.ZonesDirContainer, filename)
		return cli.Copy(ctx, tmp, dest)
	}
	path := filepath.Join(c.local.ConfigDir, "zones", filename)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// Reload signals CoreDNS to pick up config.
func (c *CoreDNSClient) Reload(ctx context.Context) error {
	if c.conn.Mode == string(ModeDocker) && c.cfg.Container != "" {
		cli, err := c.docker.For(c.conn)
		if err != nil {
			return err
		}
		if _, err := cli.cmd(ctx, "kill", "-s", "HUP", c.cfg.Container).CombinedOutput(); err != nil {
			return dockerRestart(ctx, cli, c.cfg.Container)
		}
		return nil
	}
	return reloadBinary(ctx, "coredns", []string{"-conf", filepath.Join(c.local.ConfigDir, "Corefile")})
}

// Logs tails container logs.
func (c *CoreDNSClient) Logs(ctx context.Context, tail int) (string, error) {
	if c.conn.Mode == string(ModeDocker) && c.cfg.Container != "" {
		cli, err := c.docker.For(c.conn)
		if err != nil {
			return "", err
		}
		return cli.Logs(ctx, c.cfg.Container, tail)
	}
	return "", fmt.Errorf("no log source configured")
}

// WriteCorefile saves Corefile only (no reload).
func (c *CoreDNSClient) WriteCorefile(ctx context.Context, content string) error {
	return c.writeCorefile(ctx, content)
}

// WriteCorefileAndReload writes Corefile from editor then reloads.
func (c *CoreDNSClient) WriteCorefileAndReload(ctx context.Context, content string) error {
	if err := c.WriteCorefile(ctx, content); err != nil {
		return err
	}
	return c.Reload(ctx)
}
