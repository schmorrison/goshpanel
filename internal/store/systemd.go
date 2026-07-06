package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// SystemdUnit is a unit file managed by the orchestrator.
type SystemdUnit struct {
	ID          int64
	Name        string
	UnitContent string
	Enabled     bool
	CreatedAt   time.Time
}

// CreateSystemdUnit inserts a unit definition.
func (s *Store) CreateSystemdUnit(name, content string, enabled bool) (SystemdUnit, error) {
	u := SystemdUnit{Name: name, UnitContent: content, Enabled: enabled, CreatedAt: now()}
	res, err := s.db.Exec(
		`INSERT INTO systemd_units (name, unit_content, enabled, created_at) VALUES (?, ?, ?, ?)`,
		u.Name, u.UnitContent, boolToInt(u.Enabled), u.CreatedAt)
	if err != nil {
		return SystemdUnit{}, fmt.Errorf("create systemd unit: %w", err)
	}
	u.ID, _ = res.LastInsertId()
	return u, nil
}

// SystemdUnits lists all units.
func (s *Store) SystemdUnits() ([]SystemdUnit, error) {
	rows, err := s.db.Query(`SELECT id, name, unit_content, enabled, created_at FROM systemd_units ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list systemd units: %w", err)
	}
	defer rows.Close()
	var out []SystemdUnit
	for rows.Next() {
		var u SystemdUnit
		var enabled int
		if err := rows.Scan(&u.ID, &u.Name, &u.UnitContent, &enabled, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.Enabled = enabled == 1
		out = append(out, u)
	}
	return out, rows.Err()
}

// SystemdUnitByID returns one unit.
func (s *Store) SystemdUnitByID(id int64) (SystemdUnit, error) {
	var u SystemdUnit
	var enabled int
	err := s.db.QueryRow(`SELECT id, name, unit_content, enabled, created_at FROM systemd_units WHERE id = ?`, id).
		Scan(&u.ID, &u.Name, &u.UnitContent, &enabled, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return SystemdUnit{}, ErrNotFound
	}
	u.Enabled = enabled == 1
	return u, err
}

// DeleteSystemdUnit removes a unit definition.
func (s *Store) DeleteSystemdUnit(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM systemd_units WHERE id = ?`, id))
}
