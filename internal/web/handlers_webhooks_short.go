package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/schmorrison/goshpanel/internal/shortener"
	"github.com/schmorrison/goshpanel/internal/store"
)

type shortLinksData struct {
	Links   []store.ShortLink
	BaseURL string
}

type webhooksData struct {
	Outbound []store.WebhookSubscription
	Inbound  []store.InboundWebhook
	BaseURL  string
}

func (s *Server) handleShortLinksPage(w http.ResponseWriter, r *http.Request) {
	links, _ := s.store.ShortLinks()
	s.render(w, r, "short.html", "URL Shortener", "short", shortLinksData{
		Links:   links,
		BaseURL: requestBaseURL(r),
	})
}

func (s *Server) handleShortLinkCreate(w http.ResponseWriter, r *http.Request) {
	code, err := shortener.NormalizeCode(r.FormValue("code"))
	if err != nil {
		redirectError(w, r, "/short", err)
		return
	}
	target, err := shortener.ValidateTarget(r.FormValue("target_url"))
	if err != nil {
		redirectError(w, r, "/short", err)
		return
	}
	host := r.FormValue("host")
	if _, err := s.store.CreateShortLink(code, target, host); err != nil {
		redirectError(w, r, "/short", err)
		return
	}
	if host != "" {
		s.maybeAutoApply(r.Context())
	}
	s.audit(r, "short.create", code)
	redirectFlash(w, r, "/short", "Short link created")
}

func (s *Server) handleShortLinkDelete(w http.ResponseWriter, r *http.Request) {
	id, err := formID(r, "id")
	if err != nil {
		redirectError(w, r, "/short", err)
		return
	}
	links, _ := s.store.ShortLinks()
	var hadHost bool
	for _, l := range links {
		if l.ID == id && l.Host != "" {
			hadHost = true
			break
		}
	}
	if err := s.store.DeleteShortLink(id); err != nil {
		redirectError(w, r, "/short", err)
		return
	}
	if hadHost {
		s.maybeAutoApply(r.Context())
	}
	s.audit(r, "short.delete", strconv.FormatInt(id, 10))
	redirectFlash(w, r, "/short", "Short link removed")
}

func (s *Server) handleShortRedirect(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	link, err := s.store.ShortLinkByCode("", code)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = s.store.IncrementShortLinkClicks(link.ID)
	http.Redirect(w, r, link.TargetURL, http.StatusFound)
}

func (s *Server) handleWebhooksPage(w http.ResponseWriter, r *http.Request) {
	outbound, _ := s.store.WebhookSubscriptions()
	inbound, _ := s.store.InboundWebhooks()
	s.render(w, r, "webhooks.html", "Webhooks", "webhooks", webhooksData{
		Outbound: outbound,
		Inbound:  inbound,
		BaseURL:  requestBaseURL(r),
	})
}

func (s *Server) handleWebhookOutboundCreate(w http.ResponseWriter, r *http.Request) {
	secret := r.FormValue("secret")
	if secret == "" {
		var err error
		secret, err = shortener.RandomCode(24)
		if err != nil {
			redirectError(w, r, "/webhooks", err)
			return
		}
	}
	events := r.FormValue("events")
	if events == "" {
		events = "*"
	}
	if _, err := s.store.CreateWebhookSubscription(r.FormValue("name"), r.FormValue("url"), secret, events); err != nil {
		redirectError(w, r, "/webhooks", err)
		return
	}
	s.audit(r, "webhooks.outbound.create", r.FormValue("name"))
	redirectFlash(w, r, "/webhooks", "Outbound webhook added")
}

func (s *Server) handleWebhookOutboundDelete(w http.ResponseWriter, r *http.Request) {
	id, err := formID(r, "id")
	if err != nil {
		redirectError(w, r, "/webhooks", err)
		return
	}
	if err := s.store.DeleteWebhookSubscription(id); err != nil {
		redirectError(w, r, "/webhooks", err)
		return
	}
	s.audit(r, "webhooks.outbound.delete", strconv.FormatInt(id, 10))
	redirectFlash(w, r, "/webhooks", "Outbound webhook removed")
}

func (s *Server) handleWebhookInboundCreate(w http.ResponseWriter, r *http.Request) {
	token := r.FormValue("token")
	if token == "" {
		var err error
		token, err = shortener.RandomCode(24)
		if err != nil {
			redirectError(w, r, "/webhooks", err)
			return
		}
	}
	action := r.FormValue("action")
	if action == "" {
		action = "log"
	}
	cfg := map[string]string{}
	if action == "run_function" {
		cfg["function_name"] = r.FormValue("function_name")
		cfg["token"] = r.FormValue("function_token")
	}
	raw, _ := json.Marshal(cfg)
	if _, err := s.store.CreateInboundWebhook(r.FormValue("name"), token, action, string(raw)); err != nil {
		redirectError(w, r, "/webhooks", err)
		return
	}
	s.audit(r, "webhooks.inbound.create", r.FormValue("name"))
	redirectFlash(w, r, "/webhooks", "Inbound webhook added")
}

func (s *Server) handleWebhookInboundDelete(w http.ResponseWriter, r *http.Request) {
	id, err := formID(r, "id")
	if err != nil {
		redirectError(w, r, "/webhooks", err)
		return
	}
	if err := s.store.DeleteInboundWebhook(id); err != nil {
		redirectError(w, r, "/webhooks", err)
		return
	}
	s.audit(r, "webhooks.inbound.delete", strconv.FormatInt(id, 10))
	redirectFlash(w, r, "/webhooks", "Inbound webhook removed")
}

func (s *Server) handleInboundWebhook(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	hook, err := s.store.InboundWebhookByToken(token)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if !hook.Enabled {
		http.Error(w, "disabled", http.StatusForbidden)
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	switch hook.Action {
	case "apply_caddy":
		if s.orch != nil {
			_ = s.orch.ApplyCaddy(r.Context())
		}
	case "run_function":
		if s.fns == nil {
			http.Error(w, "functions disabled", http.StatusServiceUnavailable)
			return
		}
		var cfg struct {
			FunctionName string `json:"function_name"`
			Token        string `json:"token"`
		}
		_ = json.Unmarshal([]byte(hook.ConfigJSON), &cfg)
		if cfg.FunctionName == "" {
			http.Error(w, "function not configured", http.StatusBadRequest)
			return
		}
		res, err := s.fns.Invoke(r.Context(), cfg.FunctionName, cfg.Token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "exit=%d\n%s", res.ExitCode, res.Output)
		return
	default:
		if s.log != nil {
			s.log.Info("inbound webhook", "name", hook.Name, "bytes", len(body))
		}
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("ok"))
}
