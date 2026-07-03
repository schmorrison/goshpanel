// Package docker wraps the docker CLI for container and compose management.
// All operations are thin process calls — no cgo, no embedded Docker daemon.
package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ErrUnavailable is returned when the docker binary is not found.
var ErrUnavailable = errors.New("docker binary not found in PATH")

// Container is a parsed docker ps entry.
type Container struct {
	ID      string `json:"ID"`
	Names   string `json:"Names"`
	Image   string `json:"Image"`
	Status  string `json:"Status"`
	Ports   string `json:"Ports"`
	Created string `json:"CreatedAt"`
}

// Image is a parsed docker image entry.
type Image struct {
	ID       string `json:"ID"`
	RepoTags string `json:"Repository"`
	Tag      string `json:"Tag"`
	Size     string `json:"Size"`
	Created  string `json:"CreatedAt"`
}

// Service talks to Docker via the CLI.
type Service struct {
	bin string
}

// New creates a Docker service. Returns ErrUnavailable if docker is missing.
func New() (*Service, error) {
	bin, err := exec.LookPath("docker")
	if err != nil {
		return nil, ErrUnavailable
	}
	return &Service{bin: bin}, nil
}

// Available reports whether docker CLI exists.
func Available() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}

// Ping verifies the daemon responds.
func (s *Service) Ping(ctx context.Context) error {
	out, err := exec.CommandContext(ctx, s.bin, "info", "--format", "{{.ServerVersion}}").CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker daemon: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// Containers lists all containers.
func (s *Service) Containers(ctx context.Context) ([]Container, error) {
	return listJSON[Container](ctx, s.bin, "ps", "-a",
		"--format", `{{json .}}`,
		"--no-trunc=false")
}

// Images lists local images.
func (s *Service) Images(ctx context.Context) ([]Image, error) {
	return listJSON[Image](ctx, s.bin, "images",
		"--format", `{"ID":"{{.ID}}","Repository":"{{.Repository}}","Tag":"{{.Tag}}","Size":"{{.Size}}","CreatedAt":"{{.CreatedAt}}"}`)
}

// Start starts a container by ID or name.
func (s *Service) Start(ctx context.Context, id string) error {
	return s.run(ctx, "start", id)
}

// Stop stops a container.
func (s *Service) Stop(ctx context.Context, id string) error {
	return s.run(ctx, "stop", id)
}

// Restart restarts a container.
func (s *Service) Restart(ctx context.Context, id string) error {
	return s.run(ctx, "restart", id)
}

// Remove removes a container (-f).
func (s *Service) Remove(ctx context.Context, id string) error {
	return s.run(ctx, "rm", "-f", id)
}

// Logs returns the last n lines of container logs.
func (s *Service) Logs(ctx context.Context, id string, tail int) (string, error) {
	out, err := exec.CommandContext(ctx, s.bin, "logs", "--tail", fmt.Sprintf("%d", tail), id).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker logs: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return string(out), nil
}

// Run starts a detached container and returns its ID.
func (s *Service) Run(ctx context.Context, image, name string, ports, env []string) (string, error) {
	args := []string{"run", "-d", "--name", name}
	for _, p := range ports {
		if p = strings.TrimSpace(p); p != "" {
			args = append(args, "-p", p)
		}
	}
	for _, e := range env {
		if e = strings.TrimSpace(e); e != "" {
			args = append(args, "-e", e)
		}
	}
	args = append(args, image)
	out, err := exec.CommandContext(ctx, s.bin, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker run: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ComposeUp runs docker compose up -d in workdir with the given compose file content.
func (s *Service) ComposeUp(ctx context.Context, workdir, composeYAML string) (string, error) {
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return "", err
	}
	composePath := filepath.Join(workdir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(composeYAML), 0o644); err != nil {
		return "", err
	}
	out, err := exec.CommandContext(ctx, s.bin, "compose", "-f", composePath, "up", "-d").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker compose up: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ComposeDown runs docker compose down in workdir.
func (s *Service) ComposeDown(ctx context.Context, workdir string) (string, error) {
	composePath := filepath.Join(workdir, "docker-compose.yml")
	out, err := exec.CommandContext(ctx, s.bin, "compose", "-f", composePath, "down").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker compose down: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ComposePS lists compose services in a stack workdir.
func (s *Service) ComposePS(ctx context.Context, workdir string) (string, error) {
	composePath := filepath.Join(workdir, "docker-compose.yml")
	out, err := exec.CommandContext(ctx, s.bin, "compose", "-f", composePath, "ps").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker compose ps: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return string(out), nil
}

func (s *Service) run(ctx context.Context, args ...string) error {
	out, err := exec.CommandContext(ctx, s.bin, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker %s: %s: %w", args[0], strings.TrimSpace(string(out)), err)
	}
	return nil
}

func listJSON[T any](ctx context.Context, bin string, args ...string) ([]T, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("docker: %s: %w", strings.TrimSpace(string(exitErr.Stderr)), err)
		}
		return nil, err
	}
	var items []T
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var item T
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("parse docker json: %w", err)
		}
		items = append(items, item)
	}
	return items, sc.Err()
}

// StackWorkdir returns the on-disk directory for a named compose stack.
func StackWorkdir(dataDir, stackName string) string {
	return filepath.Join(dataDir, "docker-stacks", stackName)
}

// ShortID returns the first 12 chars of a container/image ID for display.
func ShortID(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// ParseDuration is a helper for tests.
func ParseDuration(s string) time.Duration {
	d, _ := time.ParseDuration(s)
	return d
}
