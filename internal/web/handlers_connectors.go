package web

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/schmorrison/goshpanel/internal/connector"
	"github.com/schmorrison/goshpanel/internal/dbmanager"
	"github.com/schmorrison/goshpanel/internal/store"
)

type connectorsData struct {
	Connectors []store.ServiceConnector
	Containers string
}

type caddyConnectorData struct {
	Connector   store.ServiceConnector
	ConfigPath  string
	Caddyfile   string
	Logs        string
	LogSource   string
	Connectors  []store.ServiceConnector
}

func (s *Server) handleConnectorsPage(w http.ResponseWriter, r *http.Request) {
	list, err := s.connect.List()
	if err != nil {
		redirectError(w, r, "/", err)
		return
	}
	data := connectorsData{Connectors: list}
	if s.docker != nil {
		if c, err := s.connect.Default(connector.KindDocker); err == nil {
			if cli, err := s.connect.DockerService(c); err == nil {
				data.Containers, _ = cli.Containers(r.Context())
			}
		}
	}
	s.render(w, r, "connectors.html", "Connectors", "connectors", data)
}

func (s *Server) handleConnectorCreate(w http.ResponseWriter, r *http.Request) {
	kind := r.FormValue("kind")
	if err := store.ValidateConnectorKind(kind); err != nil {
		redirectError(w, r, "/connectors", err)
		return
	}
	mode := r.FormValue("mode")
	if mode == "" {
		mode = string(connector.ModeLocal)
	}
	cfg := buildConnectorConfig(kind, mode, r)
	raw, err := json.Marshal(cfg)
	if err != nil {
		redirectError(w, r, "/connectors", err)
		return
	}
	c := store.ServiceConnector{
		Name:       r.FormValue("name"),
		Kind:       kind,
		Mode:       mode,
		ConfigJSON: string(raw),
		Enabled:    r.FormValue("enabled") == "1" || r.FormValue("enabled") == "on",
		IsDefault:  r.FormValue("is_default") == "1" || r.FormValue("is_default") == "on",
	}
	if _, err := s.store.CreateServiceConnector(c); err != nil {
		redirectError(w, r, "/connectors", err)
		return
	}
	s.audit(r, "connectors.create", c.Name+" ("+kind+")")
	redirectFlash(w, r, "/connectors", "Connector saved")
}

func buildConnectorConfig(kind, mode string, r *http.Request) any {
	switch kind {
	case "caddy":
		return connector.CaddyConfig{
			Container:           r.FormValue("container"),
			ConfigPathContainer: r.FormValue("config_path_container"),
			ConfigPathHost:      r.FormValue("config_path_host"),
			AccessLogPath:       r.FormValue("access_log_path"),
			DockerHost:          r.FormValue("docker_host"),
		}
	case "docker":
		return connector.DockerConfig{DockerHost: r.FormValue("docker_host")}
	case "postgres", "mysql":
		port, _ := strconv.Atoi(r.FormValue("port"))
		return connector.DatabaseConfig{
			Host:            r.FormValue("host"),
			Port:            port,
			AdminUser:       r.FormValue("admin_user"),
			AdminPassword:   r.FormValue("admin_password"),
			DefaultDatabase: r.FormValue("default_database"),
			SSLMode:         r.FormValue("ssl_mode"),
		}
	default:
		return map[string]string{}
	}
}

func (s *Server) handleConnectorDelete(w http.ResponseWriter, r *http.Request) {
	id, err := formID(r, "id")
	if err != nil {
		redirectError(w, r, "/connectors", err)
		return
	}
	if err := s.store.DeleteServiceConnector(id); err != nil {
		redirectError(w, r, "/connectors", err)
		return
	}
	s.audit(r, "connectors.delete", strconv.FormatInt(id, 10))
	redirectFlash(w, r, "/connectors", "Connector removed")
}

func (s *Server) handleConnectorPing(w http.ResponseWriter, r *http.Request) {
	id, err := formID(r, "id")
	if err != nil {
		redirectError(w, r, "/connectors", err)
		return
	}
	c, err := s.store.ServiceConnectorByID(id)
	if err != nil {
		redirectError(w, r, "/connectors", err)
		return
	}
	if err := s.connect.Ping(r.Context(), c); err != nil {
		redirectError(w, r, "/connectors", err)
		return
	}
	redirectFlash(w, r, "/connectors", c.Name+" is reachable")
}

func (s *Server) handleConnectorDefault(w http.ResponseWriter, r *http.Request) {
	id, err := formID(r, "id")
	if err != nil {
		redirectError(w, r, "/connectors", err)
		return
	}
	c, err := s.store.ServiceConnectorByID(id)
	if err != nil {
		redirectError(w, r, "/connectors", err)
		return
	}
	c.IsDefault = true
	if err := s.store.UpdateServiceConnector(c); err != nil {
		redirectError(w, r, "/connectors", err)
		return
	}
	redirectFlash(w, r, "/connectors", c.Name+" is now the default "+c.Kind+" connector")
}

