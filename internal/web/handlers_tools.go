package web

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/schmorrison/goshpanel/internal/bandwidth"
	"github.com/schmorrison/goshpanel/internal/dbmanager"
	"github.com/schmorrison/goshpanel/internal/dkim"
	"github.com/schmorrison/goshpanel/internal/domains"
	"github.com/schmorrison/goshpanel/internal/firewall"
	"github.com/schmorrison/goshpanel/internal/gitdeploy"
	"github.com/schmorrison/goshpanel/internal/httptool"
	"github.com/schmorrison/goshpanel/internal/selfupdate"
	"github.com/schmorrison/goshpanel/internal/sshkeys"
	"github.com/schmorrison/goshpanel/internal/store"
	"github.com/schmorrison/goshpanel/internal/waf"
)

// --- tools hub ---

func (s *Server) handleToolsPage(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	links := []struct{ Href, Label, Desc string }{
		{"/http", "HTTP Client", "Postman-style request builder"},
		{"/api", "API & Explorer", "Bearer tokens and route catalog"},
		{"/bandwidth", "Bandwidth", "Per-host traffic from access logs"},
		{"/redirects", "Redirects & Aliases", "Caddy redirect and parked domains"},
		{"/firewall", "Firewall", "nftables rules export"},
		{"/ssh", "SSH Keys", "authorized_keys management"},
		{"/health-checks", "Health Checks", "HTTP uptime probes"},
		{"/secrets", "Secrets", "Encrypted environment variables"},
		{"/git-deploy", "Git Deploy", "Webhook git pull hooks"},
		{"/ddns", "Dynamic DNS", "Auto-update DNS records"},
		{"/waf", "WAF", "Coraza config per site"},
		{"/theme", "Appearance", "Theme and accent color"},
		{"/onboarding", "Onboarding", "First-run setup wizard"},
		{"/jump", "Jump", "Quick navigation palette"},
	}
	var filtered []struct{ Href, Label, Desc string }
	for _, l := range links {
		if q == "" || strings.Contains(strings.ToLower(l.Label+" "+l.Desc), q) {
			filtered = append(filtered, l)
		}
	}
	s.render(w, r, "tools.html", "Tools", "tools", map[string]any{"Links": filtered, "Query": q})
}

func (s *Server) handleJumpPage(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/tools?q="+r.URL.Query().Get("q"), http.StatusSeeOther)
}

// --- HTTP client (Postman clone) ---

func (s *Server) handleHTTPPage(w http.ResponseWriter, r *http.Request) {
	collections, _ := s.store.HTTPCollections()
	var reqs []store.HTTPRequest
	if cid, err := strconv.ParseInt(r.URL.Query().Get("collection"), 10, 64); err == nil {
		reqs, _ = s.store.HTTPRequests(cid)
	}
	s.render(w, r, "http.html", "HTTP Client", "http", map[string]any{
		"Collections": collections,
		"Requests":    reqs,
		"Collection":  r.URL.Query().Get("collection"),
		"Result":      r.URL.Query().Get("result"),
	})
}

func (s *Server) handleHTTPCollectionCreate(w http.ResponseWriter, r *http.Request) {
	if _, err := s.store.CreateHTTPCollection(r.FormValue("name")); err != nil {
		redirectError(w, r, "/http", err)
		return
	}
	redirectFlash(w, r, "/http", "Collection created")
}

func (s *Server) handleHTTPRequestSave(w http.ResponseWriter, r *http.Request) {
	cid, err := formID(r, "collection_id")
	if err != nil {
		redirectError(w, r, "/http", err)
		return
	}
	if _, err := s.store.CreateHTTPRequest(cid, r.FormValue("name"), r.FormValue("method"), r.FormValue("url"), r.FormValue("headers"), r.FormValue("body")); err != nil {
		redirectError(w, r, "/http", err)
		return
	}
	redirectFlash(w, r, "/http?collection="+strconv.FormatInt(cid, 10), "Request saved")
}

