package logging

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Source is an allowlisted log file.
type Source struct {
	Name string
	Path string
}

// Service tails allowlisted log files.
type Service struct {
	sources map[string]string
}

// NewService creates a logging service from configured sources.
func NewService(sources []Source) (*Service, error) {
	allowlist := make(map[string]string, len(sources))
	for _, source := range sources {
		name := strings.TrimSpace(source.Name)
		path := strings.TrimSpace(source.Path)
		if name == "" || path == "" {
			continue
		}

		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("resolve log path %q: %w", path, err)
		}
		allowlist[name] = abs
	}

	if len(allowlist) == 0 {
		abs, err := filepath.Abs("data/goshpanel.log")
		if err != nil {
			return nil, err
		}
		allowlist["panel"] = abs
	}

	return &Service{sources: allowlist}, nil
}

// Sources returns configured source names in stable order.
func (s *Service) Sources() []Source {
	out := make([]Source, 0, len(s.sources))
	names := make([]string, 0, len(s.sources))
	for name := range s.sources {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		out = append(out, Source{Name: name, Path: s.sources[name]})
	}
	return out
}

// Tail streams new lines from a source to writer, flushing after each line.
func (s *Service) Tail(ctx context.Context, sourceName string, w io.Writer, flush func()) error {
	path, ok := s.sources[sourceName]
	if !ok {
		return fmt.Errorf("unknown log source %q", sourceName)
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			if _, err := fmt.Fprintf(w, "data: waiting for log file %s\n\n", path); err != nil {
				return err
			}
			flush()
		} else {
			return fmt.Errorf("open log file: %w", err)
		}
	} else {
		defer file.Close()
		if err := s.writeExisting(file, w, flush); err != nil {
			return err
		}
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var offset int64
	if file != nil {
		offset, _ = file.Seek(0, io.SeekEnd)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			lines, newOffset, err := readFrom(path, offset)
			if err != nil {
				return err
			}
			offset = newOffset
			for _, line := range lines {
				if _, err := fmt.Fprintf(w, "data: %s\n\n", line); err != nil {
					return err
				}
				flush()
			}
		}
	}
}

func (s *Service) writeExisting(file *os.File, w io.Writer, flush func()) error {
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if _, err := fmt.Fprintf(w, "data: %s\n\n", scanner.Text()); err != nil {
			return err
		}
		flush()
	}
	return scanner.Err()
}

func readFrom(path string, offset int64) ([]string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, offset, nil
		}
		return nil, offset, err
	}
	defer file.Close()

	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, offset, err
	}

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, offset, err
	}

	newOffset, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, offset, err
	}
	return lines, newOffset, nil
}
