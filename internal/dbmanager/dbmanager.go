// Package dbmanager is the phpMyAdmin-style database module: it connects
// to saved MySQL, PostgreSQL or SQLite databases using pure-Go drivers,
// lists tables and runs read-only queries.
package dbmanager

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

// ErrNotReadOnly is returned when a query is not a plain SELECT-style read.
var ErrNotReadOnly = errors.New("only read-only queries (SELECT, SHOW, EXPLAIN, PRAGMA, WITH) are allowed")

// QueryResult holds the rows returned by a read-only query.
type QueryResult struct {
	Columns []string
	Rows    [][]string
	Elapsed time.Duration
}

// driverName maps a panel driver id to the database/sql driver name.
func driverName(driver string) (string, error) {
	switch driver {
	case "mysql":
		return "mysql", nil
	case "postgres":
		return "pgx", nil
	case "sqlite":
		return "sqlite", nil
	default:
		return "", fmt.Errorf("unsupported driver %q", driver)
	}
}

// open dials the target database with a short timeout.
func open(ctx context.Context, driver, dsn string) (*sql.DB, error) {
	name, err := driverName(driver)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(name, dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", driver, err)
	}
	db.SetMaxOpenConns(2)
	db.SetConnMaxLifetime(time.Minute)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect %s: %w", driver, err)
	}
	return db, nil
}

// Ping verifies connectivity to the target.
func Ping(ctx context.Context, driver, dsn string) error {
	db, err := open(ctx, driver, dsn)
	if err != nil {
		return err
	}
	return db.Close()
}

// Tables lists table names in the connected database.
func Tables(ctx context.Context, driver, dsn string) ([]string, error) {
	db, err := open(ctx, driver, dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var query string
	switch driver {
	case "mysql":
		query = "SHOW TABLES"
	case "postgres":
		query = "SELECT tablename FROM pg_catalog.pg_tables WHERE schemaname NOT IN ('pg_catalog','information_schema') ORDER BY tablename"
	case "sqlite":
		query = "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"
	}
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// readOnlyPrefixes are statement keywords accepted by Query.
var readOnlyPrefixes = []string{"select", "show", "explain", "pragma", "describe", "desc", "with"}

// Query runs a read-only SQL statement and returns up to maxRows rows.
func Query(ctx context.Context, driver, dsn, sqlText string, maxRows int) (QueryResult, error) {
	trimmed := strings.ToLower(strings.TrimSpace(sqlText))
	ok := false
	for _, p := range readOnlyPrefixes {
		if strings.HasPrefix(trimmed, p) {
			ok = true
			break
		}
	}
	if !ok {
		return QueryResult{}, ErrNotReadOnly
	}

	db, err := open(ctx, driver, dsn)
	if err != nil {
		return QueryResult{}, err
	}
	defer db.Close()

	start := time.Now()
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	rows, err := db.QueryContext(queryCtx, sqlText)
	if err != nil {
		return QueryResult{}, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return QueryResult{}, err
	}
	res := QueryResult{Columns: cols}
	raw := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range raw {
		ptrs[i] = &raw[i]
	}
	for rows.Next() && len(res.Rows) < maxRows {
		if err := rows.Scan(ptrs...); err != nil {
			return QueryResult{}, err
		}
		row := make([]string, len(cols))
		for i, v := range raw {
			row[i] = formatValue(v)
		}
		res.Rows = append(res.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return QueryResult{}, err
	}
	res.Elapsed = time.Since(start)
	return res, nil
}

func formatValue(v any) string {
	switch t := v.(type) {
	case nil:
		return "NULL"
	case []byte:
		return string(t)
	case time.Time:
		return t.Format(time.RFC3339)
	default:
		return fmt.Sprintf("%v", t)
	}
}
