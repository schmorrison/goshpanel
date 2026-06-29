package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Open opens a SQLite database at path.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := Migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

// Migrate applies embedded SQL migrations.
func Migrate(db *sql.DB) error {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		sqlBytes, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}

		if _, err := db.Exec(string(sqlBytes)); err != nil {
			return fmt.Errorf("apply migration %s: %w", entry.Name(), err)
		}
	}

	return nil
}

// User represents an authenticated panel operator.
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    string
}

// UserRepository persists panel users.
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository returns a user repository backed by db.
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Count returns the number of users in the database.
func (r *UserRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return count, nil
}

// FindByUsername returns a user by username.
func (r *UserRepository) FindByUsername(ctx context.Context, username string) (User, error) {
	var user User
	err := r.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, created_at
		FROM users
		WHERE username = ?
	`, username).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		return User{}, fmt.Errorf("find user: %w", err)
	}
	return user, nil
}

// Create inserts a new user with the given password hash.
func (r *UserRepository) Create(ctx context.Context, username, passwordHash string) (User, error) {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO users (username, password_hash)
		VALUES (?, ?)
	`, username, passwordHash)
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("last insert id: %w", err)
	}

	return r.FindByID(ctx, id)
}

// FindByID returns a user by id.
func (r *UserRepository) FindByID(ctx context.Context, id int64) (User, error) {
	var user User
	err := r.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, created_at
		FROM users
		WHERE id = ?
	`, id).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		return User{}, fmt.Errorf("find user by id: %w", err)
	}
	return user, nil
}