func (s *Server) handleHTTPRequestRun(w http.ResponseWriter, r *http.Request) {
	res := httptool.Execute(r.Context(), httptool.Request{
		Method:  r.FormValue("method"),
		URL:     r.FormValue("url"),
		Headers: httptool.ParseHeaders(r.FormValue("headers")),
		Body:    r.FormValue("body"),
	})
	msg := fmt.Sprintf("HTTP %d in %s", res.StatusCode, res.Duration)
	if res.Error != "" {
		msg = res.Error
	}
	redirectFlash(w, r, "/http?result="+msg, res.Body[:min(len(res.Body), 200)])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// --- bandwidth ---

func (s *Server) handleBandwidthPage(w http.ResponseWriter, r *http.Request) {
	since := time.Now().UTC().Add(-24 * time.Hour)
	events, _ := s.store.AccessLogEventsSince(since, 50000)
	s.render(w, r, "bandwidth.html", "Bandwidth", "bandwidth", map[string]any{
		"Hosts":   bandwidth.TopHosts(events, 20),
		"Hourly":  bandwidth.HourlyBuckets(events, 24),
		"Summary": len(events),
	})
}

// --- redirects & aliases ---

func (s *Server) handleRedirectsPage(w http.ResponseWriter, r *http.Request) {
	redirects, _ := s.store.RedirectRules()
	aliases, _ := s.store.DomainAliases()
	s.render(w, r, "redirects.html", "Redirects & Aliases", "redirects", map[string]any{
		"Redirects": redirects, "Aliases": aliases,
	})
}

func (s *Server) handleRedirectCreate(w http.ResponseWriter, r *http.Request) {
	status, _ := strconv.Atoi(r.FormValue("status"))
	if _, err := s.store.CreateRedirectRule(r.FormValue("from_host"), r.FormValue("to_url"), status); err != nil {
		redirectError(w, r, "/redirects", err)
		return
	}
	s.maybeAutoApply(r.Context())
	redirectFlash(w, r, "/redirects", "Redirect added")
}

func (s *Server) handleRedirectDelete(w http.ResponseWriter, r *http.Request) {
	id, _ := formID(r, "id")
	_ = s.store.DeleteRedirectRule(id)
	s.maybeAutoApply(r.Context())
	redirectFlash(w, r, "/redirects", "Redirect removed")
}

func (s *Server) handleAliasCreate(w http.ResponseWriter, r *http.Request) {
	if _, err := s.store.CreateDomainAlias(r.FormValue("alias_host"), r.FormValue("target_host")); err != nil {
		redirectError(w, r, "/redirects", err)
		return
	}
	s.maybeAutoApply(r.Context())
	redirectFlash(w, r, "/redirects", "Alias added")
}

func (s *Server) handleAliasDelete(w http.ResponseWriter, r *http.Request) {
	id, _ := formID(r, "id")
	_ = s.store.DeleteDomainAlias(id)
	s.maybeAutoApply(r.Context())
	redirectFlash(w, r, "/redirects", "Alias removed")
}

// --- DKIM ---

func (s *Server) handleDKIMGenerate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		redirectError(w, r, "/domains", err)
		return
	}
	d, err := s.store.DomainByID(id)
	if err != nil {
		redirectError(w, r, "/domains", err)
		return
	}
	kp, err := dkim.Generate(r.FormValue("selector"), d.Name)
	if err != nil {
		redirectError(w, r, "/domains", err)
		return
	}
	if _, err := s.store.SaveDKIMKey(id, kp.Selector, kp.PrivatePEM, kp.PublicDNS); err != nil {
		redirectError(w, r, "/domains", err)
		return
	}
	if _, err := s.store.CreateDNSRecord(store.DNSRecord{
		DomainID: id, Type: "TXT", Name: kp.Selector + "._domainkey", Value: kp.PublicDNS, TTL: 3600,
	}); err != nil {
		redirectError(w, r, "/domains", err)
		return
	}
	s.maybeAutoApply(r.Context())
	redirectFlash(w, r, "/domains", "DKIM key generated for "+d.Name)
}

// --- firewall ---

func (s *Server) handleFirewallPage(w http.ResponseWriter, r *http.Request) {
	rules, _ := s.store.FirewallRules()
	s.render(w, r, "firewall.html", "Firewall", "firewall", map[string]any{
		"Rules": rules, "Config": firewall.RenderNftables(rules),
	})
}

