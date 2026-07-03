package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// User is a panel account.
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	Role         string // "admin" or "user"
	CreatedAt    time.Time
}

// CreateUser inserts a new user and returns it with its assigned ID.
func (s *Store) CreateUser(username, passwordHash, role string) (User, error) {
	u := User{Username: username, PasswordHash: passwordHash, Role: role, CreatedAt: now()}
	res, err := s.db.Exec(
		`INSERT INTO users (username, password_hash, role, created_at) VALUES (?, ?, ?, ?)`,
		u.Username, u.PasswordHash, u.Role, u.CreatedAt)
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	u.ID, _ = res.LastInsertId()
	return u, nil
}

// UserByUsername looks a user up by name.
func (s *Store) UserByUsername(username string) (User, error) {
	return s.scanUser(s.db.QueryRow(
		`SELECT id, username, password_hash, role, created_at FROM users WHERE username = ?`, username))
}

// UserByID looks a user up by ID.
func (s *Store) UserByID(id int64) (User, error) {
	return s.scanUser(s.db.QueryRow(
		`SELECT id, username, password_hash, role, created_at FROM users WHERE id = ?`, id))
}

// Users returns all users ordered by username.
func (s *Store) Users() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, username, password_hash, role, created_at FROM users ORDER BY username`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// UpdateUserPassword replaces a user's password hash.
func (s *Store) UpdateUserPassword(id int64, passwordHash string) error {
	return s.mustAffect(s.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, id))
}

// DeleteUser removes a user and (via FK cascade) their sessions.
func (s *Store) DeleteUser(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM users WHERE id = ?`, id))
}

// CountUsers returns the number of user accounts.
func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) scanUser(row *sql.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

// mustAffect converts a zero-row result into ErrNotFound.
func (s *Store) mustAffect(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
