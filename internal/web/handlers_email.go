package web

import (
	"net/http"
	"os"
	"strconv"

	"github.com/schmorrison/goshpanel/internal/crypto"
	"github.com/schmorrison/goshpanel/internal/email"
	"github.com/schmorrison/goshpanel/internal/store"
)

type emailData struct {
	Mailboxes  []store.Mailbox
	Forwarders []store.EmailForwarder
	Domains    []string
}

func (s *Server) handleEmailPage(w http.ResponseWriter, r *http.Request) {
	boxes, err := s.store.Mailboxes()
	if err != nil {
		redirectError(w, r, "/", err)
		return
	}
	fwds, err := s.store.Forwarders()
	if err != nil {
		redirectError(w, r, "/", err)
		return
	}
	s.render(w, r, "email.html", "Email", "email", emailData{
		Mailboxes: boxes, Forwarders: fwds, Domains: email.Domains(boxes),
	})
}

func (s *Server) handleMailboxCreate(w http.ResponseWriter, r *http.Request) {
	addr := r.FormValue("address")
	if err := email.ValidateAddress(addr); err != nil {
		redirectError(w, r, "/email", err)
		return
	}
	hash, err := crypto.HashPassword(r.FormValue("password"))
	if err != nil {
		redirectError(w, r, "/email", err)
		return
	}
	quota, _ := strconv.Atoi(r.FormValue("quota_mb"))
	if _, err := s.store.CreateMailbox(addr, hash, quota); err != nil {
		redirectError(w, r, "/email", err)
		return
	}
	s.audit(r, "email.mailbox.create", addr)
	s.maybeAutoApply(r.Context())
	redirectFlash(w, r, "/email", "Mailbox created: "+addr)
}

func (s *Server) handleMailboxDelete(w http.ResponseWriter, r *http.Request) {
	id, err := formID(r, "id")
	if err != nil {
		redirectError(w, r, "/email", err)
		return
	}
	if err := s.store.DeleteMailbox(id); err != nil {
		redirectError(w, r, "/email", err)
		return
	}
	s.audit(r, "email.mailbox.delete", strconv.FormatInt(id, 10))
	s.maybeAutoApply(r.Context())
	redirectFlash(w, r, "/email", "Mailbox removed")
}

func (s *Server) handleForwarderCreate(w http.ResponseWriter, r *http.Request) {
	from, to := r.FormValue("from"), r.FormValue("to")
	if err := email.ValidateAddress(from); err != nil {
		redirectError(w, r, "/email", err)
		return
	}
	if err := email.ValidateAddress(to); err != nil {
		redirectError(w, r, "/email", err)
		return
	}
	if _, err := s.store.CreateForwarder(from, to); err != nil {
		redirectError(w, r, "/email", err)
		return
	}
	s.audit(r, "email.forwarder.create", from+" -> "+to)
	s.maybeAutoApply(r.Context())
	redirectFlash(w, r, "/email", "Forwarder created")
}

func (s *Server) handleForwarderDelete(w http.ResponseWriter, r *http.Request) {
	id, err := formID(r, "id")
	if err != nil {
		redirectError(w, r, "/email", err)
		return
	}
	if err := s.store.DeleteForwarder(id); err != nil {
		redirectError(w, r, "/email", err)
		return
	}
	s.audit(r, "email.forwarder.delete", strconv.FormatInt(id, 10))
	s.maybeAutoApply(r.Context())
	redirectFlash(w, r, "/email", "Forwarder removed")
}

func (s *Server) handleMaddyConfig(w http.ResponseWriter, r *http.Request) {
	boxes, err := s.store.Mailboxes()
	if err != nil {
		redirectError(w, r, "/email", err)
		return
	}
	fwds, err := s.store.Forwarders()
	if err != nil {
		redirectError(w, r, "/email", err)
		return
	}
	hostname, _ := os.Hostname()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="maddy.conf"`)
	w.Write([]byte(email.RenderMaddyConfig(boxes, fwds, hostname)))
}