func (s *Server) handleFirewallCreate(w http.ResponseWriter, r *http.Request) {
	port, _ := strconv.Atoi(r.FormValue("port"))
	if _, err := s.store.CreateFirewallRule(port, r.FormValue("protocol"), r.FormValue("action"), r.FormValue("direction"), r.FormValue("comment")); err != nil {
		redirectError(w, r, "/firewall", err)
		return
	}
	redirectFlash(w, r, "/firewall", "Rule added")
}

func (s *Server) handleFirewallDelete(w http.ResponseWriter, r *http.Request) {
	id, _ := formID(r, "id")
	_ = s.store.DeleteFirewallRule(id)
	redirectFlash(w, r, "/firewall", "Rule removed")
}

func (s *Server) handleFirewallDownload(w http.ResponseWriter, r *http.Request) {
	rules, _ := s.store.FirewallRules()
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition", `attachment; filename="goshpanel.nft"`)
	w.Write([]byte(firewall.RenderNftables(rules)))
}

// --- SSH keys ---

func (s *Server) handleSSHPage(w http.ResponseWriter, r *http.Request) {
	users, _ := s.store.Users()
	type row struct {
		User store.User
		Keys []store.SSHKey
	}
	var rows []row
	for _, u := range users {
		keys, _ := s.store.SSHKeysByUser(u.ID)
		rows = append(rows, row{User: u, Keys: keys})
	}
	s.render(w, r, "ssh.html", "SSH Keys", "ssh", map[string]any{"Rows": rows, "SSHDir": s.cfg.DataDir})
}

func (s *Server) handleSSHKeyCreate(w http.ResponseWriter, r *http.Request) {
	uid, err := formID(r, "user_id")
	if err != nil {
		redirectError(w, r, "/ssh", err)
		return
	}
	if _, err := s.store.CreateSSHKey(uid, r.FormValue("public_key"), r.FormValue("comment")); err != nil {
		redirectError(w, r, "/ssh", err)
		return
	}
	u, _ := s.store.UserByID(uid)
	keys, _ := s.store.SSHKeysByUser(uid)
	_ = sshkeys.Apply(filepath.Join(s.cfg.DataDir, "ssh"), u.Username, keys)
	redirectFlash(w, r, "/ssh", "SSH key added")
}

func (s *Server) handleSSHKeyDelete(w http.ResponseWriter, r *http.Request) {
	id, _ := formID(r, "id")
	_ = s.store.DeleteSSHKey(id)
	redirectFlash(w, r, "/ssh", "Key removed")
}

// --- health checks ---

func (s *Server) handleHealthChecksPage(w http.ResponseWriter, r *http.Request) {
	checks, _ := s.store.HealthChecks()
	alerts, _ := s.store.RecentAlerts(10)
	s.render(w, r, "health_checks.html", "Health Checks", "health", map[string]any{
		"Checks": checks, "Alerts": alerts,
	})
}

func (s *Server) handleHealthCheckCreate(w http.ResponseWriter, r *http.Request) {
	status, _ := strconv.Atoi(r.FormValue("expect_status"))
	timeout, _ := strconv.Atoi(r.FormValue("timeout_sec"))
	if _, err := s.store.CreateHealthCheck(r.FormValue("name"), r.FormValue("url"), r.FormValue("method"), status, timeout); err != nil {
		redirectError(w, r, "/health-checks", err)
		return
	}
	redirectFlash(w, r, "/health-checks", "Health check added")
}

func (s *Server) handleHealthCheckDelete(w http.ResponseWriter, r *http.Request) {
	id, _ := formID(r, "id")
	_ = s.store.DeleteHealthCheck(id)
	redirectFlash(w, r, "/health-checks", "Check removed")
}

// --- secrets ---

func (s *Server) handleSecretsPage(w http.ResponseWriter, r *http.Request) {
	secrets, _ := s.store.EnvSecrets()
	s.render(w, r, "secrets.html", "Secrets", "secrets", map[string]any{
		"Secrets": secrets, "VaultEnabled": s.vault != nil,
	})
}

