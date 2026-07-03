// Package files implements a sandboxed file manager: browse, read, write,
// upload, rename, delete, mkdir — always confined to a configured root.
package files

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ErrOutsideRoot is returned when a path escapes the sandbox.
var ErrOutsideRoot = errors.New("path escapes the file manager root")

// Entry describes one directory entry.
type Entry struct {
	Name    string
	Path    string // sandbox-relative, always with forward slashes
	IsDir   bool
	Size    int64
	Mode    os.FileMode
	ModTime time.Time
}

// Service is a file manager rooted at a sandbox directory.
type Service struct {
	root string
}

// New creates the service, creating root if needed.
func New(root string) (*Service, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve files root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("create files root: %w", err)
	}
	return &Service{root: abs}, nil
}

// Root returns the absolute sandbox root.
func (s *Service) Root() string { return s.root }

// resolve maps a sandbox-relative path to an absolute one, rejecting
// escapes via .. or absolute components.
func (s *Service) resolve(rel string) (string, error) {
	rel = strings.TrimPrefix(strings.TrimSpace(rel), "/")
	abs := filepath.Join(s.root, filepath.FromSlash(rel))
	abs = filepath.Clean(abs)
	if abs != s.root && !strings.HasPrefix(abs, s.root+string(os.PathSeparator)) {
		return "", ErrOutsideRoot
	}
	return abs, nil
}

// Rel converts an absolute path back to sandbox-relative form.
func (s *Service) rel(abs string) string {
	r, err := filepath.Rel(s.root, abs)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(r)
}

// List returns the entries of a directory, directories first.
func (s *Service) List(rel string) ([]Entry, error) {
	abs, err := s.resolve(rel)
	if err != nil {
		return nil, err
	}
	dirents, err := os.ReadDir(abs)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}
	entries := make([]Entry, 0, len(dirents))
	for _, de := range dirents {
		info, err := de.Info()
		if err != nil {
			continue
		}
		entries = append(entries, Entry{
			Name:    de.Name(),
			Path:    s.rel(filepath.Join(abs, de.Name())),
			IsDir:   de.IsDir(),
			Size:    info.Size(),
			Mode:    info.Mode(),
			ModTime: info.ModTime(),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

// Read returns the contents of a file, refusing anything above maxBytes.
func (s *Service) Read(rel string, maxBytes int64) ([]byte, error) {
	abs, err := s.resolve(rel)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, errors.New("is a directory")
	}
	if info.Size() > maxBytes {
		return nil, fmt.Errorf("file too large (%d bytes > %d limit)", info.Size(), maxBytes)
	}
	return os.ReadFile(abs)
}

// Write replaces the contents of a file, creating it if needed.
func (s *Service) Write(rel string, data []byte) error {
	abs, err := s.resolve(rel)
	if err != nil {
		return err
	}
	return os.WriteFile(abs, data, 0o644)
}

// Save streams r into a new file at dir/name (upload).
func (s *Service) Save(dirRel, name string, r io.Reader) error {
	if strings.ContainsAny(name, "/\\") || name == "" || name == "." || name == ".." {
		return fmt.Errorf("invalid file name %q", name)
	}
	abs, err := s.resolve(filepath.ToSlash(filepath.Join(dirRel, name)))
	if err != nil {
		return err
	}
	f, err := os.Create(abs)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("write upload: %w", err)
	}
	return nil
}

// Mkdir creates a directory (and parents).
func (s *Service) Mkdir(rel string) error {
	abs, err := s.resolve(rel)
	if err != nil {
		return err
	}
	return os.MkdirAll(abs, 0o755)
}

// Rename moves a file or directory within the sandbox.
func (s *Service) Rename(fromRel, toRel string) error {
	from, err := s.resolve(fromRel)
	if err != nil {
		return err
	}
	to, err := s.resolve(toRel)
	if err != nil {
		return err
	}
	return os.Rename(from, to)
}

// Delete removes a file or directory tree. The root itself is protected.
func (s *Service) Delete(rel string) error {
	abs, err := s.resolve(rel)
	if err != nil {
		return err
	}
	if abs == s.root {
		return errors.New("refusing to delete the sandbox root")
	}
	return os.RemoveAll(abs)
}

// Open returns a reader for downloads. Caller must close it.
func (s *Service) Open(rel string) (*os.File, os.FileInfo, error) {
	abs, err := s.resolve(rel)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	if info.IsDir() {
		f.Close()
		return nil, nil, errors.New("is a directory")
	}
	return f, info, nil
}
