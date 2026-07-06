// Package webhooks delivers outbound event notifications and handles inbound hooks.
package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/schmorrison/goshpanel/internal/store"
)

// Dispatcher sends panel events to subscribed URLs.
type Dispatcher struct {
	store  *store.Store
	client *http.Client
	log    *slog.Logger
}

// NewDispatcher creates a webhook dispatcher.
func NewDispatcher(st *store.Store, log *slog.Logger) *Dispatcher {
	return &Dispatcher{
		store: st,
		client: &http.Client{
			Timeout: 12 * time.Second,
		},
		log: log,
	}
}

// Event is a webhook payload envelope.
type Event struct {
	Event     string         `json:"event"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data"`
}

// Dispatch notifies all matching subscriptions (async-safe).
func (d *Dispatcher) Dispatch(ctx context.Context, event string, data map[string]any) {
	if d == nil || d.store == nil {
		return
	}
	subs, err := d.store.WebhookSubscriptions()
	if err != nil {
		return
	}
	payload := Event{
		Event:     event,
		Timestamp: time.Now().UTC(),
		Data:      data,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	for _, sub := range subs {
		if !sub.Enabled || !matchesEvent(sub.Events, event) {
			continue
		}
		d.deliver(ctx, sub, body)
	}
}

func matchesEvent(spec, event string) bool {
	spec = strings.TrimSpace(spec)
	if spec == "" || spec == "*" {
		return true
	}
	for _, part := range strings.Split(spec, ",") {
		if strings.TrimSpace(part) == event {
			return true
		}
	}
	return false
}

func (d *Dispatcher) deliver(ctx context.Context, sub store.WebhookSubscription, body []byte) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.URL, bytes.NewReader(body))
	if err != nil {
		_ = d.store.TouchWebhookSubscription(sub.ID, 0, err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "GoshPanel-Webhooks/1.0")
	if sub.Secret != "" {
		mac := hmac.New(sha256.New, []byte(sub.Secret))
		mac.Write(body)
		req.Header.Set("X-GoshPanel-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	resp, err := d.client.Do(req)
	if err != nil {
		_ = d.store.TouchWebhookSubscription(sub.ID, 0, err.Error())
		if d.log != nil {
			d.log.Warn("webhook delivery failed", "name", sub.Name, "err", err)
		}
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		msg := fmt.Sprintf("HTTP %d", resp.StatusCode)
		_ = d.store.TouchWebhookSubscription(sub.ID, resp.StatusCode, msg)
		return
	}
	_ = d.store.TouchWebhookSubscription(sub.ID, resp.StatusCode, "")
}
