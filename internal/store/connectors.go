package store

import (
	"database/sql"
	"fmt"
	"time"
)

// ServiceConnector describes an external infrastructure target (Caddy in Docker,
// Postgres, MariaDB, remote Docker daemon, etc.).
type ServiceConnector struct {
	ID         int64
	Name       string
	Kind       string // caddy, docker, postgres, mysql
	Mode       string // local, docker
	ConfigJSON string
	Enabled    bool
	IsDefault  bool
	LastOKAt   *time.Time
	LastError  string
	CreatedAt  time.Time
}

// ServiceConnectors returns all connectors ordered by kind, name.
func (s *Store) ServiceConnectors() ([]ServiceConnector, error) {
	rows, err := s.db.Query(`SELECT id, name, kind, mode, config_json, enabled, is_default, last_ok_at, last_error, created_at
		FROM service_connectors ORDER BY kind, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanServiceConnectors(rows)
}

// ServiceConnectorsByKind returns enabled connectors for one kind.
func (s *Store) ServiceConnectorsByKind(kind string) ([]ServiceConnector, error) {
	rows, err := s.db.Query(`SELECT id, name, kind, mode, config_json, enabled, is_default, last_ok_at, last_error, created_at
		FROM service_connectors WHERE kind = ? AND enabled = 1 ORDER BY is_default DESC, name`, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanServiceConnectors(rows)
}

// ServiceConnectorByID loads one connector.
func (s *Store) ServiceConnectorByID(id int64) (ServiceConnector, error) {
	row := s.db.QueryRow(`SELECT id, name, kind, mode, config_json, enabled, is_default, last_ok_at, last_error, created_at
		FROM service_connectors WHERE id = ?`, id)
	return scanServiceConnector(row)
}

// DefaultServiceConnector returns the default connector for a kind, if any.
func (s *Store) DefaultServiceConnector(kind string) (ServiceConnector, error) {
	row := s.db.QueryRow(`SELECT id, name, kind, mode, config_json, enabled, is_default, last_ok_at, last_error, created_at
		FROM service_connectors WHERE kind = ? AND enabled = 1 AND is_default = 1 LIMIT 1`, kind)
	c, err := scanServiceConnector(row)
	if err != nil {
		return ServiceConnector{}, err
	}
	return c, nil
}

// CreateServiceConnector inserts a connector.
func (s *Store) CreateServiceConnector(c ServiceConnector) (ServiceConnector, error) {
	enabled := 0
	if c.Enabled {
		enabled = 1
	}
	isDefault := 0
	if c.IsDefault {
		isDefault = 1
	}
	res, err := s.db.Exec(`INSERT INTO service_connectors (name, kind, mode, config_json, enabled, is_default)
		VALUES (?, ?, ?, ?, ?, ?)`,
		c.Name, c.Kind, c.Mode, c.ConfigJSON, enabled, isDefault)
	if err != nil {
		return ServiceConnector{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return ServiceConnector{}, err
	}
	if c.IsDefault {
		if err := s.clearDefaultConnector(c.Kind, id); err != nil {
			return ServiceConnector{}, err
		}
	}
	return s.ServiceConnectorByID(id)
}

// UpdateServiceConnector updates mutable fields.
func (s *Store) UpdateServiceConnector(c ServiceConnector) error {
	enabled := 0
	if c.Enabled {
		enabled = 1
	}
	isDefault := 0
	if c.IsDefault {
		isDefault = 1
	}
	if _, err := s.db.Exec(`UPDATE service_connectors SET name = ?, mode = ?, config_json = ?, enabled = ?, is_default = ?
		WHERE id = ?`, c.Name, c.Mode, c.ConfigJSON, enabled, isDefault, c.ID); err != nil {
		return err
	}
	if c.IsDefault {
		return s.clearDefaultConnector(c.Kind, c.ID)
	}
	return nil
}

// DeleteServiceConnector removes a connector.
func (s *Store) DeleteServiceConnector(id int64) error {
	_, err := s.db.Exec(`DELETE FROM service_connectors WHERE id = ?`, id)
	return err
}

// TouchServiceConnector records health check outcome.
func (s *Store) TouchServiceConnector(id int64, ok bool, errMsg string) error {
	if ok {
		_, err := s.db.Exec(`UPDATE service_connectors SET last_ok_at = ?, last_error = '' WHERE id = ?`, now(), id)
		return err
	}
	_, err := s.db.Exec(`UPDATE service_connectors SET last_error = ? WHERE id = ?`, errMsg, id)
	return err
}

func (s *Store) clearDefaultConnector(kind string, keepID int64) error {
	_, err := s.db.Exec(`UPDATE service_connectors SET is_default = 0 WHERE kind = ? AND id != ?`, kind, keepID)
	return err
}

func scanServiceConnectors(rows *sql.Rows) ([]ServiceConnector, error) {
	var out []ServiceConnector
	for rows.Next() {
		c, err := scanServiceConnector(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type scScanner interface {
	Scan(dest ...any) error
}

func scanServiceConnector(row scScanner) (ServiceConnector, error) {
	var c ServiceConnector
	var enabled, isDefault int
	var lastOK sql.NullTime
	if err := row.Scan(&c.ID, &c.Name, &c.Kind, &c.Mode, &c.ConfigJSON, &enabled, &isDefault, &lastOK, &c.LastError, &c.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return ServiceConnector{}, ErrNotFound
		}
		return ServiceConnector{}, err
	}
	c.Enabled = enabled == 1
	c.IsDefault = isDefault == 1
	if lastOK.Valid {
		t := lastOK.Time
		c.LastOKAt = &t
	}
	return c, nil
}

// ValidateConnectorKind reports whether kind is supported.
func ValidateConnectorKind(kind string) error {
	switch kind {
	case "caddy", "docker", "postgres", "mysql", "coredns", "maddy":
		return nil
	default:
		return fmt.Errorf("unsupported connector kind %q", kind)
	}
}
