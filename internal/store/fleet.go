package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// FleetNode is a remote GoshPanel instance managed by the controller.
type FleetNode struct {
	ID         int64
	Name       string
	BaseURL    string
	Token      string
	Enabled    bool
	LastSeenAt *time.Time
	LastError  string
	CreatedAt  time.Time
}

// FleetTelemetrySample is one stored telemetry payload from a node.
type FleetTelemetrySample struct {
	ID         int64
	NodeID     int64
	Payload    string
	RecordedAt time.Time
}

// FleetCommand is a remote control action sent to a node.
type FleetCommand struct {
	ID          int64
	NodeID      int64
	Action      string
	Status      string
	Result      string
	CreatedAt   time.Time
	CompletedAt *time.Time
}

// MetricSample is a local host metrics datapoint.
type MetricSample struct {
	ID          int64
	Load1       float64
	MemUsedPct  float64
	DiskUsedPct float64
	RecordedAt  time.Time
}

// CreateFleetNode registers a remote node.
func (s *Store) CreateFleetNode(name, baseURL, token string) (FleetNode, error) {
	n := FleetNode{Name: name, BaseURL: baseURL, Token: token, Enabled: true, CreatedAt: now()}
	res, err := s.db.Exec(
		`INSERT INTO fleet_nodes (name, base_url, token, enabled, created_at) VALUES (?, ?, ?, 1, ?)`,
		n.Name, n.BaseURL, n.Token, n.CreatedAt)
	if err != nil {
		return FleetNode{}, fmt.Errorf("create fleet node: %w", err)
	}
	n.ID, _ = res.LastInsertId()
	return n, nil
}

// FleetNodes lists registered nodes.
func (s *Store) FleetNodes() ([]FleetNode, error) {
	rows, err := s.db.Query(`SELECT id, name, base_url, token, enabled, last_seen_at, last_error, created_at FROM fleet_nodes ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFleetNodes(rows)
}

// FleetNodeByID returns one node.
func (s *Store) FleetNodeByID(id int64) (FleetNode, error) {
	return scanFleetNode(s.db.QueryRow(
		`SELECT id, name, base_url, token, enabled, last_seen_at, last_error, created_at FROM fleet_nodes WHERE id = ?`, id))
}

// FleetNodeByName returns one node by name.
func (s *Store) FleetNodeByName(name string) (FleetNode, error) {
	return scanFleetNode(s.db.QueryRow(
		`SELECT id, name, base_url, token, enabled, last_seen_at, last_error, created_at FROM fleet_nodes WHERE name = ?`, name))
}

// DeleteFleetNode removes a node and cascades telemetry/commands.
func (s *Store) DeleteFleetNode(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM fleet_nodes WHERE id = ?`, id))
}

// SetFleetNodeSeen updates last_seen and clears error.
func (s *Store) SetFleetNodeSeen(id int64, t time.Time) error {
	_, err := s.db.Exec(`UPDATE fleet_nodes SET last_seen_at = ?, last_error = '' WHERE id = ?`, t.UTC(), id)
	return err
}

// SetFleetNodeError records a poll/push failure.
func (s *Store) SetFleetNodeError(id int64, msg string) error {
	_, err := s.db.Exec(`UPDATE fleet_nodes SET last_error = ? WHERE id = ?`, msg, id)
	return err
}

// RecordFleetTelemetry stores a JSON telemetry payload.
func (s *Store) RecordFleetTelemetry(nodeID int64, payload []byte) error {
	_, err := s.db.Exec(`INSERT INTO fleet_telemetry (node_id, payload, recorded_at) VALUES (?, ?, ?)`, nodeID, string(payload), now())
	if err != nil {
		return err
	}
	// Keep last 500 samples per node.
	_, err = s.db.Exec(`DELETE FROM fleet_telemetry WHERE node_id = ? AND id NOT IN (
		SELECT id FROM fleet_telemetry WHERE node_id = ? ORDER BY id DESC LIMIT 500
	)`, nodeID, nodeID)
	return err
}

// LatestFleetTelemetry returns the newest sample for a node.
func (s *Store) LatestFleetTelemetry(nodeID int64) (FleetTelemetrySample, error) {
	var sample FleetTelemetrySample
	err := s.db.QueryRow(
		`SELECT id, node_id, payload, recorded_at FROM fleet_telemetry WHERE node_id = ? ORDER BY id DESC LIMIT 1`, nodeID).
		Scan(&sample.ID, &sample.NodeID, &sample.Payload, &sample.RecordedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return FleetTelemetrySample{}, ErrNotFound
	}
	return sample, err
}

