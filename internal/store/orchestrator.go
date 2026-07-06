package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// OrchestratorStatus tracks the last apply attempt for a component.
type OrchestratorStatus struct {
	Component   string
	ConfigPath  string
	LastApplied *time.Time
	LastError   string
}

// SetOrchestratorStatus records apply outcome for a component.
func (s *Store) SetOrchestratorStatus(component, configPath, lastError string, applied time.Time) error {
	_, err := s.db.Exec(`
		INSERT INTO orchestrator_status (component, config_path, last_applied, last_error)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(component) DO UPDATE SET
			config_path = excluded.config_path,
			last_applied = excluded.last_applied,
			last_error = excluded.last_error`,
		component, configPath, applied.UTC(), lastError)
	if err != nil {
		return fmt.Errorf("set orchestrator status: %w", err)
	}
	return nil
}

// OrchestratorStatuses returns status for all components.
func (s *Store) OrchestratorStatuses() ([]OrchestratorStatus, error) {
	rows, err := s.db.Query(`SELECT component, config_path, last_applied, last_error FROM orchestrator_status ORDER BY component`)
	if err != nil {
		return nil, fmt.Errorf("list orchestrator status: %w", err)
	}
	defer rows.Close()
	var out []OrchestratorStatus
	for rows.Next() {
		var st OrchestratorStatus
		var applied sql.NullTime
		if err := rows.Scan(&st.Component, &st.ConfigPath, &applied, &st.LastError); err != nil {
			return nil, err
		}
		if applied.Valid {
			t := applied.Time
			st.LastApplied = &t
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// OrchestratorStatusByComponent returns one component's status.
func (s *Store) OrchestratorStatusByComponent(component string) (OrchestratorStatus, error) {
	var st OrchestratorStatus
	var applied sql.NullTime
	err := s.db.QueryRow(`SELECT component, config_path, last_applied, last_error FROM orchestrator_status WHERE component = ?`, component).
		Scan(&st.Component, &st.ConfigPath, &applied, &st.LastError)
	if errors.Is(err, sql.ErrNoRows) {
		return OrchestratorStatus{Component: component}, nil
	}
	if applied.Valid {
		t := applied.Time
		st.LastApplied = &t
	}
	return st, err
}
