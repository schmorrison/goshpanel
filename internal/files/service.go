package files

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/schmorrison/goshpanel/internal/store"
)

const (
	ActionList   = "files.list"
	ActionUpload = "files.upload"
	ActionDelete = "files.delete"
)

// ErrPathEscape is returned when a requested path leaves the sandbox root.
var ErrPathEscape = errors.New("path escapes sandbox root")

// Entry describes a file or directory inside the sandbox.
type Entry struct {
	Name    string
	Path    string
	IsDir   bool
	Size    int64
	Mode    string
	ModTime time.Time
}

// Service performs sandboxed file operations.
type Service struct {
	root  string
	audit *store.AuditRepository
}

// NewService creates a file service rooted at root.
func NewService(root string, audit *store.AuditRepository) (*Service, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve sandbox root: %w", err)
	}

	if err := os.MkdirAll(absRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create sandbox root: %w", err)
	}

	return &Service{
		root:  absRoot,
		audit: audit,
	}, nil
}

// Root returns the absolute sandbox root path.
func (s *Service) Root() string {
	return s.root
}

// List returns directory entries for relPath.
func (s *Service) List(ctx context.Context, userID int64, username, relPath string) ([]Entry, error) {
	absPath, displayPath, err := s.resolve(relPath)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("stat path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory")
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	result := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		entryInfo, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("stat entry: %w", err)
		}

		childPath := filepath.Join(displayPath, entry.Name())
		result = append(result, Entry{
			Name:    entry.Name(),
			Path:    childPath,
			IsDir:   entry.IsDir(),
			Size:    entryInfo.Size(),
			Mode:    entryInfo.Mode().String(),
			ModTime: entryInfo.ModTime(),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	if s.audit != nil {
		_ = s.audit.Record(ctx, userID, username, ActionList, displayPath)
	}

	return result, nil
}

// Delete removes a file or empty directory inside the sandbox.
func (s *Service) Delete(ctx context.Context, userID int64, username, relPath string) error {
	absPath, displayPath, err := s.resolve(relPath)
	if err != nil {
		return err
	}

	if absPath == s.root {
		return fmt.Errorf("cannot delete sandbox root")
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("stat path: %w", err)
	}
	if info.IsDir() {
		entries, err := os.ReadDir(absPath)
		if err != nil {
			return fmt.Errorf("read directory: %w", err)
		}
		if len(entries) > 0 {
			return fmt.Errorf("directory is not empty")
		}
	}

	if err := os.Remove(absPath); err != nil {
		return fmt.Errorf("delete path: %w", err)
	}

	if s.audit != nil {
		_ = s.audit.Record(ctx, userID, username, ActionDelete, displayPath)
	}

	return nil
}

// Upload writes a file into relDir using the provided reader.
func (s *Service) Upload(ctx context.Context, userID int64, username, relDir, filename string, r io.Reader) error {
	filename = filepath.Base(filename)
	if filename == "" || filename == "." || filename == string(filepath.Separator) {
		return fmt.Errorf("invalid filename")
	}

	absDir, displayDir, err := s.resolve(relDir)
	if err != nil {
		return err
	}

	info, err := os.Stat(absDir)
	if err != nil {
		return fmt.Errorf("stat directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("upload target is not a directory")
	}

	target := filepath.Join(absDir, filename)
	if !withinRoot(s.root, target) {
		return ErrPathEscape
	}

	file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, r); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	displayPath := filepath.ToSlash(filepath.Join(displayDir, filename))
	if s.audit != nil {
		_ = s.audit.Record(ctx, userID, username, ActionUpload, displayPath)
	}

	return nil
}

func (s *Service) resolve(relPath string) (absPath string, displayPath string, err error) {
	rel := strings.TrimPrefix(filepath.ToSlash(relPath), "/")
	if rel == "" || rel == "." {
		return s.root, "", nil
	}

	rel = filepath.Clean(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") || strings.Contains(rel, "/../") {
		return "", "", ErrPathEscape
	}

	absPath = filepath.Join(s.root, filepath.FromSlash(rel))
	absPath, err = filepath.Abs(absPath)
	if err != nil {
		return "", "", fmt.Errorf("resolve absolute path: %w", err)
	}
	if !withinRoot(s.root, absPath) {
		return "", "", ErrPathEscape
	}

	return absPath, filepath.ToSlash(rel), nil
}

func withinRoot(root, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)

	if target == root {
		return true
	}

	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
