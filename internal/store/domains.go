package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Domain is a hosted site: a document root plus optional upstream proxy.
type Domain struct {
	ID        int64
	Name      string
	Root      string // static document root
	Upstream  string // optional reverse-proxy target, e.g. localhost:3000
	CreatedAt time.Time
}

// DNSRecord is a zone record belonging to a domain.
type DNSRecord struct {
	ID       int64
	DomainID int64
	Type     string // A, AAAA, CNAME, MX, TXT, NS, SRV
	Name     string
	Value    string
	TTL      int
	Priority int // MX/SRV only
}

// CreateDomain inserts a domain.
func (s *Store) CreateDomain(name, root, upstream string) (Domain, error) {
	d := Domain{Name: name, Root: root, Upstream: upstream, CreatedAt: now()}
	res, err := s.db.Exec(
		`INSERT INTO domains (name, root, upstream, created_at) VALUES (?, ?, ?, ?)`,
		d.Name, d.Root, d.Upstream, d.CreatedAt)
	if err != nil {
		return Domain{}, fmt.Errorf("create domain: %w", err)
	}
	d.ID, _ = res.LastInsertId()
	return d, nil
}

// Domains lists all domains ordered by name.
func (s *Store) Domains() ([]Domain, error) {
	rows, err := s.db.Query(`SELECT id, name, root, upstream, created_at FROM domains ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list domains: %w", err)
	}
	defer rows.Close()
	var out []Domain
	for rows.Next() {
		var d Domain
		if err := rows.Scan(&d.ID, &d.Name, &d.Root, &d.Upstream, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DomainByID returns one domain.
func (s *Store) DomainByID(id int64) (Domain, error) {
	var d Domain
	err := s.db.QueryRow(`SELECT id, name, root, upstream, created_at FROM domains WHERE id = ?`, id).
		Scan(&d.ID, &d.Name, &d.Root, &d.Upstream, &d.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Domain{}, ErrNotFound
	}
	return d, err
}

// DeleteDomain removes a domain and (via FK cascade) its DNS records.
func (s *Store) DeleteDomain(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM domains WHERE id = ?`, id))
}

// CreateDNSRecord inserts a DNS record.
func (s *Store) CreateDNSRecord(r DNSRecord) (DNSRecord, error) {
	res, err := s.db.Exec(
		`INSERT INTO dns_records (domain_id, type, name, value, ttl, priority) VALUES (?, ?, ?, ?, ?, ?)`,
		r.DomainID, r.Type, r.Name, r.Value, r.TTL, r.Priority)
	if err != nil {
		return DNSRecord{}, fmt.Errorf("create dns record: %w", err)
	}
	r.ID, _ = res.LastInsertId()
	return r, nil
}

// DNSRecords lists records for a domain.
func (s *Store) DNSRecords(domainID int64) ([]DNSRecord, error) {
	rows, err := s.db.Query(
		`SELECT id, domain_id, type, name, value, ttl, priority FROM dns_records WHERE domain_id = ? ORDER BY type, name`,
		domainID)
	if err != nil {
		return nil, fmt.Errorf("list dns records: %w", err)
	}
	defer rows.Close()
	var out []DNSRecord
	for rows.Next() {
		var r DNSRecord
		if err := rows.Scan(&r.ID, &r.DomainID, &r.Type, &r.Name, &r.Value, &r.TTL, &r.Priority); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteDNSRecord removes a DNS record.
func (s *Store) DeleteDNSRecord(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM dns_records WHERE id = ?`, id))
}
