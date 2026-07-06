package store

import (
	"database/sql"
	"errors"
	"time"
)

// ShortLink is a shortened URL (panel /s/{code} or custom host).
type ShortLink struct {
	ID        int64
	Code      string
	TargetURL string
	Host      string
	Clicks    int64
	CreatedAt time.Time
}

func (s *Store) ShortLinks() ([]ShortLink, error) {
	rows, err := s.db.Query(`SELECT id, code, target_url, host, clicks, created_at FROM short_links ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ShortLink
	for rows.Next() {
		var l ShortLink
		if err := rows.Scan(&l.ID, &l.Code, &l.TargetURL, &l.Host, &l.Clicks, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) ShortLinkByCode(host, code string) (ShortLink, error) {
	var l ShortLink
	err := s.db.QueryRow(`SELECT id, code, target_url, host, clicks, created_at FROM short_links WHERE host = ? AND code = ?`,
		host, code).Scan(&l.ID, &l.Code, &l.TargetURL, &l.Host, &l.Clicks, &l.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ShortLink{}, ErrNotFound
	}
	return l, err
}

func (s *Store) CreateShortLink(code, targetURL, host string) (ShortLink, error) {
	res, err := s.db.Exec(`INSERT INTO short_links (code, target_url, host, created_at) VALUES (?, ?, ?, ?)`,
		code, targetURL, host, now())
	if err != nil {
		return ShortLink{}, err
	}
	id, _ := res.LastInsertId()
	return ShortLink{ID: id, Code: code, TargetURL: targetURL, Host: host, CreatedAt: now()}, nil
}

func (s *Store) DeleteShortLink(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM short_links WHERE id = ?`, id))
}

func (s *Store) UpdateShortLink(id int64, targetURL, host string) error {
	return s.mustAffect(s.db.Exec(`UPDATE short_links SET target_url = ?, host = ? WHERE id = ?`, targetURL, host, id))
}

func (s *Store) ShortLinkByID(id int64) (ShortLink, error) {
	var l ShortLink
	err := s.db.QueryRow(`SELECT id, code, target_url, host, clicks, created_at FROM short_links WHERE id = ?`, id).
		Scan(&l.ID, &l.Code, &l.TargetURL, &l.Host, &l.Clicks, &l.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ShortLink{}, ErrNotFound
	}
	return l, err
}

func (s *Store) IncrementShortLinkClicks(id int64) error {
	_, err := s.db.Exec(`UPDATE short_links SET clicks = clicks + 1 WHERE id = ?`, id)
	return err
}

// WebhookSubscription is an outbound webhook target.
type WebhookSubscription struct {
	ID         int64
	Name       string
	URL        string
	Secret     string
	Events     string
	Enabled    bool
	LastStatus int
	LastError  string
	LastAt     *time.Time
	CreatedAt  time.Time
}

func (s *Store) WebhookSubscriptions() ([]WebhookSubscription, error) {
	rows, err := s.db.Query(`SELECT id, name, url, secret, events, enabled, last_status, last_error, last_at, created_at
		FROM webhook_subscriptions ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WebhookSubscription
	for rows.Next() {
		w, err := scanWebhookSubscription(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) CreateWebhookSubscription(name, url, secret, events string) (WebhookSubscription, error) {
	res, err := s.db.Exec(`INSERT INTO webhook_subscriptions (name, url, secret, events, created_at) VALUES (?, ?, ?, ?, ?)`,
		name, url, secret, events, now())
	if err != nil {
		return WebhookSubscription{}, err
	}
	id, _ := res.LastInsertId()
	return s.WebhookSubscriptionByID(id)
}

func (s *Store) WebhookSubscriptionByID(id int64) (WebhookSubscription, error) {
	row := s.db.QueryRow(`SELECT id, name, url, secret, events, enabled, last_status, last_error, last_at, created_at
		FROM webhook_subscriptions WHERE id = ?`, id)
	return scanWebhookSubscription(row)
}

func (s *Store) DeleteWebhookSubscription(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM webhook_subscriptions WHERE id = ?`, id))
}

func (s *Store) TouchWebhookSubscription(id int64, status int, errMsg string) error {
	_, err := s.db.Exec(`UPDATE webhook_subscriptions SET last_status = ?, last_error = ?, last_at = ? WHERE id = ?`,
		status, errMsg, now(), id)
	return err
}

func scanWebhookSubscription(row scScanner) (WebhookSubscription, error) {
	var w WebhookSubscription
	var enabled int
	var last sql.NullTime
	if err := row.Scan(&w.ID, &w.Name, &w.URL, &w.Secret, &w.Events, &enabled, &w.LastStatus, &w.LastError, &last, &w.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return WebhookSubscription{}, ErrNotFound
		}
		return WebhookSubscription{}, err
	}
	w.Enabled = enabled == 1
	if last.Valid {
		t := last.Time
		w.LastAt = &t
	}
	return w, nil
}

// InboundWebhook receives external HTTP callbacks.
type InboundWebhook struct {
	ID         int64
	Name       string
	Token      string
	Action     string
	ConfigJSON string
	Enabled    bool
	CreatedAt  time.Time
}

func (s *Store) InboundWebhooks() ([]InboundWebhook, error) {
	rows, err := s.db.Query(`SELECT id, name, token, action, config_json, enabled, created_at FROM inbound_webhooks ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []InboundWebhook
	for rows.Next() {
		var w InboundWebhook
		var enabled int
		if err := rows.Scan(&w.ID, &w.Name, &w.Token, &w.Action, &w.ConfigJSON, &enabled, &w.CreatedAt); err != nil {
			return nil, err
		}
		w.Enabled = enabled == 1
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) InboundWebhookByToken(token string) (InboundWebhook, error) {
	var w InboundWebhook
	var enabled int
	err := s.db.QueryRow(`SELECT id, name, token, action, config_json, enabled, created_at FROM inbound_webhooks WHERE token = ?`, token).
		Scan(&w.ID, &w.Name, &w.Token, &w.Action, &w.ConfigJSON, &enabled, &w.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return InboundWebhook{}, ErrNotFound
	}
	w.Enabled = enabled == 1
	return w, err
}

func (s *Store) CreateInboundWebhook(name, token, action, configJSON string) (InboundWebhook, error) {
	res, err := s.db.Exec(`INSERT INTO inbound_webhooks (name, token, action, config_json, created_at) VALUES (?, ?, ?, ?, ?)`,
		name, token, action, configJSON, now())
	if err != nil {
		return InboundWebhook{}, err
	}
	id, _ := res.LastInsertId()
	var w InboundWebhook
	var enabled int
	err = s.db.QueryRow(`SELECT id, name, token, action, config_json, enabled, created_at FROM inbound_webhooks WHERE id = ?`, id).
		Scan(&w.ID, &w.Name, &w.Token, &w.Action, &w.ConfigJSON, &enabled, &w.CreatedAt)
	w.Enabled = enabled == 1
	return w, err
}

func (s *Store) DeleteInboundWebhook(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM inbound_webhooks WHERE id = ?`, id))
}