// FleetTelemetryHistory returns recent samples for a node.
func (s *Store) FleetTelemetryHistory(nodeID int64, limit int) ([]FleetTelemetrySample, error) {
	rows, err := s.db.Query(
		`SELECT id, node_id, payload, recorded_at FROM fleet_telemetry WHERE node_id = ? ORDER BY id DESC LIMIT ?`, nodeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FleetTelemetrySample
	for rows.Next() {
		var sample FleetTelemetrySample
		if err := rows.Scan(&sample.ID, &sample.NodeID, &sample.Payload, &sample.RecordedAt); err != nil {
			return nil, err
		}
		out = append(out, sample)
	}
	return out, rows.Err()
}

// CreateFleetCommand records a pending remote command.
func (s *Store) CreateFleetCommand(nodeID int64, action string) (FleetCommand, error) {
	res, err := s.db.Exec(
		`INSERT INTO fleet_commands (node_id, action, status, created_at) VALUES (?, ?, 'pending', ?)`,
		nodeID, action, now())
	if err != nil {
		return FleetCommand{}, err
	}
	cmd := FleetCommand{NodeID: nodeID, Action: action, Status: "pending", CreatedAt: now()}
	cmd.ID, _ = res.LastInsertId()
	return cmd, nil
}

// CompleteFleetCommand marks a command done.
func (s *Store) CompleteFleetCommand(id int64, status, result string) error {
	_, err := s.db.Exec(
		`UPDATE fleet_commands SET status = ?, result = ?, completed_at = ? WHERE id = ?`,
		status, result, now(), id)
	return err
}

// FleetCommands lists recent commands for a node.
func (s *Store) FleetCommands(nodeID int64, limit int) ([]FleetCommand, error) {
	rows, err := s.db.Query(
		`SELECT id, node_id, action, status, result, created_at, completed_at FROM fleet_commands WHERE node_id = ? ORDER BY id DESC LIMIT ?`,
		nodeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FleetCommand
	for rows.Next() {
		var cmd FleetCommand
		var completed sql.NullTime
		if err := rows.Scan(&cmd.ID, &cmd.NodeID, &cmd.Action, &cmd.Status, &cmd.Result, &cmd.CreatedAt, &completed); err != nil {
			return nil, err
		}
		if completed.Valid {
			t := completed.Time
			cmd.CompletedAt = &t
		}
		out = append(out, cmd)
	}
	return out, rows.Err()
}

// RecordMetricSample inserts a local metrics datapoint.
func (s *Store) RecordMetricSample(load1, memPct, diskPct float64) error {
	_, err := s.db.Exec(
		`INSERT INTO metric_samples (load1, mem_used_pct, disk_used_pct, recorded_at) VALUES (?, ?, ?, ?)`,
		load1, memPct, diskPct, now())
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM metric_samples WHERE id NOT IN (SELECT id FROM metric_samples ORDER BY id DESC LIMIT 10080)`)
	return err
}

// MetricSamples returns recent local metric samples, oldest first.
func (s *Store) MetricSamples(limit int) ([]MetricSample, error) {
	rows, err := s.db.Query(
		`SELECT id, load1, mem_used_pct, disk_used_pct, recorded_at FROM metric_samples ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MetricSample
	for rows.Next() {
		var m MetricSample
		if err := rows.Scan(&m.ID, &m.Load1, &m.MemUsedPct, &m.DiskUsedPct, &m.RecordedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	// reverse to oldest-first for charts
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}

func scanFleetNode(row *sql.Row) (FleetNode, error) {
	var n FleetNode
	var enabled int
	var seen sql.NullTime
	err := row.Scan(&n.ID, &n.Name, &n.BaseURL, &n.Token, &enabled, &seen, &n.LastError, &n.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return FleetNode{}, ErrNotFound
	}
	n.Enabled = enabled == 1
	if seen.Valid {
		t := seen.Time
		n.LastSeenAt = &t
	}
	return n, err
}

func scanFleetNodes(rows *sql.Rows) ([]FleetNode, error) {
	var out []FleetNode
	for rows.Next() {
		var n FleetNode
		var enabled int
		var seen sql.NullTime
		if err := rows.Scan(&n.ID, &n.Name, &n.BaseURL, &n.Token, &enabled, &seen, &n.LastError, &n.CreatedAt); err != nil {
			return nil, err
		}
		n.Enabled = enabled == 1
		if seen.Valid {
			t := seen.Time
			n.LastSeenAt = &t
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
