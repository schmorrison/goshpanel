package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/schmorrison/goshpanel/internal/email"
	"github.com/schmorrison/goshpanel/internal/store"
)

// MaddyConfig is stored in ServiceConnector.ConfigJSON for kind=maddy.
type MaddyConfig struct {
	Container           string `json:"container"`
	ConfigPathContainer string `json:"config_path_container"`
	ConfigPathHost      string `json:"config_path_host"`
	DockerHost          string `json:"docker_host"`
}

// MaddyLocalPaths supplies on-disk paths for local / fallback mode.
type MaddyLocalPaths struct {
	ConfigPath string
}

// MaddyClient reads/writes maddy.conf and reloads maddy (host or container).
type MaddyClient struct {
	conn   store.ServiceConnector
	cfg    MaddyConfig
	local  MaddyLocalPaths
	docker *dockerFactory
}

func newMaddyClient(c store.ServiceConnector, local MaddyLocalPaths, df *dockerFactory) (*MaddyClient, error) {
	cfg, err := ParseMaddyConfig(c.ConfigJSON)
	if err != nil {
		return nil, err
	}
	return &MaddyClient{conn: c, cfg: cfg, local: local, docker: df}, nil
}

// ParseMaddyConfig unmarshals connector JSON.
func ParseMaddyConfig(raw string) (MaddyConfig, error) {
	var c MaddyConfig
	if raw == "" {
		raw = "{}"
	}
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return MaddyConfig{}, err
	}
	if c.ConfigPathContainer == "" {
		c.ConfigPathContainer = "/etc/maddy/maddy.conf"
	}
	return c, nil
}

// ConfigPath returns the path used for display/status.
func (m *MaddyClient) ConfigPath() string {
	if m.conn.Mode == string(ModeDocker) && m.cfg.ConfigPathHost != "" {
		return m.cfg.ConfigPathHost
	}
	if m.conn.Mode == string(ModeDocker) && m.cfg.Container != "" {
		return m.cfg.Container + ":" + m.cfg.ConfigPathContainer
	}
	return m.local.ConfigPath
}

// Read returns the current maddy.conf contents.
func (m *MaddyClient) Read(ctx context.Context) (string, error) {
	return readSingleFile(ctx, m.conn, m.cfg.Container, m.cfg.ConfigPathHost, m.cfg.ConfigPathContainer, m.local.ConfigPath, m.docker)
}

// Write saves maddy.conf content.
func (m *MaddyClient) Write(ctx context.Context, content string) error {
	return writeSingleFile(ctx, m.conn, m.cfg.Container, m.cfg.ConfigPathHost, m.cfg.ConfigPathContainer, m.local.ConfigPath, content, m.docker)
}

// Reload signals maddy to pick up the config.
func (m *MaddyClient) Reload(ctx context.Context) error {
	configPath := m.local.ConfigPath
	if m.conn.Mode == string(ModeDocker) && m.cfg.Container != "" {
		cli, err := m.docker.For(m.conn)
		if err != nil {
			return err
		}
		path := m.cfg.ConfigPathContainer
		if m.cfg.ConfigPathHost != "" {
			path = m.cfg.ConfigPathHost
		}
		out, err := cli.Exec(ctx, m.cfg.Container, "maddy", "reload", "-config", path)
		if err != nil {
			return fmt.Errorf("maddy reload in container: %s: %w", strings.TrimSpace(out), err)
		}
		return nil
	}
	return reloadBinary(ctx, "maddy", []string{"reload", "-config", configPath})
}

// WriteAndReload writes content then reloads.
func (m *MaddyClient) WriteAndReload(ctx context.Context, content string) error {
	if err := m.Write(ctx, content); err != nil {
		return err
	}
	return m.Reload(ctx)
}

// Logs tails container logs.
func (m *MaddyClient) Logs(ctx context.Context, tail int) (string, error) {
	if m.conn.Mode == string(ModeDocker) && m.cfg.Container != "" {
		cli, err := m.docker.For(m.conn)
		if err != nil {
			return "", err
		}
		return cli.Logs(ctx, m.cfg.Container, tail)
	}
	return "", fmt.Errorf("no log source configured")
}

// RenderMaddyFromStore builds maddy.conf from panel email state.
func RenderMaddyFromStore(st *store.Store, hostname string) (string, error) {
	boxes, err := st.Mailboxes()
	if err != nil {
		return "", err
	}
	fwds, err := st.Forwarders()
	if err != nil {
		return "", err
	}
	if hostname == "" {
		hostname = "localhost"
	}
	return email.RenderMaddyConfig(boxes, fwds, hostname), nil
}

// dockerRestart reloads a container by restart (fallback).
func dockerRestart(ctx context.Context, cli *DockerCLI, container string) error {
	out, err := cli.cmd(ctx, "restart", container).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker restart %s: %s: %w", container, strings.TrimSpace(string(out)), err)
	}
	return nil
}

func reloadBinary(ctx context.Context, bin string, args []string) error {
	path, err := exec.LookPath(bin)
	if err != nil {
		return fmt.Errorf("%s binary not found in PATH", bin)
	}
	out, err := exec.CommandContext(ctx, path, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s: %w", bin, strings.TrimSpace(string(out)), err)
	}
	return nil
}
