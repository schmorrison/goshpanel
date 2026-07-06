package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// DatabaseConn is a saved connection to a MySQL/PostgreSQL/SQLite database.
type DatabaseConn struct {
	ID        int64
	Name      string
	Driver    string // mysql, postgres, sqlite
	DSN       string
	CreatedAt time.Time
}

// CreateDatabaseConn inserts a saved connection.
func (s *Store) CreateDatabaseConn(name, driver, dsn string) (DatabaseConn, error) {
	c := DatabaseConn{Name: name, Driver: driver, DSN: dsn, CreatedAt: now()}
	res, err := s.db.Exec(
		`INSERT INTO db_connections (name, driver, dsn, created_at) VALUES (?, ?, ?, ?)`,
		c.Name, c.Driver, c.DSN, c.CreatedAt)
	if err != nil {
		return DatabaseConn{}, fmt.Errorf("create db connection: %w", err)
	}
	c.ID, _ = res.LastInsertId()
	return c, nil
}

// DatabaseConns lists saved connections.
func (s *Store) DatabaseConns() ([]DatabaseConn, error) {
	rows, err := s.db.Query(`SELECT id, name, driver, dsn, created_at FROM db_connections ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list db connections: %w", err)
	}
	defer rows.Close()
	var out []DatabaseConn
	for rows.Next() {
		var c DatabaseConn
		if err := rows.Scan(&c.ID, &c.Name, &c.Driver, &c.DSN, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DatabaseConnByID returns one saved connection.
func (s *Store) DatabaseConnByID(id int64) (DatabaseConn, error) {
	var c DatabaseConn
	err := s.db.QueryRow(`SELECT id, name, driver, dsn, created_at FROM db_connections WHERE id = ?`, id).
		Scan(&c.ID, &c.Name, &c.Driver, &c.DSN, &c.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return DatabaseConn{}, ErrNotFound
	}
	return c, err
}

// DeleteDatabaseConn removes a saved connection.
func (s *Store) DeleteDatabaseConn(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM db_connections WHERE id = ?`, id))
}
