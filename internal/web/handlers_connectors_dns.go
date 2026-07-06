package web

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/schmorrison/goshpanel/internal/api"
	"github.com/schmorrison/goshpanel/internal/shortener"
	"github.com/schmorrison/goshpanel/internal/store"
)

type dnsConnectorData struct {
	Connector  store.ServiceConnector
	ConfigPath string
	Content    string
	Logs       string
	Connectors []store.ServiceConnector
}

func (s *Server) handleCoreDNSConnectorPage(w http.ResponseWriter, r *http.Request) {
	list, _ := s.connect.List()
	conn, paths, err := s.connect.EffectiveCoreDNS()
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
	client, err := s.connect.CoreDNS(conn, paths)
	if err != nil {
		redirectError(w, r, "/connectors", err)
		return
	}
	content, _ := client.ReadCorefile(r.Context())
	logs, _ := client.Logs(r.Context(), 150)
	s.render(w, r, "connectors_coredns.html", "CoreDNS", "connectors", dnsConnectorData{
		Connector:  conn,
		ConfigPath: client.ConfigPath(),
		Content:    content,
		Logs:       logs,
		Connectors: filterConnectors(list, "coredns"),
	})
}

func (s *Server) handleCoreDNSConnectorSave(w http.ResponseWriter, r *http.Request) {
	id, _ := formID(r, "connector_id")
	conn, paths, err := s.connect.EffectiveCoreDNS()
	if err != nil {
		redirectError(w, r, "/connectors/coredns", err)
		return
	}
	if id > 0 {
		if c, err := s.store.ServiceConnectorByID(id); err == nil {
			conn = c
		}
	}
	client, err := s.connect.CoreDNS(conn, paths)
	if err != nil {
		redirectError(w, r, "/connectors/coredns", err)
		return
	}
	if err := client.WriteCorefile(r.Context(), r.FormValue("corefile")); err != nil {
		redirectError(w, r, "/connectors/coredns?id="+strconv.FormatInt(conn.ID, 10), err)
		return
	}
	s.audit(r, "coredns.write", conn.Name)
	redirectFlash(w, r, "/connectors/coredns?id="+strconv.FormatInt(conn.ID, 10), "Corefile saved")
}

func (s *Server) handleCoreDNSConnectorReload(w http.ResponseWriter, r *http.Request) {
	id, _ := formID(r, "connector_id")
	conn, paths, err := s.connect.EffectiveCoreDNS()
	if err != nil {
		redirectError(w, r, "/connectors/coredns", err)
		return
	}
	if id > 0 {
		if c, err := s.store.ServiceConnectorByID(id); err == nil {
			conn = c
		}
	}
	client, err := s.connect.CoreDNS(conn, paths)
	if err != nil {
		redirectError(w, r, "/connectors/coredns", err)
		return
	}
	if err := client.Reload(r.Context()); err != nil {
		redirectError(w, r, "/connectors/coredns?id="+strconv.FormatInt(conn.ID, 10), err)
		return
	}
	s.audit(r, "coredns.reload", conn.Name)
	redirectFlash(w, r, "/connectors/coredns?id="+strconv.FormatInt(conn.ID, 10), "CoreDNS reloaded")
}

func (s *Server) handleMaddyConnectorPage(w http.ResponseWriter, r *http.Request) {
	list, _ := s.connect.List()
	conn, paths, err := s.connect.EffectiveMaddy()
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
	client, err := s.connect.Maddy(conn, paths)
	if err != nil {
		redirectError(w, r, "/connectors", err)
		return
	}
	content, _ := client.Read(r.Context())
	logs, _ := client.Logs(r.Context(), 150)
	s.render(w, r, "connectors_maddy.html", "Maddy", "connectors", dnsConnectorData{
		Connector:  conn,
		ConfigPath: client.ConfigPath(),
		Content:    content,
		Logs:       logs,
		Connectors: filterConnectors(list, "maddy"),
	})
}

func (s *Server) handleMaddyConnectorSave(w http.ResponseWriter, r *http.Request) {
	id, _ := formID(r, "connector_id")
	conn, paths, err := s.connect.EffectiveMaddy()
	if err != nil {
		redirectError(w, r, "/connectors/maddy", err)
		return
	}
	if id > 0 {
		if c, err := s.store.ServiceConnectorByID(id); err == nil {
			conn = c
		}
	}
	client, err := s.connect.Maddy(conn, paths)
	if err != nil {
		redirectError(w, r, "/connectors/maddy", err)
		return
	}
	if err := client.Write(r.Context(), r.FormValue("config")); err != nil {
		redirectError(w, r, "/connectors/maddy?id="+strconv.FormatInt(conn.ID, 10), err)
		return
	}
	s.audit(r, "maddy.write", conn.Name)
	redirectFlash(w, r, "/connectors/maddy?id="+strconv.FormatInt(conn.ID, 10), "maddy.conf saved")
}

