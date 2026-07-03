// Package logs exposes tail-style reads over an allowlist of log files —
// the cPanel "Errors" / log viewer equivalent.
package logs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
)

// ErrNotAllowed is returned for paths outside the allowlist.
var ErrNotAllowed = errors.New("log file is not in the configured allowlist")

// Source describes one viewable log file.
type Source struct {
	Path   string
	Size   int64
	Exists bool
}

// Service reads from a fixed set of log files.
type Service struct {
	allowed []string
}

// New creates the service with an allowlist of absolute paths.
func New(allowed []string) *Service { return &Service{allowed: allowed} }

// Sources returns the allowlisted files with their current status.
func (s *Service) Sources() []Source {
	out := make([]Source, 0, len(s.allowed))
	for _, p := range s.allowed {
		src := Source{Path: p}
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			src.Exists = true
			src.Size = info.Size()
		}
		out = append(out, src)
	}
	return out
}

// Tail returns the last n lines of an allowlisted file.
func (s *Service) Tail(path string, n int) ([]string, error) {
	if !s.isAllowed(path) {
		return nil, ErrNotAllowed
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open log: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	// Read at most 1 MiB from the end; ample for any sensible n.
	const maxRead = 1 << 20
	readFrom := info.Size() - maxRead
	if readFrom < 0 {
		readFrom = 0
	}
	if _, err := f.Seek(readFrom, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	lines := bytes.Split(bytes.TrimRight(data, "\n"), []byte("\n"))
	if readFrom > 0 && len(lines) > 0 {
		lines = lines[1:] // first line is likely truncated
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = string(l)
	}
	return out, nil
}

func (s *Service) isAllowed(path string) bool {
	for _, p := range s.allowed {
		if p == path {
			return true
		}
	}
	return false
}