func (s *Server) handleSecretSave(w http.ResponseWriter, r *http.Request) {
	if s.vault == nil {
		redirectError(w, r, "/secrets", fmt.Errorf("set GOSHPANEL_SECRETS_KEY to enable secrets"))
		return
	}
	enc, err := s.vault.Encrypt(r.FormValue("value"))
	if err != nil {
		redirectError(w, r, "/secrets", err)
		return
	}
	if err := s.store.SaveEnvSecret(r.FormValue("name"), enc); err != nil {
		redirectError(w, r, "/secrets", err)
		return
	}
	redirectFlash(w, r, "/secrets", "Secret saved")
}

func (s *Server) handleSecretDelete(w http.ResponseWriter, r *http.Request) {
	_ = s.store.DeleteEnvSecret(r.FormValue("name"))
	redirectFlash(w, r, "/secrets", "Secret deleted")
}

// --- git deploy ---

func (s *Server) handleGitDeployPage(w http.ResponseWriter, r *http.Request) {
	repos, _ := s.store.GitRepos()
	domains, _ := s.store.Domains()
	s.render(w, r, "git_deploy.html", "Git Deploy", "git", map[string]any{
		"Repos": repos, "Domains": domains,
	})
}

func (s *Server) handleGitRepoCreate(w http.ResponseWriter, r *http.Request) {
	did, _ := formID(r, "domain_id")
	if _, err := s.store.CreateGitRepo(did, r.FormValue("path"), r.FormValue("branch"), r.FormValue("webhook_secret")); err != nil {
		redirectError(w, r, "/git-deploy", err)
		return
	}
	redirectFlash(w, r, "/git-deploy", "Git repo registered")
}

