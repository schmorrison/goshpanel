package store

import (
	"fmt"
	"time"
)

// AuditEntry records a panel action for the security log.
type AuditEntry struct {
	ID        int64
	Username  string
	Action    string
	Detail    string
	CreatedAt time.Time
}

// IPRule is a deny rule for the panel-level IP blocker.
type IPRule struct {
	ID        int64
	CIDR      string
	Comment   string
	CreatedAt time.Time
}

// AppendAudit records an action in the audit log.
func (s *Store) AppendAudit(username, action, detail string) error {
	_, err := s.db.Exec(
		`INSERT INTO audit_log (username, action, detail, created_at) VALUES (?, ?, ?, ?)`,
		username, action, detail, now())
	if err != nil {
		return fmt.Errorf("append audit: %w", err)
	}
	return nil
}

// AuditEntries returns the most recent limit entries, newest first.
func (s *Store) AuditEntries(limit int) ([]AuditEntry, error) {
	rows, err := s.db.Query(
		`SELECT id, username, action, detail, created_at FROM audit_log ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list audit: %w", err)
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.Username, &e.Action, &e.Detail, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// CreateIPRule inserts a deny rule.
func (s *Store) CreateIPRule(cidr, comment string) (IPRule, error) {
	r := IPRule{CIDR: cidr, Comment: comment, CreatedAt: now()}
	res, err := s.db.Exec(
		`INSERT INTO ip_rules (cidr, comment, created_at) VALUES (?, ?, ?)`, r.CIDR, r.Comment, r.CreatedAt)
	if err != nil {
		return IPRule{}, fmt.Errorf("create ip rule: %w", err)
	}
	r.ID, _ = res.LastInsertId()
	return r, nil
}

// IPRules lists all deny rules.
func (s *Store) IPRules() ([]IPRule, error) {
	rows, err := s.db.Query(`SELECT id, cidr, comment, created_at FROM ip_rules ORDER BY cidr`)
	if err != nil {
		return nil, fmt.Errorf("list ip rules: %w", err)
	}
	defer rows.Close()
	var out []IPRule
	for rows.Next() {
		var r IPRule
		if err := rows.Scan(&r.ID, &r.CIDR, &r.Comment, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteIPRule removes a deny rule.
func (s *Store) DeleteIPRule(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM ip_rules WHERE id = ?`, id))
}
