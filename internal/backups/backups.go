// Package backups creates and restores tar.gz archives of the file
// manager sandbox using archive/tar and compress/gzip — pure Go, no
// external tar binary.
package backups

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Info describes one stored backup archive.
type Info struct {
	Name    string
	Size    int64
	Created time.Time
}

// Service manages archives of srcDir stored in backupDir.
type Service struct {
	srcDir    string
	backupDir string
}

// New creates the service, creating backupDir if needed.
func New(srcDir, backupDir string) (*Service, error) {
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return nil, fmt.Errorf("create backup dir: %w", err)
	}
	return &Service{srcDir: srcDir, backupDir: backupDir}, nil
}

// Create archives the source directory to a timestamped tar.gz and
// returns its name.
func (s *Service) Create() (string, error) {
	name := fmt.Sprintf("backup-%s.tar.gz", time.Now().UTC().Format("20060102-150405"))
	path := filepath.Join(s.backupDir, name)

	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create archive: %w", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	err = filepath.Walk(s.srcDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(s.srcDir, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			src, err := os.Open(p)
			if err != nil {
				return err
			}
			defer src.Close()
			if _, err := io.Copy(tw, src); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		tw.Close()
		gw.Close()
		os.Remove(path)
		return "", fmt.Errorf("archive walk: %w", err)
	}
	if err := tw.Close(); err != nil {
		return "", err
	}
	if err := gw.Close(); err != nil {
		return "", err
	}
	return name, nil
}

// List returns stored backups, newest first.
func (s *Service) List() ([]Info, error) {
	entries, err := os.ReadDir(s.backupDir)
	if err != nil {
		return nil, fmt.Errorf("read backup dir: %w", err)
	}
	var out []Info
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".tar.gz") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, Info{Name: e.Name(), Size: info.Size(), Created: info.ModTime()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out, nil
}

// Open returns a reader over a stored archive for download.
func (s *Service) Open(name string) (*os.File, os.FileInfo, error) {
	path, err := s.safePath(name)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	return f, info, nil
}

// Delete removes a stored archive.
func (s *Service) Delete(name string) error {
	path, err := s.safePath(name)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

// Restore extracts an archive back into the source directory,
// overwriting existing files. Paths are sanitized against traversal.
func (s *Service) Restore(name string) error {
	path, err := s.safePath(name)
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("read gzip: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read archive: %w", err)
		}
		target := filepath.Join(s.srcDir, filepath.FromSlash(hdr.Name))
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(s.srcDir)+string(os.PathSeparator)) {
			return fmt.Errorf("archive entry %q escapes destination", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, hdr.FileInfo().Mode()); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			dst, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, hdr.FileInfo().Mode())
			if err != nil {
				return err
			}
			if _, err := io.Copy(dst, tr); err != nil {
				dst.Close()
				return err
			}
			dst.Close()
		}
	}
}

// safePath validates an archive name and returns its full path.
func (s *Service) safePath(name string) (string, error) {
	if strings.ContainsAny(name, "/\\") || !strings.HasSuffix(name, ".tar.gz") {
		return "", fmt.Errorf("invalid backup name %q", name)
	}
	return filepath.Join(s.backupDir, name), nil
}
