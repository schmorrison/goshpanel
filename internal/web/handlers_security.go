package web

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/schmorrison/goshpanel/internal/security"
)

func (s *Server) handleIPRuleCreate(w http.ResponseWriter, r *http.Request) {
	cidr, err := security.NormalizeCIDR(r.FormValue("cidr"))
	if err != nil {
		redirectError(w, r, "/security", err)
		return
	}
	if _, err := s.store.CreateIPRule(cidr, r.FormValue("comment")); err != nil {
		redirectError(w, r, "/security", err)
		return
	}
	if err := s.blocker.Reload(); err != nil {
		s.log.Error("blocker reload failed", "err", err)
	}
	s.audit(r, "security.ip.block", cidr)
	redirectFlash(w, r, "/security", "Blocked "+cidr)
}

func (s *Server) handleIPRuleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := formID(r, "id")
	if err != nil {
		redirectError(w, r, "/security", err)
		return
	}
	if err := s.store.DeleteIPRule(id); err != nil {
		redirectError(w, r, "/security", err)
		return
	}
	if err := s.blocker.Reload(); err != nil {
		s.log.Error("blocker reload failed", "err", err)
	}
	s.audit(r, "security.ip.unblock", strconv.FormatInt(id, 10))
	redirectFlash(w, r, "/security", "Rule removed")
}

func (s *Server) handleUserCreate(w http.ResponseWriter, r *http.Request) {
	if currentUser(r).Role != "admin" {
		redirectError(w, r, "/security", errors.New("only admins can create users"))
		return
	}
	u, err := s.auth.CreateUser(r.FormValue("username"), r.FormValue("password"), r.FormValue("role"))
	if err != nil {
		redirectError(w, r, "/security", err)
		return
	}
	s.audit(r, "security.user.create", u.Username)
	redirectFlash(w, r, "/security", "User created: "+u.Username)
}

func (s *Server) handleUserDelete(w http.ResponseWriter, r *http.Request) {
	if currentUser(r).Role != "admin" {
		redirectError(w, r, "/security", errors.New("only admins can delete users"))
		return
	}
	id, err := formID(r, "id")
	if err != nil {
		redirectError(w, r, "/security", err)
		return
	}
	if id == currentUser(r).ID {
		redirectError(w, r, "/security", errors.New("you cannot delete your own account"))
		return
	}
	if err := s.store.DeleteUser(id); err != nil {
		redirectError(w, r, "/security", err)
		return
	}
	s.audit(r, "security.user.delete", strconv.FormatInt(id, 10))
	redirectFlash(w, r, "/security", "User deleted")
}

func (s *Server) handlePasswordChange(w http.ResponseWriter, r *http.Request) {
	if err := s.auth.ChangePassword(currentUser(r).ID, r.FormValue("password")); err != nil {
		redirectError(w, r, "/security", err)
		return
	}
	s.audit(r, "security.password.change", "own password")
	redirectFlash(w, r, "/security", "Password changed")
}

func (s *Server) handle2FASetup(w http.ResponseWriter, r *http.Request) {
	_, uri, err := s.auth.BeginTOTPSetup(currentUser(r).ID)
	if err != nil {
		redirectError(w, r, "/security", err)
		return
	}
	s.audit(r, "security.2fa.setup", "started")
	http.Redirect(w, r, "/security?totp_uri="+url.QueryEscape(uri), http.StatusSeeOther)
}

func (s *Server) handle2FAEnable(w http.ResponseWriter, r *http.Request) {
	if err := s.auth.ConfirmTOTPSetup(currentUser(r).ID, r.FormValue("totp_code")); err != nil {
		redirectError(w, r, "/security", err)
		return
	}
	s.audit(r, "security.2fa.enable", "enabled")
	redirectFlash(w, r, "/security", "Two-factor authentication enabled")
}

func (s *Server) handleUserFilesSubdir(w http.ResponseWriter, r *http.Request) {
	if currentUser(r).Role != "admin" {
		redirectError(w, r, "/security", errors.New("only admins can edit user file directories"))
		return
	}
	id, err := formID(r, "id")
	if err != nil {
		redirectError(w, r, "/security", err)
		return
	}
	subdir := strings.Trim(strings.TrimSpace(r.FormValue("files_subdir")), "/")
	if strings.Contains(subdir, "..") {
		redirectError(w, r, "/security", errors.New("invalid files subdirectory"))
		return
	}
	if err := s.store.UpdateUserFilesSubdir(id, subdir); err != nil {
		redirectError(w, r, "/security", err)
		return
	}
	s.audit(r, "security.user.files_subdir", strconv.FormatInt(id, 10)+" "+subdir)
	redirectFlash(w, r, "/security", "Files directory updated")
}

func (s *Server) handle2FADisable(w http.ResponseWriter, r *http.Request) {
	if err := s.auth.DisableTOTP(currentUser(r).ID, r.FormValue("totp_code")); err != nil {
		redirectError(w, r, "/security", err)
		return
	}
	s.audit(r, "security.2fa.disable", "disabled")
	redirectFlash(w, r, "/security", "Two-factor authentication disabled")
}