func (s *Server) handleMaddyConnectorReload(w http.ResponseWriter, r *http.Request) {
	id, _ := formID(r, "connector_id")
	conn, paths, err := s.connect.EffectiveMaddy()
	if err != nil {
		redirectError(w, r, "/connectors/maddy", err)
		return
	}
	if id > 0 {
		if c, err := s.store.ServiceConnectorByID(id); err == nil {
			conn = c
		}
	}
	client, err := s.connect.Maddy(conn, paths)
	if err != nil {
		redirectError(w, r, "/connectors/maddy", err)
		return
	}
	if err := client.Reload(r.Context()); err != nil {
		redirectError(w, r, "/connectors/maddy?id="+strconv.FormatInt(conn.ID, 10), err)
		return
	}
	s.audit(r, "maddy.reload", conn.Name)
	redirectFlash(w, r, "/connectors/maddy?id="+strconv.FormatInt(conn.ID, 10), "Maddy reloaded")
}

// API handlers for short links and webhooks.

type apiShortLinkReq struct {
	Code      string `json:"code"`
	TargetURL string `json:"target_url"`
	Host      string `json:"host"`
}

type apiWebhookOutboundReq struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Secret string `json:"secret"`
	Events string `json:"events"`
}

type apiWebhookOutbound struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	Events     string `json:"events"`
	Enabled    bool   `json:"enabled"`
	LastStatus int    `json:"last_status"`
	LastError  string `json:"last_error,omitempty"`
}

func (s *Server) handleAPIShortLinks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.store.ShortLinks()
		if err != nil {
			api.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		api.WriteJSON(w, http.StatusOK, list)
	case http.MethodPost:
		var req apiShortLinkReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			api.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		code, err := shortener.NormalizeCode(req.Code)
		if err != nil {
			api.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		target, err := shortener.ValidateTarget(req.TargetURL)
		if err != nil {
			api.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		link, err := s.store.CreateShortLink(code, target, req.Host)
		if err != nil {
			api.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		api.WriteJSON(w, http.StatusCreated, link)
	default:
		api.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleAPIShortLinkByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "bad id")
		return
	}
	switch r.Method {
	case http.MethodGet:
		link, err := s.store.ShortLinkByID(id)
		if err != nil {
			api.WriteError(w, http.StatusNotFound, "not found")
			return
		}
		api.WriteJSON(w, http.StatusOK, link)
	case http.MethodDelete:
		if err := s.store.DeleteShortLink(id); err != nil {
			api.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]string{"ok": "true"})
	default:
		api.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleAPIWebhookOutbound(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		subs, err := s.store.WebhookSubscriptions()
		if err != nil {
			api.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		out := make([]apiWebhookOutbound, 0, len(subs))
		for _, sub := range subs {
			out = append(out, apiWebhookOutbound{
				ID: sub.ID, Name: sub.Name, URL: sub.URL, Events: sub.Events,
				Enabled: sub.Enabled, LastStatus: sub.LastStatus, LastError: sub.LastError,
			})
		}
		api.WriteJSON(w, http.StatusOK, out)
	case http.MethodPost:
		var req apiWebhookOutboundReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			api.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		secret := req.Secret
		if secret == "" {
			secret, err := shortener.RandomCode(24)
			if err != nil {
				api.WriteError(w, http.StatusInternalServerError, err.Error())
				return
			}
			req.Secret = secret
		}
		events := req.Events
		if events == "" {
			events = "*"
		}
		sub, err := s.store.CreateWebhookSubscription(req.Name, req.URL, req.Secret, events)
		if err != nil {
			api.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		api.WriteJSON(w, http.StatusCreated, apiWebhookOutbound{
			ID: sub.ID, Name: sub.Name, URL: sub.URL, Events: sub.Events, Enabled: sub.Enabled,
		})
	default:
		api.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleAPIWebhookOutboundByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "bad id")
		return
	}
	switch r.Method {
	case http.MethodGet:
		sub, err := s.store.WebhookSubscriptionByID(id)
		if err != nil {
			api.WriteError(w, http.StatusNotFound, "not found")
			return
		}
		api.WriteJSON(w, http.StatusOK, apiWebhookOutbound{
			ID: sub.ID, Name: sub.Name, URL: sub.URL, Events: sub.Events,
			Enabled: sub.Enabled, LastStatus: sub.LastStatus, LastError: sub.LastError,
		})
	case http.MethodDelete:
		if err := s.store.DeleteWebhookSubscription(id); err != nil {
			api.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]string{"ok": "true"})
	default:
		api.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleAPIWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "bad id")
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	list, err := s.store.WebhookDeliveries(id, limit)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	api.WriteJSON(w, http.StatusOK, list)
}

func (s *Server) handleAPIWebhookInbound(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	list, err := s.store.InboundWebhooks()
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	api.WriteJSON(w, http.StatusOK, list)
}
