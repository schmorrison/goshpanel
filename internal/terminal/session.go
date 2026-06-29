package terminal

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/creack/pty"
)

// Session runs a local shell inside a pseudo-terminal.
type Session struct {
	cmd *exec.Cmd
	pty *os.File
}

// NewSession starts a shell in workdir.
func NewSession(shell, workdir string) (*Session, error) {
	if shell == "" {
		shell = "/bin/bash"
	}

	absWorkdir, err := filepath.Abs(workdir)
	if err != nil {
		return nil, fmt.Errorf("resolve workdir: %w", err)
	}
	if err := os.MkdirAll(absWorkdir, 0o755); err != nil {
		return nil, fmt.Errorf("create workdir: %w", err)
	}

	cmd := exec.Command(shell, "-l")
	cmd.Dir = absWorkdir
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"HOME="+absWorkdir,
	)

	ptyFile, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("start pty: %w", err)
	}

	_ = pty.Setsize(ptyFile, &pty.Winsize{Rows: 30, Cols: 100})

	return &Session{cmd: cmd, pty: ptyFile}, nil
}

// Read writes terminal output into writer.
func (s *Session) Read(p []byte) (int, error) {
	return s.pty.Read(p)
}

// Write sends input to the terminal.
func (s *Session) Write(p []byte) (int, error) {
	return s.pty.Write(p)
}

// Resize updates terminal dimensions.
func (s *Session) Resize(cols, rows uint16) error {
	return pty.Setsize(s.pty, &pty.Winsize{Rows: rows, Cols: cols})
}

// Close stops the shell session.
func (s *Session) Close() error {
	_ = s.pty.Close()
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	_, err := s.cmd.Process.Wait()
	if err == io.EOF {
		return nil
	}
	return err
}
