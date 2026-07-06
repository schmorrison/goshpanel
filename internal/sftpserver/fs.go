package sftpserver

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
	"github.com/schmorrison/goshpanel/internal/files"
)

type sandboxFS struct {
	svc *files.Service
}

func (f *sandboxFS) rel(path string) string {
	return strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(path)), "/")
}

func (f *sandboxFS) Fileread(r *sftp.Request) (io.ReaderAt, error) {
	file, info, err := f.svc.Open(f.rel(r.Filepath))
	if err != nil {
		return nil, err
	}
	return &fileReaderAt{file: file, size: info.Size()}, nil
}

func (f *sandboxFS) Filewrite(r *sftp.Request) (io.WriterAt, error) {
	rel := f.rel(r.Filepath)
	if err := f.svc.Write(rel, []byte{}); err != nil {
		return nil, err
	}
	file, _, err := f.svc.Open(rel)
	if err != nil {
		return nil, err
	}
	return &fileWriterAt{file: file}, nil
}

func (f *sandboxFS) Filecmd(r *sftp.Request) error {
	switch r.Method {
	case "Setstat":
		return nil
	case "Rename":
		return f.svc.Rename(f.rel(r.Filepath), f.rel(r.Target))
	case "Rmdir":
		return f.svc.Delete(f.rel(r.Filepath))
	case "Mkdir":
		return f.svc.Mkdir(f.rel(r.Filepath))
	case "Remove":
		return f.svc.Delete(f.rel(r.Filepath))
	default:
		return sftp.ErrSSHFxOpUnsupported
	}
}

func (f *sandboxFS) Filelist(r *sftp.Request) (sftp.ListerAt, error) {
	switch r.Method {
	case "List":
		entries, err := f.svc.List(f.rel(r.Filepath))
		if err != nil {
			return nil, err
		}
		stats := make([]os.FileInfo, 0, len(entries))
		for _, e := range entries {
			abs := filepath.Join(f.svc.Root(), filepath.FromSlash(e.Path))
			info, err := os.Stat(abs)
			if err != nil {
				continue
			}
			stats = append(stats, info)
		}
		return listerAt(stats), nil
	case "Stat", "Lstat":
		rel := f.rel(r.Filepath)
		abs := filepath.Join(f.svc.Root(), filepath.FromSlash(rel))
		if rel == "" || rel == "." {
			abs = f.svc.Root()
		}
		info, err := os.Stat(abs)
		if err != nil {
			return nil, err
		}
		return listerAt([]os.FileInfo{info}), nil
	default:
		return nil, sftp.ErrSSHFxOpUnsupported
	}
}

type listerAt []os.FileInfo

func (l listerAt) ListAt(dest []os.FileInfo, offset int64) (int, error) {
	if offset >= int64(len(l)) {
		return 0, io.EOF
	}
	n := copy(dest, l[offset:])
	if int(offset)+n >= len(l) {
		return n, io.EOF
	}
	return n, nil
}

type fileReaderAt struct {
	file *os.File
	size int64
}

func (f *fileReaderAt) ReadAt(p []byte, off int64) (int, error) {
	return f.file.ReadAt(p, off)
}

type fileWriterAt struct {
	file *os.File
}

func (f *fileWriterAt) WriteAt(p []byte, off int64) (int, error) {
	return f.file.WriteAt(p, off)
}
