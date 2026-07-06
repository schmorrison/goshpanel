package store

import (
	"database/sql"
	"errors"
	"time"
)

// APIToken is a bearer token for the JSON API.
type APIToken struct {
	ID        int64
	Name      string
	TokenHash string
	Role      string
	CreatedAt time.Time
	LastUsed  *time.Time
}

func (s *Store) CreateAPIToken(name, tokenHash, role string) (APIToken, error) {
	t := APIToken{Name: name, TokenHash: tokenHash, Role: role, CreatedAt: now()}
	res, err := s.db.Exec(`INSERT INTO api_tokens (name, token_hash, role, created_at) VALUES (?, ?, ?, ?)`,
		t.Name, t.TokenHash, t.Role, t.CreatedAt)
	if err != nil {
		return APIToken{}, err
	}
	t.ID, _ = res.LastInsertId()
	return t, nil
}

func (s *Store) APITokens() ([]APIToken, error) {
	rows, err := s.db.Query(`SELECT id, name, token_hash, role, created_at, last_used FROM api_tokens ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAPITokens(rows)
}

func (s *Store) APITokenByHash(hash string) (APIToken, error) {
	var t APIToken
	var last sql.NullTime
	err := s.db.QueryRow(`SELECT id, name, token_hash, role, created_at, last_used FROM api_tokens WHERE token_hash = ?`, hash).
		Scan(&t.ID, &t.Name, &t.TokenHash, &t.Role, &t.CreatedAt, &last)
	if errors.Is(err, sql.ErrNoRows) {
		return APIToken{}, ErrNotFound
	}
	if last.Valid {
		t.LastUsed = &last.Time
	}
	return t, err
}

func (s *Store) TouchAPIToken(id int64) error {
	_, err := s.db.Exec(`UPDATE api_tokens SET last_used = ? WHERE id = ?`, now(), id)
	return err
}

func (s *Store) DeleteAPIToken(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM api_tokens WHERE id = ?`, id))
}

