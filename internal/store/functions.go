package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// MicroFunction is an HTTP-triggered bash script.
type MicroFunction struct {
	ID          int64
	Name        string
	Description string
	Script      string
	Token       string
	Enabled     bool
	TimeoutSec  int
	CreatedAt   time.Time
}

// CreateMicroFunction inserts a function.
func (s *Store) CreateMicroFunction(name, description, script, token string, timeoutSec int) (MicroFunction, error) {
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	f := MicroFunction{
		Name: name, Description: description, Script: script, Token: token,
		Enabled: true, TimeoutSec: timeoutSec, CreatedAt: now(),
	}
	res, err := s.db.Exec(
		`INSERT INTO micro_functions (name, description, script, token, enabled, timeout_sec, created_at) VALUES (?, ?, ?, ?, 1, ?, ?)`,
		f.Name, f.Description, f.Script, f.Token, f.TimeoutSec, f.CreatedAt)
	if err != nil {
		return MicroFunction{}, fmt.Errorf("create function: %w", err)
	}
	f.ID, _ = res.LastInsertId()
	return f, nil
}

// MicroFunctions lists all functions ordered by name.
func (s *Store) MicroFunctions() ([]MicroFunction, error) {
	rows, err := s.db.Query(`SELECT id, name, description, script, token, enabled, timeout_sec, created_at FROM micro_functions ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list functions: %w", err)
	}
	defer rows.Close()
	return scanFunctions(rows)
}

// MicroFunctionByName returns one function by name.
func (s *Store) MicroFunctionByName(name string) (MicroFunction, error) {
	row := s.db.QueryRow(`SELECT id, name, description, script, token, enabled, timeout_sec, created_at FROM micro_functions WHERE name = ?`, name)
	return scanFunction(row)
}

// MicroFunctionByID returns one function by ID.
func (s *Store) MicroFunctionByID(id int64) (MicroFunction, error) {
	row := s.db.QueryRow(`SELECT id, name, description, script, token, enabled, timeout_sec, created_at FROM micro_functions WHERE id = ?`, id)
	return scanFunction(row)
}

// UpdateMicroFunction replaces script and metadata.
func (s *Store) UpdateMicroFunction(id int64, description, script string, enabled bool, timeoutSec int) error {
	return s.mustAffect(s.db.Exec(
		`UPDATE micro_functions SET description = ?, script = ?, enabled = ?, timeout_sec = ? WHERE id = ?`,
		description, script, boolToInt(enabled), timeoutSec, id))
}

// DeleteMicroFunction removes a function.
func (s *Store) DeleteMicroFunction(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM micro_functions WHERE id = ?`, id))
}

func scanFunction(row *sql.Row) (MicroFunction, error) {
	var f MicroFunction
	var enabled int
	err := row.Scan(&f.ID, &f.Name, &f.Description, &f.Script, &f.Token, &enabled, &f.TimeoutSec, &f.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return MicroFunction{}, ErrNotFound
	}
	f.Enabled = enabled == 1
	return f, err
}

func scanFunctions(rows *sql.Rows) ([]MicroFunction, error) {
	var out []MicroFunction
	for rows.Next() {
		var f MicroFunction
		var enabled int
		if err := rows.Scan(&f.ID, &f.Name, &f.Description, &f.Script, &f.Token, &enabled, &f.TimeoutSec, &f.CreatedAt); err != nil {
			return nil, err
		}
		f.Enabled = enabled == 1
		out = append(out, f)
	}
	return out, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
