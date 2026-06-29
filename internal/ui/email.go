package ui

import (
	"net/http"
	"strconv"

	"github.com/schmorrison/goshpanel/internal/ui/pages"
)

// Email renders mailbox management.
func (h *Handler) Email(w http.ResponseWriter, r *http.Request) {
	user, ok, err := h.requireUser(w, r)
	if !ok {
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	mailboxes, err := h.email.List(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	pages.EmailPage(user.Username, h.auth.CSRFToken(r.Context()), mailboxes, h.emailDefaultDomain, r.URL.Query().Get("message"), r.URL.Query().Get("error")).Render(r.Context(), w)
}

// EmailCreate adds a mailbox.
func (h *Handler) EmailCreate(w http.ResponseWriter, r *http.Request) {
	if _, ok, err := h.requireUser(w, r); !ok {
		return
	} else if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if _, err := h.email.Create(r.Context(), r.FormValue("local_part"), r.FormValue("domain"), r.FormValue("password"), r.FormValue("forward_to")); err != nil {
		http.Redirect(w, r, "/email?error="+urlQueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/email?message=Mailbox+created", http.StatusSeeOther)
}

// EmailDelete removes a mailbox.
func (h *Handler) EmailDelete(w http.ResponseWriter, r *http.Request) {
	if _, ok, err := h.requireUser(w, r); !ok {
		return
	} else if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil {
		http.Redirect(w, r, "/email?error=Invalid+mailbox+id", http.StatusSeeOther)
		return
	}
	if err := h.email.Delete(r.Context(), id); err != nil {
		http.Redirect(w, r, "/email?error="+urlQueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/email?message=Mailbox+deleted", http.StatusSeeOther)
}

// EmailExport writes go-guerrilla configuration to disk.
func (h *Handler) EmailExport(w http.ResponseWriter, r *http.Request) {
	if _, ok, err := h.requireUser(w, r); !ok {
		return
	} else if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	message, err := h.email.ExportConfig(r.Context())
	if err != nil {
		http.Redirect(w, r, "/email?error="+urlQueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/email?message="+urlQueryEscape(message), http.StatusSeeOther)
}
