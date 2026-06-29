package ui

import (
	"errors"
	"net/http"

	"github.com/schmorrison/goshpanel/internal/auth"
	"github.com/schmorrison/goshpanel/internal/ui/pages"
)

// Handler serves HTML pages for the panel UI.
type Handler struct {
	auth *auth.Service
}

// NewHandler returns a UI handler.
func NewHandler(authService *auth.Service) *Handler {
	return &Handler{auth: authService}
}

// Home serves the public landing page.
func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	user, ok, err := h.auth.CurrentUser(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if ok {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	_ = user
	pages.Home().Render(r.Context(), w)
}

// LoginGet renders the login form.
func (h *Handler) LoginGet(w http.ResponseWriter, r *http.Request) {
	pages.Login(h.auth.CSRFToken(r.Context()), "").Render(r.Context(), w)
}

// LoginPost authenticates a user.
func (h *Handler) LoginPost(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	if err := h.auth.Login(r.Context(), w, r, username, password); err != nil {
		message := "Invalid username or password."
		if !errors.Is(err, auth.ErrInvalidCredentials) {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		pages.Login(h.auth.CSRFToken(r.Context()), message).Render(r.Context(), w)
		return
	}

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// LogoutPost ends the current session.
func (h *Handler) LogoutPost(w http.ResponseWriter, r *http.Request) {
	if err := h.auth.Logout(r.Context()); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// Dashboard renders the authenticated overview page.
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	user, ok, err := h.auth.CurrentUser(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	pages.Dashboard(user.Username, h.auth.CSRFToken(r.Context())).Render(r.Context(), w)
}
