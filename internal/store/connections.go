package store

import (
	"context"
	"database/sql"
	"fmt"
)

// DBConnection stores metadata for an external SQL database.
type DBConnection struct {
	ID                int64
	Name              string
	Driver            string
	Host              string
	Port              int
	DatabaseName      string
	Username          string
	PasswordEncrypted string
}

// ConnectionRepository persists SQL connection profiles.
type ConnectionRepository struct {
	db *sql.DB
}

// NewConnectionRepository returns a connection repository backed by db.
func NewConnectionRepository(db *sql.DB) *ConnectionRepository {
	return &ConnectionRepository{db: db}
}

// List returns all saved connections without passwords.
func (r *ConnectionRepository) List(ctx context.Context) ([]DBConnection, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, driver, host, port, database_name, username, password_encrypted
		FROM db_connections
		ORDER BY name COLLATE NOCASE
	`)
	if err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}
	defer rows.Close()

	var connections []DBConnection
	for rows.Next() {
		var conn DBConnection
		if err := rows.Scan(&conn.ID, &conn.Name, &conn.Driver, &conn.Host, &conn.Port, &conn.DatabaseName, &conn.Username, &conn.PasswordEncrypted); err != nil {
			return nil, fmt.Errorf("scan connection: %w", err)
		}
		connections = append(connections, conn)
	}
	return connections, rows.Err()
}

// Create inserts a new connection profile.
func (r *ConnectionRepository) Create(ctx context.Context, conn DBConnection) (DBConnection, error) {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO db_connections (name, driver, host, port, database_name, username, password_encrypted)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, conn.Name, conn.Driver, conn.Host, conn.Port, conn.DatabaseName, conn.Username, conn.PasswordEncrypted)
	if err != nil {
		return DBConnection{}, fmt.Errorf("create connection: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return DBConnection{}, fmt.Errorf("last insert id: %w", err)
	}
	return r.FindByID(ctx, id)
}

// FindByID returns a connection by id.
func (r *ConnectionRepository) FindByID(ctx context.Context, id int64) (DBConnection, error) {
	var conn DBConnection
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, driver, host, port, database_name, username, password_encrypted
		FROM db_connections
		WHERE id = ?
	`, id).Scan(&conn.ID, &conn.Name, &conn.Driver, &conn.Host, &conn.Port, &conn.DatabaseName, &conn.Username, &conn.PasswordEncrypted)
	if err != nil {
		return DBConnection{}, fmt.Errorf("find connection: %w", err)
	}
	return conn, nil
}

// Delete removes a connection by id.
func (r *ConnectionRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM db_connections WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete connection: %w", err)
	}
	return nil
}
