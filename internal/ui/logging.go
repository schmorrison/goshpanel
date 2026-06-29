package ui

import (
	"fmt"
	"net/http"

	"github.com/schmorrison/goshpanel/internal/ui/pages"
)

// Logging renders the log tailing page.
func (h *Handler) Logging(w http.ResponseWriter, r *http.Request) {
	user, ok, err := h.requireUser(w, r)
	if !ok {
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	source := r.URL.Query().Get("source")
	if source == "" && len(h.logging.Sources()) > 0 {
		source = h.logging.Sources()[0].Name
	}

	pages.LoggingPage(user.Username, h.auth.CSRFToken(r.Context()), h.logging.Sources(), source).Render(r.Context(), w)
}

// LoggingTail streams log lines over server-sent events.
func (h *Handler) LoggingTail(w http.ResponseWriter, r *http.Request) {
	if _, ok, err := h.requireUser(w, r); !ok {
		return
	} else if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	source := r.URL.Query().Get("source")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	if err := h.logging.Tail(r.Context(), source, w, flusher.Flush); err != nil {
		_, _ = fmt.Fprintf(w, "data: error: %v\n\n", err)
		flusher.Flush()
	}
}
