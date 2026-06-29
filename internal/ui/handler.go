package ui

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/schmorrison/goshpanel/internal/auth"
	"github.com/schmorrison/goshpanel/internal/files"
	"github.com/schmorrison/goshpanel/internal/store"
	"github.com/schmorrison/goshpanel/internal/ui/pages"
)

const maxUploadSize = 32 << 20

// Handler serves HTML pages for the panel UI.
type Handler struct {
	auth  *auth.Service
	files *files.Service
}

// NewHandler returns a UI handler.
func NewHandler(authService *auth.Service, filesService *files.Service) *Handler {
	return &Handler{
		auth:  authService,
		files: filesService,
	}
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
	user, ok, err := h.requireUser(w, r)
	if !ok {
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	pages.Dashboard(user.Username, h.auth.CSRFToken(r.Context())).Render(r.Context(), w)
}

// Files renders the file manager page or HTMX partial.
func (h *Handler) Files(w http.ResponseWriter, r *http.Request) {
	h.renderFiles(w, r, r.URL.Query().Get("path"), "")
}

// FilesList renders the file table partial for HTMX navigation.
func (h *Handler) FilesList(w http.ResponseWriter, r *http.Request) {
	h.renderFiles(w, r, r.URL.Query().Get("path"), "")
}

// FilesUpload handles file uploads inside the sandbox.
func (h *Handler) FilesUpload(w http.ResponseWriter, r *http.Request) {
	user, ok, err := h.requireUser(w, r)
	if !ok {
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		h.renderFiles(w, r, r.FormValue("path"), "Upload failed: file is too large or invalid.")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		h.renderFiles(w, r, r.FormValue("path"), "Upload failed: missing file.")
		return
	}
	defer file.Close()

	if err := h.files.Upload(r.Context(), user.ID, user.Username, r.FormValue("path"), header.Filename, file); err != nil {
		h.renderFiles(w, r, r.FormValue("path"), fmt.Sprintf("Upload failed: %v", err))
		return
	}

	h.renderFiles(w, r, r.FormValue("path"), "")
}

// FilesDelete removes a file or empty directory.
func (h *Handler) FilesDelete(w http.ResponseWriter, r *http.Request) {
	user, ok, err := h.requireUser(w, r)
	if !ok {
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	targetPath := r.FormValue("path")
	currentPath := r.FormValue("current_path")
	if err := h.files.Delete(r.Context(), user.ID, user.Username, targetPath); err != nil {
		h.renderFiles(w, r, currentPath, fmt.Sprintf("Delete failed: %v", err))
		return
	}

	h.renderFiles(w, r, currentPath, "")
}

func (h *Handler) renderFiles(w http.ResponseWriter, r *http.Request, path string, errorMessage string) {
	user, ok, err := h.requireUser(w, r)
	if !ok {
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	entries, err := h.files.List(r.Context(), user.ID, user.Username, path)
	if err != nil {
		errorMessage = fmt.Sprintf("Could not load directory: %v", err)
		entries = nil
	}

	csrfToken := h.auth.CSRFToken(r.Context())
	if r.Header.Get("HX-Request") == "true" {
		pages.FilesList(path, entries, csrfToken, errorMessage).Render(r.Context(), w)
		return
	}

	pages.FilesPage(user.Username, csrfToken, path, entries, errorMessage).Render(r.Context(), w)
}

func (h *Handler) requireUser(w http.ResponseWriter, r *http.Request) (store.User, bool, error) {
	user, ok, err := h.auth.CurrentUser(r.Context())
	if err != nil {
		return store.User{}, false, err
	}
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return store.User{}, false, nil
	}
	return user, true, nil
}
