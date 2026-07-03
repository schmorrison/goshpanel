package store

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"
)

// AccessLogEvent is one parsed HTTP access log entry.
type AccessLogEvent struct {
	ID         int64
	RecordedAt time.Time
	Host       string
	Method     string
	Path       string
	Status     int
	Bytes      int64
	RemoteIP   string
}

// RecordAccessLogEvents inserts parsed access log rows.
func (s *Store) RecordAccessLogEvents(events []AccessLogEvent) error {
	if len(events) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO access_log_events (recorded_at, host, method, path, status, bytes, remote_ip) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, e := range events {
		if _, err := stmt.Exec(e.RecordedAt.UTC(), e.Host, e.Method, e.Path, e.Status, e.Bytes, e.RemoteIP); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM access_log_events WHERE id NOT IN (SELECT id FROM access_log_events ORDER BY id DESC LIMIT 500000)`)
	return err
}

// AccessLogSummary holds aggregate analytics for a time window.
type AccessLogSummary struct {
	TotalRequests int64
	TotalBytes    int64
	Status2xx     int64
	Status3xx     int64
	Status4xx     int64
	Status5xx     int64
}

// AccessLogSummarySince aggregates events since the given time.
func (s *Store) AccessLogSummarySince(since time.Time) (AccessLogSummary, error) {
	var sum AccessLogSummary
	err := s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(bytes),0),
			COALESCE(SUM(CASE WHEN status BETWEEN 200 AND 299 THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN status BETWEEN 300 AND 399 THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN status BETWEEN 400 AND 499 THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN status >= 500 THEN 1 ELSE 0 END),0)
		FROM access_log_events WHERE recorded_at >= ?`, since.UTC()).
		Scan(&sum.TotalRequests, &sum.TotalBytes, &sum.Status2xx, &sum.Status3xx, &sum.Status4xx, &sum.Status5xx)
	return sum, err
}

// TopPathCount is a path and its hit count.
type TopPathCount struct {
	Path  string
	Count int64
}

// TopAccessPaths returns the most requested paths since since.
func (s *Store) TopAccessPaths(since time.Time, limit int) ([]TopPathCount, error) {
	rows, err := s.db.Query(`
		SELECT path, COUNT(*) AS c FROM access_log_events
		WHERE recorded_at >= ? GROUP BY path ORDER BY c DESC LIMIT ?`, since.UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TopPathCount
	for rows.Next() {
		var row TopPathCount
		if err := rows.Scan(&row.Path, &row.Count); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// AccessLogHourly returns request counts bucketed by hour.
func (s *Store) AccessLogHourly(since time.Time) ([]int64, []string, error) {
	rows, err := s.db.Query(`
		SELECT strftime('%Y-%m-%d %H:00', recorded_at) AS bucket, COUNT(*)
		FROM access_log_events WHERE recorded_at >= ?
		GROUP BY bucket ORDER BY bucket`, since.UTC())
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var counts []int64
	var labels []string
	for rows.Next() {
		var label string
		var count int64
		if err := rows.Scan(&label, &count); err != nil {
			return nil, nil, err
		}
		labels = append(labels, label)
		counts = append(counts, count)
	}
	return counts, labels, rows.Err()
}

// AnalyticsOffset returns the persisted byte offset for log ingestion.
func (s *Store) AnalyticsOffset() (int64, error) {
	var val string
	err := s.db.QueryRow(`SELECT value FROM analytics_state WHERE key = 'access_log_offset'`).Scan(&val)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse analytics offset: %w", err)
	}
	return n, nil
}

// SetAnalyticsOffset persists the log ingestion byte offset.
func (s *Store) SetAnalyticsOffset(offset int64) error {
	_, err := s.db.Exec(`INSERT INTO analytics_state (key, value) VALUES ('access_log_offset', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, strconv.FormatInt(offset, 10))
	return err
}
