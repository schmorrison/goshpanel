package connector

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/schmorrison/goshpanel/internal/domains"
	"github.com/schmorrison/goshpanel/internal/store"
)

// CaddyClient reads/writes a Caddyfile and reloads Caddy (host or container).
type CaddyClient struct {
	conn   store.ServiceConnector
	cfg    CaddyConfig
	local  CaddyLocalPaths
	docker *dockerFactory
}

func newCaddyClient(c store.ServiceConnector, local CaddyLocalPaths, df *dockerFactory) (*CaddyClient, error) {
	cfg, err := ParseCaddyConfig(c.ConfigJSON)
	if err != nil {
		return nil, err
	}
	if cfg.AccessLogPath == "" {
		cfg.AccessLogPath = local.AccessLogPath
	}
	return &CaddyClient{conn: c, cfg: cfg, local: local, docker: df}, nil
}

// ConfigPath returns the path used for display/status.
func (c *CaddyClient) ConfigPath() string {
	if c.conn.Mode == string(ModeDocker) && c.cfg.ConfigPathHost != "" {
		return c.cfg.ConfigPathHost
	}
	if c.conn.Mode == string(ModeDocker) && c.cfg.Container != "" {
		return c.cfg.Container + ":" + c.cfg.ConfigPathContainer
	}
	return c.local.ConfigPath
}

// AccessLogPath returns the configured access log path.
func (c *CaddyClient) AccessLogPath() string {
	return c.cfg.AccessLogPath
}

