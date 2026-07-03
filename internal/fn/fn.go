// Package fn implements HTTP-triggered micro functions: small bash scripts
// stored in SQLite, materialized on disk, and invoked via GET/POST /fn/{name}.
package fn

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/schmorrison/goshpanel/internal/crypto"
	"github.com/schmorrison/goshpanel/internal/store"
)

var nameRe = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

// ErrInvalidName is returned for bad function names.
var ErrInvalidName = errors.New("function name must match [a-z][a-z0-9_-]{0,63}")

// ErrDisabled is returned when invoking a disabled function.
var ErrDisabled = errors.New("function is disabled")

// ErrUnauthorized is returned for a bad invoke token.
var ErrUnauthorized = errors.New("invalid or missing function token")

// InvokeResult holds stdout/stderr from one invocation.
type InvokeResult struct {
	Name     string
	Output   string
	ExitCode int
	Elapsed  time.Duration
	TimedOut bool
}

// Service manages micro functions.
type Service struct {
	store   *store.Store
	scripts string // directory where .sh files are written
}

// New creates a function service.
func New(st *store.Store, scriptsDir string) (*Service, error) {
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create functions dir: %w", err)
	}
	return &Service{store: st, scripts: scriptsDir}, nil
}

// ValidateName checks the function name slug.
func ValidateName(name string) error {
	if !nameRe.MatchString(name) {
		return fmt.Errorf("%w: got %q", ErrInvalidName, name)
	}
	return nil
}

// Create validates and stores a new function with an auto-generated token.
func (s *Service) Create(name, description, script string, timeoutSec int) (store.MicroFunction, error) {
	if err := ValidateName(name); err != nil {
		return store.MicroFunction{}, err
	}
	script = strings.TrimSpace(script)
	if script == "" {
		return store.MicroFunction{}, errors.New("script body is required")
	}
	token, err := crypto.RandomToken()
	if err != nil {
		return store.MicroFunction{}, err
	}
	f, err := s.store.CreateMicroFunction(name, description, script, token, timeoutSec)
	if err != nil {
		return store.MicroFunction{}, err
	}
	if err := s.materialize(f); err != nil {
		return store.MicroFunction{}, err
	}
	return f, nil
}

// Update saves changes and rewrites the script file.
func (s *Service) Update(id int64, description, script string, enabled bool, timeoutSec int) error {
	f, err := s.store.MicroFunctionByID(id)
	if err != nil {
		return err
	}
	if err := s.store.UpdateMicroFunction(id, description, script, enabled, timeoutSec); err != nil {
		return err
	}
	f.Description = description
	f.Script = script
	f.Enabled = enabled
	f.TimeoutSec = timeoutSec
	return s.materialize(f)
}

// Delete removes a function and its script file.
func (s *Service) Delete(id int64) error {
	f, err := s.store.MicroFunctionByID(id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteMicroFunction(id); err != nil {
		return err
	}
	_ = os.Remove(s.scriptPath(f.Name))
	return nil
}

// Invoke runs a function by name. token must match when the function has one set.
func (s *Service) Invoke(ctx context.Context, name, token string) (InvokeResult, error) {
	f, err := s.store.MicroFunctionByName(name)
	if err != nil {
		return InvokeResult{}, err
	}
	if !f.Enabled {
		return InvokeResult{}, ErrDisabled
	}
	if f.Token != "" && f.Token != token {
		return InvokeResult{}, ErrUnauthorized
	}
	if err := s.materialize(f); err != nil {
		return InvokeResult{}, err
	}

	timeout := time.Duration(f.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(runCtx, "bash", s.scriptPath(f.Name))
	cmd.Env = append(os.Environ(),
		"GOSHPANEL_FN_NAME="+f.Name,
		"GOSHPANEL_FN_ID="+fmt.Sprintf("%d", f.ID),
	)
	out, err := cmd.CombinedOutput()

	res := InvokeResult{Name: f.Name, Output: string(out), Elapsed: time.Since(start)}
	if runCtx.Err() == context.DeadlineExceeded {
		res.TimedOut = true
	}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		res.ExitCode = 0
	case errors.As(err, &exitErr):
		res.ExitCode = exitErr.ExitCode()
	default:
		return res, err
	}
	return res, nil
}

// InvokeURL returns the public HTTP URL for a function.
func InvokeURL(baseURL string, f store.MicroFunction) string {
	return strings.TrimSuffix(baseURL, "/") + "/fn/" + f.Name + "?token=" + f.Token
}

func (s *Service) materialize(f store.MicroFunction) error {
	path := s.scriptPath(f.Name)
	content := "#!/usr/bin/env bash\nset -euo pipefail\n" + f.Script + "\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		return fmt.Errorf("write script: %w", err)
	}
	return nil
}

func (s *Service) scriptPath(name string) string {
	return filepath.Join(s.scripts, name+".sh")
}