func (s *Server) handleGitWebhook(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	repo, err := s.store.GitRepoByID(id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if repo.WebhookSecret != "" && r.Header.Get("X-Hub-Signature-256") == "" && r.URL.Query().Get("token") != repo.WebhookSecret {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	out, err := gitdeploy.Pull(r.Context(), repo.Path, repo.Branch)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if s.orch != nil {
		_ = s.orch.ApplyCaddy(r.Context())
	}
	fmt.Fprintf(w, "ok: %s", out)
}

// --- DDNS ---

func (s *Server) handleDDNSPage(w http.ResponseWriter, r *http.Request) {
	configs, _ := s.store.DDNSConfigs()
	s.render(w, r, "ddns.html", "Dynamic DNS", "ddns", map[string]any{"Configs": configs})
}

func (s *Server) handleDDNSCreate(w http.ResponseWriter, r *http.Request) {
	if _, err := s.store.CreateDDNSConfig(r.FormValue("hostname"), r.FormValue("provider"), r.FormValue("update_url"), r.FormValue("token")); err != nil {
		redirectError(w, r, "/ddns", err)
		return
	}
	redirectFlash(w, r, "/ddns", "DDNS config added")
}

func (s *Server) handleDDNSDelete(w http.ResponseWriter, r *http.Request) {
	id, _ := formID(r, "id")
	_ = s.store.DeleteDDNSConfig(id)
	redirectFlash(w, r, "/ddns", "Removed")
}

// --- WAF ---

func (s *Server) handleWAFPage(w http.ResponseWriter, r *http.Request) {
	sites, _ := s.store.WAFSites()
	s.render(w, r, "waf.html", "WAF", "waf", map[string]any{
		"Sites": sites, "Stub": waf.RenderCorazaConfig(),
	})
}

func (s *Server) handleWAFCreate(w http.ResponseWriter, r *http.Request) {
	if _, err := s.store.CreateWAFSite(r.FormValue("host"), r.FormValue("policy_path")); err != nil {
		redirectError(w, r, "/waf", err)
		return
	}
	s.maybeAutoApply(r.Context())
	redirectFlash(w, r, "/waf", "WAF site added")
}

func (s *Server) handleWAFDelete(w http.ResponseWriter, r *http.Request) {
	id, _ := formID(r, "id")
	_ = s.store.DeleteWAFSite(id)
	s.maybeAutoApply(r.Context())
	redirectFlash(w, r, "/waf", "Removed")
}

// --- theme ---

func (s *Server) handleThemePage(w http.ResponseWriter, r *http.Request) {
	theme, _ := s.store.UITheme()
	accent, _ := s.store.UIAccent()
	release, _ := selfupdate.CheckLatest(r.Context(), "")
	s.render(w, r, "theme.html", "Appearance", "theme", map[string]any{
		"Theme": theme, "Accent": accent, "Release": release,
	})
}

func (s *Server) handleThemeSave(w http.ResponseWriter, r *http.Request) {
	_ = s.store.SetUITheme(r.FormValue("theme"))
	_ = s.store.SetUIAccent(r.FormValue("accent"))
	redirectFlash(w, r, "/theme", "Appearance saved")
}

// --- onboarding ---

func (s *Server) handleOnboardingPage(w http.ResponseWriter, r *http.Request) {
	done, _ := s.store.OnboardingDone()
	s.render(w, r, "onboarding.html", "Onboarding", "onboarding", map[string]any{"Done": done})
}

func (s *Server) handleOnboardingComplete(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("domain") != "" {
		if err := domains.ValidateName(r.FormValue("domain")); err == nil {
			_, _ = s.store.CreateDomain(r.FormValue("domain"), r.FormValue("root"), r.FormValue("upstream"))
			s.maybeAutoApply(r.Context())
		}
	}
	_ = s.store.SetOnboardingDone(true)
	redirectFlash(w, r, "/", "Welcome to GoshPanel!")
}

// --- backup schedules ---

func (s *Server) handleBackupScheduleCreate(w http.ResponseWriter, r *http.Request) {
	hours, _ := strconv.Atoi(r.FormValue("interval_hours"))
	if _, err := s.store.CreateBackupSchedule(r.FormValue("name"), hours); err != nil {
		redirectError(w, r, "/backups", err)
		return
	}
	redirectFlash(w, r, "/backups", "Backup schedule added")
}

// --- DB grants ---

func (s *Server) handleDBGrantCreate(w http.ResponseWriter, r *http.Request) {
	cid, _ := formID(r, "conn_id")
	conn, err := s.store.DatabaseConnByID(cid)
	if err != nil {
		redirectError(w, r, "/databases", err)
		return
	}
	if err := dbmanager.ApplyGrant(r.Context(), conn.Driver, conn.DSN, r.FormValue("username"), r.FormValue("database"), r.FormValue("privileges")); err != nil {
		redirectError(w, r, "/databases", err)
		return
	}
	_, _ = s.store.CreateDBGrant(cid, r.FormValue("username"), r.FormValue("database"), r.FormValue("privileges"))
	redirectFlash(w, r, "/databases", "Grant applied")
}

// --- function schedules ---

func (s *Server) handleFunctionScheduleCreate(w http.ResponseWriter, r *http.Request) {
	fid, _ := formID(r, "function_id")
	mins, _ := strconv.Atoi(r.FormValue("interval_minutes"))
	if _, err := s.store.CreateFunctionSchedule(fid, mins); err != nil {
		redirectError(w, r, "/functions", err)
		return
	}
	redirectFlash(w, r, "/functions", "Function schedule added")
}

// --- webdav info ---

func (s *Server) handleWebDAVPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "webdav.html", "WebDAV", "webdav", map[string]any{
		"Enabled": s.cfg.WebDAVEnabled, "Addr": s.cfg.WebDAVAddr, "Root": s.files.Root(),
	})
}

// --- AI log summary (calls optional local endpoint) ---

func (s *Server) handleLogSummary(w http.ResponseWriter, r *http.Request) {
	path := r.FormValue("path")
	if path == "" && len(s.cfg.LogSources) > 0 {
		path = s.cfg.LogSources[0]
	}
	lines, err := bandwidth.TailFile(path, 50)
	if err != nil {
		redirectError(w, r, "/logs", err)
		return
	}
	summary := "Last " + strconv.Itoa(len(lines)) + " lines from " + filepath.Base(path) + ". Configure a local LLM endpoint for full summarization."
	if len(lines) > 0 {
		summary += " Latest: " + lines[len(lines)-1]
	}
	redirectFlash(w, r, "/logs", summary)
}
