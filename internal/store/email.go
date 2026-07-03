package store

import (
	"fmt"
	"time"
)

// Mailbox is an email account definition.
type Mailbox struct {
	ID           int64
	Address      string
	PasswordHash string
	QuotaMB      int
	CreatedAt    time.Time
}

// EmailForwarder redirects mail from one address to another.
type EmailForwarder struct {
	ID   int64
	From string
	To   string
}

// CreateMailbox inserts a mailbox.
func (s *Store) CreateMailbox(address, passwordHash string, quotaMB int) (Mailbox, error) {
	m := Mailbox{Address: address, PasswordHash: passwordHash, QuotaMB: quotaMB, CreatedAt: now()}
	res, err := s.db.Exec(
		`INSERT INTO mailboxes (address, password_hash, quota_mb, created_at) VALUES (?, ?, ?, ?)`,
		m.Address, m.PasswordHash, m.QuotaMB, m.CreatedAt)
	if err != nil {
		return Mailbox{}, fmt.Errorf("create mailbox: %w", err)
	}
	m.ID, _ = res.LastInsertId()
	return m, nil
}

// Mailboxes lists all mailboxes ordered by address.
func (s *Store) Mailboxes() ([]Mailbox, error) {
	rows, err := s.db.Query(`SELECT id, address, password_hash, quota_mb, created_at FROM mailboxes ORDER BY address`)
	if err != nil {
		return nil, fmt.Errorf("list mailboxes: %w", err)
	}
	defer rows.Close()
	var out []Mailbox
	for rows.Next() {
		var m Mailbox
		if err := rows.Scan(&m.ID, &m.Address, &m.PasswordHash, &m.QuotaMB, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// DeleteMailbox removes a mailbox.
func (s *Store) DeleteMailbox(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM mailboxes WHERE id = ?`, id))
}

// CreateForwarder inserts an email forwarder.
func (s *Store) CreateForwarder(from, to string) (EmailForwarder, error) {
	f := EmailForwarder{From: from, To: to}
	res, err := s.db.Exec(`INSERT INTO forwarders (from_addr, to_addr) VALUES (?, ?)`, from, to)
	if err != nil {
		return EmailForwarder{}, fmt.Errorf("create forwarder: %w", err)
	}
	f.ID, _ = res.LastInsertId()
	return f, nil
}

// Forwarders lists all forwarders.
func (s *Store) Forwarders() ([]EmailForwarder, error) {
	rows, err := s.db.Query(`SELECT id, from_addr, to_addr FROM forwarders ORDER BY from_addr`)
	if err != nil {
		return nil, fmt.Errorf("list forwarders: %w", err)
	}
	defer rows.Close()
	var out []EmailForwarder
	for rows.Next() {
		var f EmailForwarder
		if err := rows.Scan(&f.ID, &f.From, &f.To); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// DeleteForwarder removes a forwarder.
func (s *Store) DeleteForwarder(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM forwarders WHERE id = ?`, id))
}
