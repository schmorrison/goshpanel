package ui

import (
	"net/http"
	"strconv"

	"github.com/schmorrison/goshpanel/internal/dbmanager"
	"github.com/schmorrison/goshpanel/internal/store"
	"github.com/schmorrison/goshpanel/internal/ui/pages"
)

// Database renders saved SQL connections.
func (h *Handler) Database(w http.ResponseWriter, r *http.Request) {
	user, ok, err := h.requireUser(w, r)
	if !ok {
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	connections, err := h.dbmanager.ListConnections(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	pages.DatabasePage(user.Username, h.auth.CSRFToken(r.Context()), connections, r.URL.Query().Get("message"), r.URL.Query().Get("error")).Render(r.Context(), w)
}

// DatabaseCreate stores a new SQL connection.
func (h *Handler) DatabaseCreate(w http.ResponseWriter, r *http.Request) {
	if _, ok, err := h.requireUser(w, r); !ok {
		return
	} else if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	port, _ := strconv.Atoi(r.FormValue("port"))
	conn := store.DBConnection{
		Name:         r.FormValue("name"),
		Driver:       r.FormValue("driver"),
		Host:         r.FormValue("host"),
		Port:         port,
		DatabaseName: r.FormValue("database_name"),
		Username:     r.FormValue("username"),
	}
	if _, err := h.dbmanager.CreateConnection(r.Context(), conn, r.FormValue("password")); err != nil {
		http.Redirect(w, r, "/database?error="+urlQueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/database?message=Connection+saved", http.StatusSeeOther)
}

// DatabaseDelete removes a saved connection.
func (h *Handler) DatabaseDelete(w http.ResponseWriter, r *http.Request) {
	if _, ok, err := h.requireUser(w, r); !ok {
		return
	} else if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil {
		http.Redirect(w, r, "/database?error=Invalid+connection+id", http.StatusSeeOther)
		return
	}
	if err := h.dbmanager.DeleteConnection(r.Context(), id); err != nil {
		http.Redirect(w, r, "/database?error="+urlQueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/database?message=Connection+deleted", http.StatusSeeOther)
}

// DatabaseView renders schema browser and query runner.
func (h *Handler) DatabaseView(w http.ResponseWriter, r *http.Request) {
	user, ok, err := h.requireUser(w, r)
	if !ok {
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		http.Redirect(w, r, "/database?error=Invalid+connection+id", http.StatusSeeOther)
		return
	}

	connections, err := h.dbmanager.ListConnections(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var conn store.DBConnection
	for _, item := range connections {
		if item.ID == id {
			conn = item
			break
		}
	}
	if conn.ID == 0 {
		http.Redirect(w, r, "/database?error=Connection+not+found", http.StatusSeeOther)
		return
	}

	tables, err := h.dbmanager.ListTables(r.Context(), id)
	if err != nil {
		pages.DatabaseViewPage(user.Username, h.auth.CSRFToken(r.Context()), conn, nil, "", dbmanager.QueryResult{}, err.Error()).Render(r.Context(), w)
		return
	}

	pages.DatabaseViewPage(user.Username, h.auth.CSRFToken(r.Context()), conn, tables, "", dbmanager.QueryResult{}, "").Render(r.Context(), w)
}

// DatabaseQuery runs a read-only SQL query.
func (h *Handler) DatabaseQuery(w http.ResponseWriter, r *http.Request) {
	user, ok, err := h.requireUser(w, r)
	if !ok {
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		http.Redirect(w, r, "/database?error=Invalid+connection+id", http.StatusSeeOther)
		return
	}

	connections, err := h.dbmanager.ListConnections(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var conn store.DBConnection
	for _, item := range connections {
		if item.ID == id {
			conn = item
			break
		}
	}
	if conn.ID == 0 {
		http.Redirect(w, r, "/database?error=Connection+not+found", http.StatusSeeOther)
		return
	}

	query := r.FormValue("query")
	tables, _ := h.dbmanager.ListTables(r.Context(), id)
	result, qerr := h.dbmanager.RunReadOnlyQuery(r.Context(), id, query)
	errMessage := ""
	if qerr != nil {
		errMessage = qerr.Error()
	}
	pages.DatabaseViewPage(user.Username, h.auth.CSRFToken(r.Context()), conn, tables, query, result, errMessage).Render(r.Context(), w)
}
