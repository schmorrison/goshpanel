// Package runner executes one-shot shell commands with a timeout — the
// cPanel "Terminal" analogue, implemented as a request/response command
// runner rather than a persistent PTY.
package runner

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

// Result holds the outcome of one command execution.
type Result struct {
	Command  string
	Output   string
	ExitCode int
	Elapsed  time.Duration
	TimedOut bool
}

// Service runs commands under bash in a fixed working directory.
type Service struct {
	workdir string
	timeout time.Duration
	enabled bool
}

// New creates a runner. When enabled is false, Run always errors.
func New(workdir string, timeout time.Duration, enabled bool) *Service {
	return &Service{workdir: workdir, timeout: timeout, enabled: enabled}
}

// Enabled reports whether the module is switched on.
func (s *Service) Enabled() bool { return s.enabled }

// Run executes command via bash -c, capturing combined output.
func (s *Service) Run(ctx context.Context, command string) (Result, error) {
	if !s.enabled {
		return Result{}, errors.New("command runner is disabled (set GOSHPANEL_COMMAND_RUNNER=true)")
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return Result{}, errors.New("empty command")
	}

	runCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(runCtx, "bash", "-c", command)
	cmd.Dir = s.workdir
	out, err := cmd.CombinedOutput()

	res := Result{
		Command: command,
		Output:  string(out),
		Elapsed: time.Since(start),
	}
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
