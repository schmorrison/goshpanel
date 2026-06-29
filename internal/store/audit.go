package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// AuditEvent records a user action against a filesystem path.
type AuditEvent struct {
	ID         int64
	UserID     int64
	Username   string
	Action     string
	TargetPath string
	CreatedAt  time.Time
}

// AuditRepository persists audit log entries.
type AuditRepository struct {
	db *sql.DB
}

// NewAuditRepository returns an audit repository backed by db.
func NewAuditRepository(db *sql.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

// Record inserts a new audit event.
func (r *AuditRepository) Record(ctx context.Context, userID int64, username, action, targetPath string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO audit_events (user_id, username, action, target_path)
		VALUES (?, ?, ?, ?)
	`, userID, username, action, targetPath)
	if err != nil {
		return fmt.Errorf("record audit event: %w", err)
	}
	return nil
}

// Recent returns the latest audit events up to limit.
func (r *AuditRepository) Recent(ctx context.Context, limit int) ([]AuditEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, username, action, target_path, created_at
		FROM audit_events
		ORDER BY id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list audit events: %w", err)
	}
	defer rows.Close()

	var events []AuditEvent
	for rows.Next() {
		var event AuditEvent
		var createdAt string
		if err := rows.Scan(&event.ID, &event.UserID, &event.Username, &event.Action, &event.TargetPath, &createdAt); err != nil {
			return nil, fmt.Errorf("scan audit event: %w", err)
		}
		parsed, err := time.Parse("2006-01-02 15:04:05", createdAt)
		if err != nil {
			parsed, err = time.Parse(time.RFC3339, createdAt)
			if err != nil {
				return nil, fmt.Errorf("parse audit timestamp: %w", err)
			}
		}
		event.CreatedAt = parsed
		events = append(events, event)
	}

	return events, rows.Err()
}
