package web

import (
	"net/http"
	"strconv"

	"github.com/schmorrison/goshpanel/internal/dbmanager"
	"github.com/schmorrison/goshpanel/internal/store"
)

type databasesData struct {
	Conns []store.DatabaseConn

	// Query console state (set after POST /databases/query re-renders).
	ActiveConn store.DatabaseConn
	SQL        string
	Result     *dbmanager.QueryResult
	QueryErr   string
	Tables     []string
}

func (s *Server) handleDatabasesPage(w http.ResponseWriter, r *http.Request) {
	conns, err := s.store.DatabaseConns()
	if err != nil {
		redirectError(w, r, "/", err)
		return
	}
	data := databasesData{Conns: conns}

	// Optional ?conn=ID opens the query console for that connection.
	if idStr := r.URL.Query().Get("conn"); idStr != "" {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err == nil {
			if c, err := s.store.DatabaseConnByID(id); err == nil {
				data.ActiveConn = c
				tables, err := dbmanager.Tables(r.Context(), c.Driver, c.DSN)
				if err != nil {
					data.QueryErr = err.Error()
				} else {
					data.Tables = tables
				}
			}
		}
	}
	s.render(w, r, "databases.html", "Databases", "databases", data)
}

func (s *Server) handleDatabaseCreate(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	driver := r.FormValue("driver")
	dsn := r.FormValue("dsn")
	if err := dbmanager.Ping(r.Context(), driver, dsn); err != nil {
		redirectError(w, r, "/databases", err)
		return
	}
	if _, err := s.store.CreateDatabaseConn(name, driver, dsn); err != nil {
		redirectError(w, r, "/databases", err)
		return
	}
	s.audit(r, "databases.create", name+" ("+driver+")")
	redirectFlash(w, r, "/databases", "Connection saved and verified")
}

func (s *Server) handleDatabaseDelete(w http.ResponseWriter, r *http.Request) {
	id, err := formID(r, "id")
	if err != nil {
		redirectError(w, r, "/databases", err)
		return
	}
	if err := s.store.DeleteDatabaseConn(id); err != nil {
		redirectError(w, r, "/databases", err)
		return
	}
	s.audit(r, "databases.delete", strconv.FormatInt(id, 10))
	redirectFlash(w, r, "/databases", "Connection removed")
}

func (s *Server) handleDatabaseQuery(w http.ResponseWriter, r *http.Request) {
	id, err := formID(r, "conn_id")
	if err != nil {
		redirectError(w, r, "/databases", err)
		return
	}
	conn, err := s.store.DatabaseConnByID(id)
	if err != nil {
		redirectError(w, r, "/databases", err)
		return
	}
	conns, err := s.store.DatabaseConns()
	if err != nil {
		redirectError(w, r, "/databases", err)
		return
	}

	sqlText := r.FormValue("sql")
	data := databasesData{Conns: conns, ActiveConn: conn, SQL: sqlText}
	if tables, err := dbmanager.Tables(r.Context(), conn.Driver, conn.DSN); err == nil {
		data.Tables = tables
	}

	res, err := dbmanager.Query(r.Context(), conn.Driver, conn.DSN, sqlText, 500)
	if err != nil {
		data.QueryErr = err.Error()
	} else {
		data.Result = &res
	}
	s.audit(r, "databases.query", conn.Name)
	s.render(w, r, "databases.html", "Databases", "databases", data)
}
