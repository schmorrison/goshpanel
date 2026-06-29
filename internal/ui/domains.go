package ui

import (
	"net/http"
	"strconv"

	"github.com/schmorrison/goshpanel/internal/ui/pages"
)

// Domains renders the virtual host manager.
func (h *Handler) Domains(w http.ResponseWriter, r *http.Request) {
	user, ok, err := h.requireUser(w, r)
	if !ok {
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	sites, err := h.domains.List(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	caddyfile, _ := h.domains.RenderCaddyfile(r.Context())
	pages.DomainsPage(user.Username, h.auth.CSRFToken(r.Context()), sites, caddyfile, r.URL.Query().Get("message"), r.URL.Query().Get("error")).Render(r.Context(), w)
}

// DomainsCreate adds a virtual host.
func (h *Handler) DomainsCreate(w http.ResponseWriter, r *http.Request) {
	if _, ok, err := h.requireUser(w, r); !ok {
		return
	} else if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if _, err := h.domains.Create(r.Context(), r.FormValue("domain"), r.FormValue("upstream")); err != nil {
		http.Redirect(w, r, "/domains?error="+urlQueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/domains?message=Site+created", http.StatusSeeOther)
}

// DomainsDelete removes a virtual host.
func (h *Handler) DomainsDelete(w http.ResponseWriter, r *http.Request) {
	if _, ok, err := h.requireUser(w, r); !ok {
		return
	} else if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil {
		http.Redirect(w, r, "/domains?error=Invalid+site+id", http.StatusSeeOther)
		return
	}
	if err := h.domains.Delete(r.Context(), id); err != nil {
		http.Redirect(w, r, "/domains?error="+urlQueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/domains?message=Site+deleted", http.StatusSeeOther)
}

// DomainsApply writes and reloads the Caddy configuration.
func (h *Handler) DomainsApply(w http.ResponseWriter, r *http.Request) {
	if _, ok, err := h.requireUser(w, r); !ok {
		return
	} else if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	message, err := h.domains.Apply(r.Context())
	if err != nil {
		http.Redirect(w, r, "/domains?error="+urlQueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/domains?message="+urlQueryEscape(message), http.StatusSeeOther)
}
