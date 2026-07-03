package web

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/schmorrison/goshpanel/internal/store"
)

// pageData is the payload every template receives.
type pageData struct {
	Title  string
	Active string // nav highlight key
	User   store.User
	CSRF   string
	Flash  string
	Error  string
	Data   any
}

// render executes the named template with a fully populated pageData.
func (s *Server) render(w http.ResponseWriter, r *http.Request, name, title, active string, data any) {
	pd := pageData{
		Title:  title,
		Active: active,
		User:   currentUser(r),
		CSRF:   currentSession(r).CSRFToken,
		Flash:  r.URL.Query().Get("flash"),
		Error:  r.URL.Query().Get("error"),
		Data:   data,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, pd); err != nil {
		s.log.Error("render failed", "template", name, "err", err)
	}
}

// redirectFlash redirects with a flash (or error) message in the query.
func redirectFlash(w http.ResponseWriter, r *http.Request, path, flash string) {
	http.Redirect(w, r, path+"?flash="+url.QueryEscape(flash), http.StatusSeeOther)
}

func redirectError(w http.ResponseWriter, r *http.Request, path string, err error) {
	http.Redirect(w, r, path+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
}

// formID parses an int64 form field.
func formID(r *http.Request, field string) (int64, error) {
	return strconv.ParseInt(r.FormValue(field), 10, 64)
}
