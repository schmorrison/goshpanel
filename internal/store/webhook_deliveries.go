package store

import (
	"database/sql"
	"time"
)

// WebhookDelivery is one outbound webhook attempt.
type WebhookDelivery struct {
	ID             int64     `json:"id"`
	SubscriptionID int64     `json:"subscription_id"`
	Event          string    `json:"event"`
	Status         int       `json:"status"`
	Error          string    `json:"error,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// AppendWebhookDelivery records a delivery attempt.
func (s *Store) AppendWebhookDelivery(subID int64, event string, status int, errMsg string) error {
	_, err := s.db.Exec(`INSERT INTO webhook_deliveries (subscription_id, event, status, error, created_at) VALUES (?, ?, ?, ?, ?)`,
		subID, event, status, errMsg, now())
	if err != nil {
		return err
	}
	_, _ = s.db.Exec(`DELETE FROM webhook_deliveries WHERE subscription_id = ? AND id NOT IN (
		SELECT id FROM webhook_deliveries WHERE subscription_id = ? ORDER BY created_at DESC LIMIT 200
	)`, subID, subID)
	return nil
}

// WebhookDeliveries returns recent deliveries for one subscription (or all when subID=0).
func (s *Store) WebhookDeliveries(subID int64, limit int) ([]WebhookDelivery, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows *sql.Rows
	var err error
	if subID > 0 {
		rows, err = s.db.Query(`SELECT id, subscription_id, event, status, error, created_at
			FROM webhook_deliveries WHERE subscription_id = ? ORDER BY created_at DESC LIMIT ?`, subID, limit)
	} else {
		rows, err = s.db.Query(`SELECT id, subscription_id, event, status, error, created_at
			FROM webhook_deliveries ORDER BY created_at DESC LIMIT ?`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WebhookDelivery
	for rows.Next() {
		var d WebhookDelivery
		if err := rows.Scan(&d.ID, &d.SubscriptionID, &d.Event, &d.Status, &d.Error, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
