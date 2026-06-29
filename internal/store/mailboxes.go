package store

import (
	"context"
	"database/sql"
	"fmt"
)

// Mailbox stores a local mail account definition.
type Mailbox struct {
	ID                int64
	LocalPart         string
	Domain            string
	PasswordEncrypted string
	ForwardTo         string
	Enabled           bool
}

// MailboxRepository persists mailbox definitions.
type MailboxRepository struct {
	db *sql.DB
}

// NewMailboxRepository returns a mailbox repository backed by db.
func NewMailboxRepository(db *sql.DB) *MailboxRepository {
	return &MailboxRepository{db: db}
}

// List returns configured mailboxes.
func (r *MailboxRepository) List(ctx context.Context) ([]Mailbox, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, local_part, domain, password_encrypted, forward_to, enabled
		FROM mailboxes
		ORDER BY domain COLLATE NOCASE, local_part COLLATE NOCASE
	`)
	if err != nil {
		return nil, fmt.Errorf("list mailboxes: %w", err)
	}
	defer rows.Close()

	var mailboxes []Mailbox
	for rows.Next() {
		var mailbox Mailbox
		var enabled int
		if err := rows.Scan(&mailbox.ID, &mailbox.LocalPart, &mailbox.Domain, &mailbox.PasswordEncrypted, &mailbox.ForwardTo, &enabled); err != nil {
			return nil, fmt.Errorf("scan mailbox: %w", err)
		}
		mailbox.Enabled = enabled == 1
		mailboxes = append(mailboxes, mailbox)
	}
	return mailboxes, rows.Err()
}

// Create inserts a mailbox.
func (r *MailboxRepository) Create(ctx context.Context, mailbox Mailbox) (Mailbox, error) {
	enabled := 0
	if mailbox.Enabled {
		enabled = 1
	}
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO mailboxes (local_part, domain, password_encrypted, forward_to, enabled)
		VALUES (?, ?, ?, ?, ?)
	`, mailbox.LocalPart, mailbox.Domain, mailbox.PasswordEncrypted, mailbox.ForwardTo, enabled)
	if err != nil {
		return Mailbox{}, fmt.Errorf("create mailbox: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Mailbox{}, fmt.Errorf("last insert id: %w", err)
	}
	return r.FindByID(ctx, id)
}

// FindByID returns a mailbox by id.
func (r *MailboxRepository) FindByID(ctx context.Context, id int64) (Mailbox, error) {
	var mailbox Mailbox
	var enabled int
	err := r.db.QueryRowContext(ctx, `
		SELECT id, local_part, domain, password_encrypted, forward_to, enabled
		FROM mailboxes
		WHERE id = ?
	`, id).Scan(&mailbox.ID, &mailbox.LocalPart, &mailbox.Domain, &mailbox.PasswordEncrypted, &mailbox.ForwardTo, &enabled)
	if err != nil {
		return Mailbox{}, fmt.Errorf("find mailbox: %w", err)
	}
	mailbox.Enabled = enabled == 1
	return mailbox, nil
}

// Delete removes a mailbox by id.
func (r *MailboxRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM mailboxes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete mailbox: %w", err)
	}
	return nil
}