// Read returns the current Caddyfile contents.
func (c *CaddyClient) Read(ctx context.Context) (string, error) {
	if c.conn.Mode == string(ModeDocker) && c.cfg.Container != "" {
		if c.cfg.ConfigPathHost != "" {
			b, err := os.ReadFile(c.cfg.ConfigPathHost)
			if err != nil {
				return "", err
			}
			return string(b), nil
		}
		cli, err := c.docker.For(c.conn)
		if err != nil {
			return "", err
		}
		out, err := cli.Exec(ctx, c.cfg.Container, "cat", c.cfg.ConfigPathContainer)
		if err != nil {
			return "", err
		}
		return out, nil
	}
	b, err := os.ReadFile(c.local.ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}

// Write saves Caddyfile content to the configured target.
func (c *CaddyClient) Write(ctx context.Context, content string) error {
	if c.conn.Mode == string(ModeDocker) && c.cfg.Container != "" {
		if c.cfg.ConfigPathHost != "" {
			if err := os.MkdirAll(filepath.Dir(c.cfg.ConfigPathHost), 0o755); err != nil {
				return err
			}
			return os.WriteFile(c.cfg.ConfigPathHost, []byte(content), 0o644)
		}
		cli, err := c.docker.For(c.conn)
		if err != nil {
			return err
		}
		tmp := filepath.Join(os.TempDir(), "goshpanel-caddyfile")
		if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
			return err
		}
		defer os.Remove(tmp)
		dest := c.cfg.Container + ":" + c.cfg.ConfigPathContainer
		return cli.Copy(ctx, tmp, dest)
	}
	if err := os.MkdirAll(filepath.Dir(c.local.ConfigPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(c.local.ConfigPath, []byte(content), 0o644)
}

// Reload signals Caddy to pick up the config.
func (c *CaddyClient) Reload(ctx context.Context) error {
	if c.cfg.AdminURL != "" {
		content, err := c.Read(ctx)
		if err != nil {
			return err
		}
		return c.reloadAdminAPI(ctx, content)
	}
	configPath := c.local.ConfigPath
	if c.conn.Mode == string(ModeDocker) && c.cfg.Container != "" {
		cli, err := c.docker.For(c.conn)
		if err != nil {
			return err
		}
		path := c.cfg.ConfigPathContainer
		if c.cfg.ConfigPathHost != "" {
			path = c.cfg.ConfigPathHost
		}
		out, err := cli.Exec(ctx, c.cfg.Container, "caddy", "reload", "--config", path, "--adapter", "caddyfile")
		if err != nil {
			return fmt.Errorf("caddy reload in container: %s: %w", strings.TrimSpace(out), err)
		}
		return nil
	}
	return domains.ApplyCaddyfile(ctx, configPath)
}

func (c *CaddyClient) reloadAdminAPI(ctx context.Context, content string) error {
	url := strings.TrimRight(c.cfg.AdminURL, "/") + "/load"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(content))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/caddyfile")
	if c.cfg.AdminToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.AdminToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("caddy admin API: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("caddy admin API %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// WriteAndReload writes content then reloads.
func (c *CaddyClient) WriteAndReload(ctx context.Context, content string) error {
	if err := c.Write(ctx, content); err != nil {
		return err
	}
	return c.Reload(ctx)
}

// Logs tails Caddy container logs or reads the access log file.
func (c *CaddyClient) Logs(ctx context.Context, tail int) (string, error) {
	if tail <= 0 {
		tail = 200
	}
	if c.conn.Mode == string(ModeDocker) && c.cfg.Container != "" {
		cli, err := c.docker.For(c.conn)
		if err != nil {
			return "", err
		}
		return cli.Logs(ctx, c.cfg.Container, tail)
	}
	if c.cfg.AccessLogPath != "" {
		return tailFile(c.cfg.AccessLogPath, tail)
	}
	return "", fmt.Errorf("no log source configured")
}

func tailFile(path string, n int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer f.Close()
	// simple read whole file for small logs; orchestrator uses json rolling
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n"), nil
}

// WriteRendered renders panel domain state and applies via this client.
func (c *CaddyClient) WriteRendered(ctx context.Context, caddyCtx domains.CaddyContext) error {
	content := domains.RenderCaddyfileFull(caddyCtx)
	return c.WriteAndReload(ctx, content)
}

// dockerFactory builds docker CLI helpers.
type dockerFactory struct{}

// DockerCLI wraps docker CLI with optional DOCKER_HOST.
type DockerCLI struct {
	bin        string
	dockerHost string
}

func (f *dockerFactory) For(c store.ServiceConnector) (*DockerCLI, error) {
	bin, err := exec.LookPath("docker")
	if err != nil {
		return nil, fmt.Errorf("docker binary not found in PATH")
	}
	host := ""
	if c.Kind == string(KindDocker) {
		cfg, err := ParseDockerConfig(c.ConfigJSON)
		if err != nil {
			return nil, err
		}
		host = cfg.DockerHost
	} else if c.Kind == string(KindCaddy) {
		cfg, err := ParseCaddyConfig(c.ConfigJSON)
		if err != nil {
			return nil, err
		}
		host = cfg.DockerHost
	}
	return &DockerCLI{bin: bin, dockerHost: host}, nil
}

func (d *DockerCLI) cmd(ctx context.Context, args ...string) *exec.Cmd {
	c := exec.CommandContext(ctx, d.bin, args...)
	if d.dockerHost != "" {
		c.Env = append(os.Environ(), "DOCKER_HOST="+d.dockerHost)
	}
	return c
}

// Ping checks docker daemon connectivity.
func (d *DockerCLI) Ping(ctx context.Context) error {
	out, err := d.cmd(ctx, "info", "--format", "{{.ServerVersion}}").CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker daemon: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// Exec runs a command inside a container.
func (d *DockerCLI) Exec(ctx context.Context, container string, args ...string) (string, error) {
	full := append([]string{"exec", container}, args...)
	out, err := d.cmd(ctx, full...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("docker exec: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return string(out), nil
}

// Copy copies a host file into container:path.
func (d *DockerCLI) Copy(ctx context.Context, src, dest string) error {
	out, err := d.cmd(ctx, "cp", src, dest).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker cp: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// Logs returns container log tail.
func (d *DockerCLI) Logs(ctx context.Context, container string, tail int) (string, error) {
	out, err := d.cmd(ctx, "logs", "--tail", fmt.Sprintf("%d", tail), container).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker logs: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return string(out), nil
}

// Containers lists running containers (name + image).
func (d *DockerCLI) Containers(ctx context.Context) (string, error) {
	out, err := d.cmd(ctx, "ps", "--format", "table {{.Names}}\t{{.Image}}\t{{.Status}}").CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
