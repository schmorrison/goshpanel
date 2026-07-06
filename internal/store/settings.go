package store

import (
	"database/sql"
	"errors"
	"strings"
)

// WebmailSettings configures a Caddy reverse proxy to a webmail app.
type WebmailSettings struct {
	Host     string // e.g. webmail.example.com
	Upstream string // e.g. localhost:8080
	Enabled  bool
}

const (
	settingWebmailHost     = "webmail_host"
	settingWebmailUpstream = "webmail_upstream"
	settingWebmailEnabled  = "webmail_enabled"
)

// GetSetting returns a panel setting value.
func (s *Store) GetSetting(key string) (string, error) {
	var val string
	err := s.db.QueryRow(`SELECT value FROM panel_settings WHERE key = ?`, key).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return val, err
}

// SetSetting upserts a panel setting.
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO panel_settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// WebmailSettings loads webmail configuration.
func (s *Store) WebmailSettings() (WebmailSettings, error) {
	host, err := s.GetSetting(settingWebmailHost)
	if err != nil {
		return WebmailSettings{}, err
	}
	upstream, err := s.GetSetting(settingWebmailUpstream)
	if err != nil {
		return WebmailSettings{}, err
	}
	enabledStr, err := s.GetSetting(settingWebmailEnabled)
	if err != nil {
		return WebmailSettings{}, err
	}
	enabled := enabledStr == "1" || strings.EqualFold(enabledStr, "true")
	return WebmailSettings{Host: host, Upstream: upstream, Enabled: enabled}, nil
}

// SaveWebmailSettings persists webmail configuration.
func (s *Store) SaveWebmailSettings(ws WebmailSettings) error {
	if err := s.SetSetting(settingWebmailHost, strings.TrimSpace(ws.Host)); err != nil {
		return err
	}
	if err := s.SetSetting(settingWebmailUpstream, strings.TrimSpace(ws.Upstream)); err != nil {
		return err
	}
	val := "0"
	if ws.Enabled {
		val = "1"
	}
	return s.SetSetting(settingWebmailEnabled, val)
}
