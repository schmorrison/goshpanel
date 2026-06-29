package dbmanager

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/schmorrison/goshpanel/internal/crypto"
	"github.com/schmorrison/goshpanel/internal/store"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

const maxRows = 200

// QueryResult contains tabular query output.
type QueryResult struct {
	Columns []string
	Rows    [][]string
}

// Service manages external SQL database connections.
type Service struct {
	repo   *store.ConnectionRepository
	secret string
}

// NewService creates a database manager service.
func NewService(repo *store.ConnectionRepository, secret string) *Service {
	return &Service{repo: repo, secret: secret}
}

// ListConnections returns saved connection profiles.
func (s *Service) ListConnections(ctx context.Context) ([]store.DBConnection, error) {
	return s.repo.List(ctx)
}

// CreateConnection validates and stores a new connection.
func (s *Service) CreateConnection(ctx context.Context, conn store.DBConnection, password string) (store.DBConnection, error) {
	conn.Name = strings.TrimSpace(conn.Name)
	conn.Driver = strings.TrimSpace(strings.ToLower(conn.Driver))
	conn.DatabaseName = strings.TrimSpace(conn.DatabaseName)
	conn.Host = strings.TrimSpace(conn.Host)
	conn.Username = strings.TrimSpace(conn.Username)

	if conn.Name == "" || conn.Driver == "" || conn.DatabaseName == "" {
		return store.DBConnection{}, fmt.Errorf("name, driver, and database are required")
	}
	switch conn.Driver {
	case "sqlite", "postgres", "mysql":
	default:
		return store.DBConnection{}, fmt.Errorf("unsupported driver %q", conn.Driver)
	}

	if err := s.ping(conn, password); err != nil {
		return store.DBConnection{}, fmt.Errorf("connection test failed: %w", err)
	}

	encrypted, err := crypto.Encrypt(s.secret, password)
	if err != nil {
		return store.DBConnection{}, fmt.Errorf("encrypt password: %w", err)
	}
	conn.PasswordEncrypted = encrypted
	return s.repo.Create(ctx, conn)
}

// DeleteConnection removes a saved connection.
func (s *Service) DeleteConnection(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

// ListTables returns table names for a saved connection.
func (s *Service) ListTables(ctx context.Context, id int64) ([]string, error) {
	conn, password, db, err := s.open(ctx, id)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	switch conn.Driver {
	case "sqlite":
		return querySingleColumn(ctx, db, `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	case "postgres":
		return querySingleColumn(ctx, db, `SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename`)
	case "mysql":
		return querySingleColumn(ctx, db, `SHOW TABLES`)
	default:
		_ = password
		return nil, fmt.Errorf("unsupported driver %q", conn.Driver)
	}
}

// RunReadOnlyQuery executes a read-only SQL statement.
func (s *Service) RunReadOnlyQuery(ctx context.Context, id int64, query string) (QueryResult, error) {
	if err := validateReadOnlyQuery(query); err != nil {
		return QueryResult{}, err
	}

	_, _, db, err := s.open(ctx, id)
	if err != nil {
		return QueryResult{}, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return QueryResult{}, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return QueryResult{}, err
	}

	result := QueryResult{Columns: columns}
	for rows.Next() {
		if len(result.Rows) >= maxRows {
			break
		}
		values := make([]any, len(columns))
		ptrs := make([]any, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return QueryResult{}, err
		}
		row := make([]string, len(columns))
		for i, value := range values {
			row[i] = fmt.Sprint(value)
		}
		result.Rows = append(result.Rows, row)
	}
	return result, rows.Err()
}

func (s *Service) open(ctx context.Context, id int64) (store.DBConnection, string, *sql.DB, error) {
	conn, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return store.DBConnection{}, "", nil, err
	}

	password, err := crypto.Decrypt(s.secret, conn.PasswordEncrypted)
	if err != nil {
		return store.DBConnection{}, "", nil, fmt.Errorf("decrypt password: %w", err)
	}

	dsn, err := buildDSN(conn, password)
	if err != nil {
		return store.DBConnection{}, "", nil, err
	}

	driverName := conn.Driver
	if driverName == "postgres" {
		driverName = "pgx"
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return store.DBConnection{}, "", nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return store.DBConnection{}, "", nil, fmt.Errorf("ping database: %w", err)
	}

	return conn, password, db, nil
}

func (s *Service) ping(conn store.DBConnection, password string) error {
	dsn, err := buildDSN(conn, password)
	if err != nil {
		return err
	}
	driverName := conn.Driver
	if driverName == "postgres" {
		driverName = "pgx"
	}
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Ping()
}

func buildDSN(conn store.DBConnection, password string) (string, error) {
	switch conn.Driver {
	case "sqlite":
		return conn.DatabaseName, nil
	case "postgres":
		host := conn.Host
		if host == "" {
			host = "127.0.0.1"
		}
		port := conn.Port
		if port == 0 {
			port = 5432
		}
		user := conn.Username
		if user == "" {
			user = "postgres"
		}
		u := &url.URL{
			Scheme: "postgres",
			User:   url.UserPassword(user, password),
			Host:   fmt.Sprintf("%s:%d", host, port),
			Path:   conn.DatabaseName,
		}
		q := u.Query()
		q.Set("sslmode", "disable")
		u.RawQuery = q.Encode()
		return u.String(), nil
	case "mysql":
		host := conn.Host
		if host == "" {
			host = "127.0.0.1"
		}
		port := conn.Port
		if port == 0 {
			port = 3306
		}
		user := conn.Username
		if user == "" {
			user = "root"
		}
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true", user, password, host, port, conn.DatabaseName), nil
	default:
		return "", fmt.Errorf("unsupported driver %q", conn.Driver)
	}
}

func querySingleColumn(ctx context.Context, db *sql.DB, query string) ([]string, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func validateReadOnlyQuery(query string) error {
	query = strings.TrimSpace(query)
	query = strings.TrimSuffix(query, ";")
	if query == "" {
		return errors.New("query is required")
	}
	if strings.Contains(query, ";") {
		return errors.New("multiple statements are not allowed")
	}

	upper := strings.ToUpper(query)
	allowed := []string{"SELECT", "WITH", "SHOW", "PRAGMA", "EXPLAIN"}
	ok := false
	for _, prefix := range allowed {
		if strings.HasPrefix(upper, prefix) {
			ok = true
			break
		}
	}
	if !ok {
		return errors.New("only read-only queries are allowed")
	}
	return nil
}

// FormatPort returns the port as a string for forms.
func FormatPort(port int) string {
	if port == 0 {
		return ""
	}
	return strconv.Itoa(port)
}
