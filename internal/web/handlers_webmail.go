package web

import (
	"net/http"

	"github.com/schmorrison/goshpanel/internal/domains"
	"github.com/schmorrison/goshpanel/internal/store"
)

type webmailData struct {
	Settings store.WebmailSettings
}

func (s *Server) handleWebmailPage(w http.ResponseWriter, r *http.Request) {
	ws, err := s.store.WebmailSettings()
	if err != nil {
		redirectError(w, r, "/", err)
		return
	}
	s.render(w, r, "webmail.html", "Webmail", "webmail", webmailData{Settings: ws})
}

func (s *Server) handleWebmailSave(w http.ResponseWriter, r *http.Request) {
	ws := store.WebmailSettings{
		Host:     r.FormValue("host"),
		Upstream: r.FormValue("upstream"),
		Enabled:  r.FormValue("enabled") == "1",
	}
	if ws.Enabled {
		if err := domains.ValidateName(ws.Host); err != nil {
			redirectError(w, r, "/webmail", err)
			return
		}
		if ws.Upstream == "" {
			redirectError(w, r, "/webmail", errRequiredFields)
			return
		}
	}
	if err := s.store.SaveWebmailSettings(ws); err != nil {
		redirectError(w, r, "/webmail", err)
		return
	}
	if s.orch != nil {
		_ = s.orch.ApplyCaddy(r.Context())
	}
	s.audit(r, "webmail.save", ws.Host)
	redirectFlash(w, r, "/webmail", "Webmail settings saved")
}
