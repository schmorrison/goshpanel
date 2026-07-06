package web

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/schmorrison/goshpanel/internal/files"
)

const maxEditableBytes = 2 << 20 // 2 MiB in-browser editor limit

type filesData struct {
	Dir     string
	Parent  string
	Entries []files.Entry
}

type fileViewData struct {
	Path    string
	Dir     string
	Content string
}

// filesURL builds /files?dir=... preserving the current directory.
func filesURL(dir string) string {
	if dir == "" || dir == "." {
		return "/files"
	}
	return "/files?dir=" + url.QueryEscape(dir)
}

func (s *Server) handleFilesPage(w http.ResponseWriter, r *http.Request) {
	dir := strings.Trim(r.URL.Query().Get("dir"), "/")
	entries, err := s.files.List(dir)
	if err != nil {
		redirectError(w, r, "/files", err)
		return
	}
	parent := ""
	if dir != "" {
		parent = path.Dir(dir)
		if parent == "." {
			parent = ""
		}
	}
	s.render(w, r, "files.html", "File Manager", "files", filesData{Dir: dir, Parent: parent, Entries: entries})
}

func (s *Server) handleFileView(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	data, err := s.files.Read(p, maxEditableBytes)
	if err != nil {
		redirectError(w, r, filesURL(path.Dir(p)), err)
		return
	}
	s.render(w, r, "file_edit.html", "Edit "+p, "files", fileViewData{
		Path: p, Dir: path.Dir(p), Content: string(data),
	})
}

func (s *Server) handleFileDownload(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	f, info, err := s.files.Open(p)
	if err != nil {
		redirectError(w, r, "/files", err)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", info.Name()))
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

func (s *Server) handleFileSave(w http.ResponseWriter, r *http.Request) {
	p := r.FormValue("path")
	if err := s.files.Write(p, []byte(r.FormValue("content"))); err != nil {
		redirectError(w, r, filesURL(path.Dir(p)), err)
		return
	}
	s.audit(r, "files.save", p)
	redirectFlash(w, r, filesURL(path.Dir(p)), "Saved "+p)
}

func (s *Server) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	dir := r.FormValue("dir")
	file, header, err := r.FormFile("file")
	if err != nil {
		redirectError(w, r, filesURL(dir), fmt.Errorf("no file uploaded"))
		return
	}
	defer file.Close()
	if err := s.files.Save(dir, header.Filename, file); err != nil {
		redirectError(w, r, filesURL(dir), err)
		return
	}
	s.audit(r, "files.upload", path.Join(dir, header.Filename))
	redirectFlash(w, r, filesURL(dir), "Uploaded "+header.Filename)
}

func (s *Server) handleFileMkdir(w http.ResponseWriter, r *http.Request) {
	dir := r.FormValue("dir")
	name := r.FormValue("name")
	target := path.Join(dir, name)
	if err := s.files.Mkdir(target); err != nil {
		redirectError(w, r, filesURL(dir), err)
		return
	}
	s.audit(r, "files.mkdir", target)
	redirectFlash(w, r, filesURL(dir), "Created folder "+name)
}

func (s *Server) handleFileRename(w http.ResponseWriter, r *http.Request) {
	dir := r.FormValue("dir")
	from := r.FormValue("from")
	to := path.Join(dir, r.FormValue("to"))
	if err := s.files.Rename(from, to); err != nil {
		redirectError(w, r, filesURL(dir), err)
		return
	}
	s.audit(r, "files.rename", from+" -> "+to)
	redirectFlash(w, r, filesURL(dir), "Renamed to "+r.FormValue("to"))
}

func (s *Server) handleFileDelete(w http.ResponseWriter, r *http.Request) {
	dir := r.FormValue("dir")
	p := r.FormValue("path")
	if err := s.files.Delete(p); err != nil {
		redirectError(w, r, filesURL(dir), err)
		return
	}
	s.audit(r, "files.delete", p)
	redirectFlash(w, r, filesURL(dir), "Deleted "+p)
}