func (s *Server) handleCaddyConnectorPage(w http.ResponseWriter, r *http.Request) {
	list, _ := s.connect.List()
	conn, paths, err := s.connect.EffectiveCaddy()
	if err != nil {
		redirectError(w, r, "/connectors", err)
		return
	}
	if idStr := r.URL.Query().Get("id"); idStr != "" {
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			if c, err := s.store.ServiceConnectorByID(id); err == nil {
				conn = c
			}
		}
	}
	client, err := s.connect.Caddy(conn, paths)
	if err != nil {
		redirectError(w, r, "/connectors", err)
		return
	}
	content, _ := client.Read(r.Context())
	logs, _ := client.Logs(r.Context(), 150)
	logSource := "access log file"
	if conn.Mode == string(connector.ModeDocker) && conn.ConfigJSON != "" {
		if cfg, err := connector.ParseCaddyConfig(conn.ConfigJSON); err == nil && cfg.Container != "" {
			logSource = "docker logs: " + cfg.Container
		}
	}
	s.render(w, r, "connectors_caddy.html", "Caddy", "connectors", caddyConnectorData{
		Connector:  conn,
		ConfigPath: client.ConfigPath(),
		Caddyfile:  content,
		Logs:       logs,
		LogSource:  logSource,
		Connectors: filterConnectors(list, "caddy"),
	})
}

func filterConnectors(list []store.ServiceConnector, kind string) []store.ServiceConnector {
	var out []store.ServiceConnector
	for _, c := range list {
		if c.Kind == kind {
			out = append(out, c)
		}
	}
	return out
}

func (s *Server) handleCaddyConnectorSave(w http.ResponseWriter, r *http.Request) {
	id, _ := formID(r, "connector_id")
	conn, paths, err := s.connect.EffectiveCaddy()
	if err != nil {
		redirectError(w, r, "/connectors/caddy", err)
		return
	}
	if id > 0 {
		if c, err := s.store.ServiceConnectorByID(id); err == nil {
			conn = c
		}
	}
	client, err := s.connect.Caddy(conn, paths)
	if err != nil {
		redirectError(w, r, "/connectors/caddy", err)
		return
	}
	if err := client.Write(r.Context(), r.FormValue("caddyfile")); err != nil {
		redirectError(w, r, "/connectors/caddy?id="+strconv.FormatInt(conn.ID, 10), err)
		return
	}
	s.audit(r, "caddy.write", conn.Name)
	redirectFlash(w, r, "/connectors/caddy?id="+strconv.FormatInt(conn.ID, 10), "Caddyfile saved")
}

func (s *Server) handleCaddyConnectorReload(w http.ResponseWriter, r *http.Request) {
	id, _ := formID(r, "connector_id")
	conn, paths, err := s.connect.EffectiveCaddy()
	if err != nil {
		redirectError(w, r, "/connectors/caddy", err)
		return
	}
	if id > 0 {
		if c, err := s.store.ServiceConnectorByID(id); err == nil {
			conn = c
		}
	}
	client, err := s.connect.Caddy(conn, paths)
	if err != nil {
		redirectError(w, r, "/connectors/caddy", err)
		return
	}
	if err := client.Reload(r.Context()); err != nil {
		redirectError(w, r, "/connectors/caddy?id="+strconv.FormatInt(conn.ID, 10), err)
		return
	}
	s.audit(r, "caddy.reload", conn.Name)
	redirectFlash(w, r, "/connectors/caddy?id="+strconv.FormatInt(conn.ID, 10), "Caddy reloaded")
}

func (s *Server) handleDatabaseProvision(w http.ResponseWriter, r *http.Request) {
	id, err := formID(r, "connector_id")
	if err != nil {
		redirectError(w, r, "/databases", err)
		return
	}
	c, err := s.store.ServiceConnectorByID(id)
	if err != nil {
		redirectError(w, r, "/databases", err)
		return
	}
	cfg, err := connector.ParseDatabaseConfig(c.ConfigJSON)
	if err != nil {
		redirectError(w, r, "/databases", err)
		return
	}
	dbName := r.FormValue("database")
	username := r.FormValue("username")
	password := r.FormValue("password")
	if err := connector.ProvisionDatabase(r.Context(), connector.Kind(c.Kind), cfg, dbName, username, password, r.FormValue("privileges")); err != nil {
		redirectError(w, r, "/databases", err)
		return
	}
	if username != "" && password != "" {
		driver, dsn, err := connector.DatabaseDSN(connector.Kind(c.Kind), cfg)
		if err == nil {
			appDSN := buildAppDSN(connector.Kind(c.Kind), cfg, username, password, dbName)
			name := r.FormValue("conn_name")
			if name == "" {
				name = dbName
			}
			if err := dbmanager.Ping(r.Context(), driver, appDSN); err == nil {
				_, _ = s.store.CreateDatabaseConn(name, driver, appDSN)
			} else {
				_, _ = s.store.CreateDatabaseConn(name, driver, dsn)
			}
		}
	}
	s.audit(r, "databases.provision", c.Name+":"+dbName)
	redirectFlash(w, r, "/databases", "Database provisioned on "+c.Name)
}

func buildAppDSN(kind connector.Kind, cfg connector.DatabaseConfig, user, pass, db string) string {
	switch kind {
	case connector.KindPostgres:
		port := cfg.Port
		if port == 0 {
			port = 5432
		}
		ssl := cfg.SSLMode
		if ssl == "" {
			ssl = "disable"
		}
		return "postgres://" + user + ":" + pass + "@" + cfg.Host + ":" + strconv.Itoa(port) + "/" + db + "?sslmode=" + ssl
	case connector.KindMySQL:
		port := cfg.Port
		if port == 0 {
			port = 3306
		}
		return user + ":" + pass + "@tcp(" + cfg.Host + ":" + strconv.Itoa(port) + ")/" + db
	default:
		return ""
	}
}