func scanAPITokens(rows *sql.Rows) ([]APIToken, error) {
	var out []APIToken
	for rows.Next() {
		var t APIToken
		var last sql.NullTime
		if err := rows.Scan(&t.ID, &t.Name, &t.TokenHash, &t.Role, &t.CreatedAt, &last); err != nil {
			return nil, err
		}
		if last.Valid {
			t.LastUsed = &last.Time
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// RedirectRule is a Caddy redirect.
type RedirectRule struct {
	ID       int64
	FromHost string
	ToURL    string
	Status   int
	Enabled  bool
}

func (s *Store) CreateRedirectRule(fromHost, toURL string, status int) (RedirectRule, error) {
	if status <= 0 {
		status = 301
	}
	r := RedirectRule{FromHost: fromHost, ToURL: toURL, Status: status, Enabled: true}
	res, err := s.db.Exec(`INSERT INTO redirect_rules (from_host, to_url, status, enabled) VALUES (?, ?, ?, 1)`,
		r.FromHost, r.ToURL, r.Status)
	if err != nil {
		return RedirectRule{}, err
	}
	r.ID, _ = res.LastInsertId()
	return r, nil
}

func (s *Store) RedirectRules() ([]RedirectRule, error) {
	rows, err := s.db.Query(`SELECT id, from_host, to_url, status, enabled FROM redirect_rules ORDER BY from_host`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RedirectRule
	for rows.Next() {
		var r RedirectRule
		var en int
		if err := rows.Scan(&r.ID, &r.FromHost, &r.ToURL, &r.Status, &en); err != nil {
			return nil, err
		}
		r.Enabled = en == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DeleteRedirectRule(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM redirect_rules WHERE id = ?`, id))
}

// DomainAlias parks one hostname on another domain's site.
type DomainAlias struct {
	ID          int64
	AliasHost   string
	TargetHost  string
}

func (s *Store) CreateDomainAlias(aliasHost, targetHost string) (DomainAlias, error) {
	a := DomainAlias{AliasHost: aliasHost, TargetHost: targetHost}
	res, err := s.db.Exec(`INSERT INTO domain_aliases (alias_host, target_host) VALUES (?, ?)`, a.AliasHost, a.TargetHost)
	if err != nil {
		return DomainAlias{}, err
	}
	a.ID, _ = res.LastInsertId()
	return a, nil
}

func (s *Store) DomainAliases() ([]DomainAlias, error) {
	rows, err := s.db.Query(`SELECT id, alias_host, target_host FROM domain_aliases ORDER BY alias_host`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DomainAlias
	for rows.Next() {
		var a DomainAlias
		if err := rows.Scan(&a.ID, &a.AliasHost, &a.TargetHost); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) DeleteDomainAlias(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM domain_aliases WHERE id = ?`, id))
}

// DKIMKey stores signing keys per domain.
type DKIMKey struct {
	ID         int64
	DomainID   int64
	Selector   string
	PrivatePEM string
	PublicDNS  string
	CreatedAt  time.Time
}

func (s *Store) SaveDKIMKey(domainID int64, selector, privatePEM, publicDNS string) (DKIMKey, error) {
	k := DKIMKey{DomainID: domainID, Selector: selector, PrivatePEM: privatePEM, PublicDNS: publicDNS, CreatedAt: now()}
	res, err := s.db.Exec(`INSERT INTO dkim_keys (domain_id, selector, private_pem, public_dns, created_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(domain_id) DO UPDATE SET selector=excluded.selector, private_pem=excluded.private_pem, public_dns=excluded.public_dns`,
		k.DomainID, k.Selector, k.PrivatePEM, k.PublicDNS, k.CreatedAt)
	if err != nil {
		return DKIMKey{}, err
	}
	k.ID, _ = res.LastInsertId()
	return k, nil
}

func (s *Store) DKIMKeyByDomain(domainID int64) (DKIMKey, error) {
	var k DKIMKey
	err := s.db.QueryRow(`SELECT id, domain_id, selector, private_pem, public_dns, created_at FROM dkim_keys WHERE domain_id = ?`, domainID).
		Scan(&k.ID, &k.DomainID, &k.Selector, &k.PrivatePEM, &k.PublicDNS, &k.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return DKIMKey{}, ErrNotFound
	}
	return k, err
}

// HTTPCollection groups saved HTTP requests.
type HTTPCollection struct {
	ID   int64
	Name string
}

type HTTPRequest struct {
	ID           int64
	CollectionID int64
	Name         string
	Method       string
	URL          string
	Headers      string
	Body         string
	CreatedAt    time.Time
}

func (s *Store) CreateHTTPCollection(name string) (HTTPCollection, error) {
	res, err := s.db.Exec(`INSERT INTO http_collections (name) VALUES (?)`, name)
	if err != nil {
		return HTTPCollection{}, err
	}
	id, _ := res.LastInsertId()
	return HTTPCollection{ID: id, Name: name}, nil
}

func (s *Store) HTTPCollections() ([]HTTPCollection, error) {
	rows, err := s.db.Query(`SELECT id, name FROM http_collections ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HTTPCollection
	for rows.Next() {
		var c HTTPCollection
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) CreateHTTPRequest(collectionID int64, name, method, url, headers, body string) (HTTPRequest, error) {
	r := HTTPRequest{CollectionID: collectionID, Name: name, Method: method, URL: url, Headers: headers, Body: body, CreatedAt: now()}
	res, err := s.db.Exec(`INSERT INTO http_requests (collection_id, name, method, url, headers, body, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.CollectionID, r.Name, r.Method, r.URL, r.Headers, r.Body, r.CreatedAt)
	if err != nil {
		return HTTPRequest{}, err
	}
	r.ID, _ = res.LastInsertId()
	return r, nil
}

func (s *Store) HTTPRequests(collectionID int64) ([]HTTPRequest, error) {
	rows, err := s.db.Query(`SELECT id, collection_id, name, method, url, headers, body, created_at FROM http_requests WHERE collection_id = ? ORDER BY name`, collectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HTTPRequest
	for rows.Next() {
		var r HTTPRequest
		if err := rows.Scan(&r.ID, &r.CollectionID, &r.Name, &r.Method, &r.URL, &r.Headers, &r.Body, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) HTTPRequestByID(id int64) (HTTPRequest, error) {
	var r HTTPRequest
	err := s.db.QueryRow(`SELECT id, collection_id, name, method, url, headers, body, created_at FROM http_requests WHERE id = ?`, id).
		Scan(&r.ID, &r.CollectionID, &r.Name, &r.Method, &r.URL, &r.Headers, &r.Body, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return HTTPRequest{}, ErrNotFound
	}
	return r, err
}

func (s *Store) DeleteHTTPRequest(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM http_requests WHERE id = ?`, id))
}

// FirewallRule is an nftables port rule.
type FirewallRule struct {
	ID        int64
	Port      int
	Protocol  string
	Action    string
	Direction string
	Comment   string
	Enabled   bool
}

func (s *Store) CreateFirewallRule(port int, protocol, action, direction, comment string) (FirewallRule, error) {
	r := FirewallRule{Port: port, Protocol: protocol, Action: action, Direction: direction, Comment: comment, Enabled: true}
	res, err := s.db.Exec(`INSERT INTO firewall_rules (port, protocol, action, direction, comment, enabled) VALUES (?, ?, ?, ?, ?, 1)`,
		r.Port, r.Protocol, r.Action, r.Direction, r.Comment)
	if err != nil {
		return FirewallRule{}, err
	}
	r.ID, _ = res.LastInsertId()
	return r, nil
}

func (s *Store) FirewallRules() ([]FirewallRule, error) {
	rows, err := s.db.Query(`SELECT id, port, protocol, action, direction, comment, enabled FROM firewall_rules ORDER BY port`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FirewallRule
	for rows.Next() {
		var r FirewallRule
		var en int
		if err := rows.Scan(&r.ID, &r.Port, &r.Protocol, &r.Action, &r.Direction, &r.Comment, &en); err != nil {
			return nil, err
		}
		r.Enabled = en == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DeleteFirewallRule(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM firewall_rules WHERE id = ?`, id))
}

// SSHKey is an authorized public key for a panel user.
type SSHKey struct {
	ID        int64
	UserID    int64
	PublicKey string
	Comment   string
	CreatedAt time.Time
}

func (s *Store) CreateSSHKey(userID int64, publicKey, comment string) (SSHKey, error) {
	k := SSHKey{UserID: userID, PublicKey: publicKey, Comment: comment, CreatedAt: now()}
	res, err := s.db.Exec(`INSERT INTO ssh_keys (user_id, public_key, comment, created_at) VALUES (?, ?, ?, ?)`,
		k.UserID, k.PublicKey, k.Comment, k.CreatedAt)
	if err != nil {
		return SSHKey{}, err
	}
	k.ID, _ = res.LastInsertId()
	return k, nil
}

func (s *Store) SSHKeysByUser(userID int64) ([]SSHKey, error) {
	rows, err := s.db.Query(`SELECT id, user_id, public_key, comment, created_at FROM ssh_keys WHERE user_id = ? ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SSHKey
	for rows.Next() {
		var k SSHKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.PublicKey, &k.Comment, &k.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *Store) DeleteSSHKey(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM ssh_keys WHERE id = ?`, id))
}

// WAFSite enables Coraza on a hostname.
type WAFSite struct {
	ID         int64
	Host       string
	PolicyPath string
	Enabled    bool
}

func (s *Store) CreateWAFSite(host, policyPath string) (WAFSite, error) {
	w := WAFSite{Host: host, PolicyPath: policyPath, Enabled: true}
	res, err := s.db.Exec(`INSERT INTO waf_sites (host, policy_path, enabled) VALUES (?, ?, 1)`, w.Host, w.PolicyPath)
	if err != nil {
		return WAFSite{}, err
	}
	w.ID, _ = res.LastInsertId()
	return w, nil
}

func (s *Store) WAFSites() ([]WAFSite, error) {
	rows, err := s.db.Query(`SELECT id, host, policy_path, enabled FROM waf_sites ORDER BY host`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WAFSite
	for rows.Next() {
		var w WAFSite
		var en int
		if err := rows.Scan(&w.ID, &w.Host, &w.PolicyPath, &en); err != nil {
			return nil, err
		}
		w.Enabled = en == 1
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) DeleteWAFSite(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM waf_sites WHERE id = ?`, id))
}

// DDNSConfig is a dynamic DNS updater.
type DDNSConfig struct {
	ID         int64
	Hostname   string
	Provider   string
	UpdateURL  string
	Token      string
	Enabled    bool
	LastUpdate *time.Time
}

func (s *Store) CreateDDNSConfig(hostname, provider, updateURL, token string) (DDNSConfig, error) {
	c := DDNSConfig{Hostname: hostname, Provider: provider, UpdateURL: updateURL, Token: token, Enabled: true}
	res, err := s.db.Exec(`INSERT INTO ddns_configs (hostname, provider, update_url, token, enabled) VALUES (?, ?, ?, ?, 1)`,
		c.Hostname, c.Provider, c.UpdateURL, c.Token)
	if err != nil {
		return DDNSConfig{}, err
	}
	c.ID, _ = res.LastInsertId()
	return c, nil
}

func (s *Store) DDNSConfigs() ([]DDNSConfig, error) {
	rows, err := s.db.Query(`SELECT id, hostname, provider, update_url, token, enabled, last_update FROM ddns_configs ORDER BY hostname`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DDNSConfig
	for rows.Next() {
		var c DDNSConfig
		var en int
		var last sql.NullTime
		if err := rows.Scan(&c.ID, &c.Hostname, &c.Provider, &c.UpdateURL, &c.Token, &en, &last); err != nil {
			return nil, err
		}
		c.Enabled = en == 1
		if last.Valid {
			c.LastUpdate = &last.Time
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) DeleteDDNSConfig(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM ddns_configs WHERE id = ?`, id))
}

// HealthCheck is an HTTP uptime probe.
type HealthCheck struct {
	ID           int64
	Name         string
	URL          string
	Method       string
	ExpectStatus int
	TimeoutSec   int
	Enabled      bool
	LastStatus   string
	LastMessage  string
	LastChecked  *time.Time
}

func (s *Store) CreateHealthCheck(name, url, method string, expectStatus, timeoutSec int) (HealthCheck, error) {
	h := HealthCheck{Name: name, URL: url, Method: method, ExpectStatus: expectStatus, TimeoutSec: timeoutSec, Enabled: true}
	res, err := s.db.Exec(`INSERT INTO health_checks (name, url, method, expect_status, timeout_sec, enabled) VALUES (?, ?, ?, ?, ?, 1)`,
		h.Name, h.URL, h.Method, h.ExpectStatus, h.TimeoutSec)
	if err != nil {
		return HealthCheck{}, err
	}
	h.ID, _ = res.LastInsertId()
	return h, nil
}

func (s *Store) HealthChecks() ([]HealthCheck, error) {
	rows, err := s.db.Query(`SELECT id, name, url, method, expect_status, timeout_sec, enabled, last_status, last_message, last_checked FROM health_checks ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HealthCheck
	for rows.Next() {
		var h HealthCheck
		var en int
		var last sql.NullTime
		if err := rows.Scan(&h.ID, &h.Name, &h.URL, &h.Method, &h.ExpectStatus, &h.TimeoutSec, &en, &h.LastStatus, &h.LastMessage, &last); err != nil {
			return nil, err
		}
		h.Enabled = en == 1
		if last.Valid {
			h.LastChecked = &last.Time
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Store) UpdateHealthCheckResult(id int64, status, message string) error {
	_, err := s.db.Exec(`UPDATE health_checks SET last_status = ?, last_message = ?, last_checked = ? WHERE id = ?`,
		status, message, now(), id)
	return err
}

func (s *Store) DeleteHealthCheck(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM health_checks WHERE id = ?`, id))
}

// BackupSchedule triggers periodic backups.
type BackupSchedule struct {
	ID            int64
	Name          string
	IntervalHours int
	Enabled       bool
	LastRunAt     *time.Time
}

func (s *Store) CreateBackupSchedule(name string, intervalHours int) (BackupSchedule, error) {
	if intervalHours <= 0 {
		intervalHours = 24
	}
	b := BackupSchedule{Name: name, IntervalHours: intervalHours, Enabled: true}
	res, err := s.db.Exec(`INSERT INTO backup_schedules (name, interval_hours, enabled) VALUES (?, ?, 1)`, b.Name, b.IntervalHours)
	if err != nil {
		return BackupSchedule{}, err
	}
	b.ID, _ = res.LastInsertId()
	return b, nil
}

func (s *Store) BackupSchedules() ([]BackupSchedule, error) {
	rows, err := s.db.Query(`SELECT id, name, interval_hours, enabled, last_run_at FROM backup_schedules ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BackupSchedule
	for rows.Next() {
		var b BackupSchedule
		var en int
		var last sql.NullTime
		if err := rows.Scan(&b.ID, &b.Name, &b.IntervalHours, &en, &last); err != nil {
			return nil, err
		}
		b.Enabled = en == 1
		if last.Valid {
			b.LastRunAt = &last.Time
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) TouchBackupSchedule(id int64) error {
	_, err := s.db.Exec(`UPDATE backup_schedules SET last_run_at = ? WHERE id = ?`, now(), id)
	return err
}

func (s *Store) DeleteBackupSchedule(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM backup_schedules WHERE id = ?`, id))
}

// EnvSecret is an encrypted key-value secret.
type EnvSecret struct {
	ID            int64
	Name          string
	EncryptedValue string
	CreatedAt     time.Time
}

func (s *Store) SaveEnvSecret(name, encrypted string) error {
	_, err := s.db.Exec(`INSERT INTO env_secrets (name, encrypted_value, created_at) VALUES (?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET encrypted_value = excluded.encrypted_value`, name, encrypted, now())
	return err
}

func (s *Store) EnvSecrets() ([]EnvSecret, error) {
	rows, err := s.db.Query(`SELECT id, name, encrypted_value, created_at FROM env_secrets ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EnvSecret
	for rows.Next() {
		var e EnvSecret
		if err := rows.Scan(&e.ID, &e.Name, &e.EncryptedValue, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) EnvSecretByName(name string) (EnvSecret, error) {
	var e EnvSecret
	err := s.db.QueryRow(`SELECT id, name, encrypted_value, created_at FROM env_secrets WHERE name = ?`, name).
		Scan(&e.ID, &e.Name, &e.EncryptedValue, &e.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return EnvSecret{}, ErrNotFound
	}
	return e, err
}

func (s *Store) DeleteEnvSecret(name string) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM env_secrets WHERE name = ?`, name))
}

// FunctionSchedule runs a micro-function on an interval.
type FunctionSchedule struct {
	ID              int64
	FunctionID      int64
	IntervalMinutes int
	Enabled         bool
	LastRunAt       *time.Time
}

func (s *Store) CreateFunctionSchedule(functionID int64, intervalMinutes int) (FunctionSchedule, error) {
	if intervalMinutes <= 0 {
		intervalMinutes = 60
	}
	f := FunctionSchedule{FunctionID: functionID, IntervalMinutes: intervalMinutes, Enabled: true}
	res, err := s.db.Exec(`INSERT INTO function_schedules (function_id, interval_minutes, enabled) VALUES (?, ?, 1)`,
		f.FunctionID, f.IntervalMinutes)
	if err != nil {
		return FunctionSchedule{}, err
	}
	f.ID, _ = res.LastInsertId()
	return f, nil
}

func (s *Store) FunctionSchedules() ([]FunctionSchedule, error) {
	rows, err := s.db.Query(`SELECT id, function_id, interval_minutes, enabled, last_run_at FROM function_schedules`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FunctionSchedule
	for rows.Next() {
		var f FunctionSchedule
		var en int
		var last sql.NullTime
		if err := rows.Scan(&f.ID, &f.FunctionID, &f.IntervalMinutes, &en, &last); err != nil {
			return nil, err
		}
		f.Enabled = en == 1
		if last.Valid {
			f.LastRunAt = &last.Time
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) TouchFunctionSchedule(id int64) error {
	_, err := s.db.Exec(`UPDATE function_schedules SET last_run_at = ? WHERE id = ?`, now(), id)
	return err
}

func (s *Store) DeleteFunctionSchedule(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM function_schedules WHERE id = ?`, id))
}

// GitRepo is a deploy hook target.
type GitRepo struct {
	ID            int64
	DomainID      int64
	Path          string
	Branch        string
	WebhookSecret string
	CreatedAt     time.Time
}

func (s *Store) CreateGitRepo(domainID int64, path, branch, webhookSecret string) (GitRepo, error) {
	g := GitRepo{DomainID: domainID, Path: path, Branch: branch, WebhookSecret: webhookSecret, CreatedAt: now()}
	res, err := s.db.Exec(`INSERT INTO git_repos (domain_id, path, branch, webhook_secret, created_at) VALUES (?, ?, ?, ?, ?)`,
		g.DomainID, g.Path, g.Branch, g.WebhookSecret, g.CreatedAt)
	if err != nil {
		return GitRepo{}, err
	}
	g.ID, _ = res.LastInsertId()
	return g, nil
}

func (s *Store) GitRepos() ([]GitRepo, error) {
	rows, err := s.db.Query(`SELECT id, domain_id, path, branch, webhook_secret, created_at FROM git_repos ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GitRepo
	for rows.Next() {
		var g GitRepo
		if err := rows.Scan(&g.ID, &g.DomainID, &g.Path, &g.Branch, &g.WebhookSecret, &g.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) GitRepoByID(id int64) (GitRepo, error) {
	var g GitRepo
	err := s.db.QueryRow(`SELECT id, domain_id, path, branch, webhook_secret, created_at FROM git_repos WHERE id = ?`, id).
		Scan(&g.ID, &g.DomainID, &g.Path, &g.Branch, &g.WebhookSecret, &g.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return GitRepo{}, ErrNotFound
	}
	return g, err
}

func (s *Store) DeleteGitRepo(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM git_repos WHERE id = ?`, id))
}

// Alert is a panel notification.
type Alert struct {
	ID        int64
	Kind      string
	Message   string
	CreatedAt time.Time
	Acked     bool
}

func (s *Store) CreateAlert(kind, message string) error {
	_, err := s.db.Exec(`INSERT INTO alerts (kind, message, created_at, acked) VALUES (?, ?, ?, 0)`, kind, message, now())
	return err
}

func (s *Store) RecentAlerts(limit int) ([]Alert, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT id, kind, message, created_at, acked FROM alerts ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Alert
	for rows.Next() {
		var a Alert
		var ack int
		if err := rows.Scan(&a.ID, &a.Kind, &a.Message, &a.CreatedAt, &ack); err != nil {
			return nil, err
		}
		a.Acked = ack == 1
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) AckAlert(id int64) error {
	return s.mustAffect(s.db.Exec(`UPDATE alerts SET acked = 1 WHERE id = ?`, id))
}

// FleetLogChunk is centralized log text from a worker.
type FleetLogChunk struct {
	ID         int64
	NodeID     int64
	Source     string
	Content    string
	RecordedAt time.Time
}

func (s *Store) AppendFleetLog(nodeID int64, source, content string) error {
	_, err := s.db.Exec(`INSERT INTO fleet_log_chunks (node_id, source, content, recorded_at) VALUES (?, ?, ?, ?)`,
		nodeID, source, content, now())
	return err
}

func (s *Store) FleetLogs(nodeID int64, limit int) ([]FleetLogChunk, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id, node_id, source, content, recorded_at FROM fleet_log_chunks WHERE node_id = ? ORDER BY id DESC LIMIT ?`, nodeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FleetLogChunk
	for rows.Next() {
		var c FleetLogChunk
		if err := rows.Scan(&c.ID, &c.NodeID, &c.Source, &c.Content, &c.RecordedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// AccessLogEventsSince returns recent access log rows.
func (s *Store) AccessLogEventsSince(since time.Time, limit int) ([]AccessLogEvent, error) {
	if limit <= 0 {
		limit = 10000
	}
	rows, err := s.db.Query(`SELECT id, recorded_at, host, method, path, status, bytes, remote_ip FROM access_log_events WHERE recorded_at >= ? ORDER BY id DESC LIMIT ?`,
		since.UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AccessLogEvent
	for rows.Next() {
		var e AccessLogEvent
		if err := rows.Scan(&e.ID, &e.RecordedAt, &e.Host, &e.Method, &e.Path, &e.Status, &e.Bytes, &e.RemoteIP); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// IncidentMode settings keys.
const (
	SettingIncidentMode = "incident_mode"
	SettingTheme        = "ui_theme"
	SettingAccent       = "ui_accent"
	SettingOnboarding   = "onboarding_done"
)

func (s *Store) IncidentMode() (bool, error) {
	v, err := s.GetSetting(SettingIncidentMode)
	return v == "1", err
}

func (s *Store) SetIncidentMode(on bool) error {
	val := "0"
	if on {
		val = "1"
	}
	return s.SetSetting(SettingIncidentMode, val)
}

func (s *Store) UITheme() (string, error) {
	v, err := s.GetSetting(SettingTheme)
	if err != nil || v == "" {
		return "dark", err
	}
	return v, nil
}

func (s *Store) SetUITheme(theme string) error {
	return s.SetSetting(SettingTheme, theme)
}

func (s *Store) UIAccent() (string, error) {
	return s.GetSetting(SettingAccent)
}

func (s *Store) SetUIAccent(color string) error {
	return s.SetSetting(SettingAccent, color)
}

func (s *Store) OnboardingDone() (bool, error) {
	v, err := s.GetSetting(SettingOnboarding)
	return v == "1", err
}

func (s *Store) SetOnboardingDone(done bool) error {
	val := "0"
	if done {
		val = "1"
	}
	return s.SetSetting(SettingOnboarding, val)
}

// DBPrivilegeGrant records a SQL grant to run.
type DBPrivilegeGrant struct {
	ID       int64
	ConnID   int64
	Username string
	Database string
	Privileges string
	CreatedAt time.Time
}

func (s *Store) CreateDBGrant(connID int64, username, database, privileges string) (DBPrivilegeGrant, error) {
	g := DBPrivilegeGrant{ConnID: connID, Username: username, Database: database, Privileges: privileges, CreatedAt: now()}
	res, err := s.db.Exec(`INSERT INTO db_grants (conn_id, username, database_name, privileges, created_at) VALUES (?, ?, ?, ?, ?)`,
		g.ConnID, g.Username, g.Database, g.Privileges, g.CreatedAt)
	if err != nil {
		return DBPrivilegeGrant{}, err
	}
	g.ID, _ = res.LastInsertId()
	return g, nil
}

func (s *Store) DBGrants(connID int64) ([]DBPrivilegeGrant, error) {
	rows, err := s.db.Query(`SELECT id, conn_id, username, database_name, privileges, created_at FROM db_grants WHERE conn_id = ? ORDER BY id DESC`, connID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DBPrivilegeGrant
	for rows.Next() {
		var g DBPrivilegeGrant
		if err := rows.Scan(&g.ID, &g.ConnID, &g.Username, &g.Database, &g.Privileges, &g.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) DeleteDBGrant(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM db_grants WHERE id = ?`, id))
}

// ConfigHash stores drift detection hashes per node.
func (s *Store) SetNodeConfigHash(nodeID int64, component, hash string) error {
	_, err := s.db.Exec(`INSERT INTO fleet_config_hashes (node_id, component, hash, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(node_id, component) DO UPDATE SET hash = excluded.hash, updated_at = excluded.updated_at`,
		nodeID, component, hash, now())
	return err
}

func (s *Store) NodeConfigHashes(nodeID int64) (map[string]string, error) {
	rows, err := s.db.Query(`SELECT component, hash FROM fleet_config_hashes WHERE node_id = ?`, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var comp, hash string
		if err := rows.Scan(&comp, &hash); err != nil {
			return nil, err
		}
		out[comp] = hash
	}
	return out, rows.Err()
}
