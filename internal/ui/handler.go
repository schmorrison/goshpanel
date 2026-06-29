package ui

import (
	"net/http"

	"github.com/schmorrison/goshpanel/internal/ui/pages"
)

// Handler serves HTML pages for the panel UI.
type Handler struct{}

// NewHandler returns a UI handler.
func NewHandler() *Handler {
	return &Handler{}
}

// Home serves the panel landing page.
func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	pages.Home().Render(r.Context(), w)
}
