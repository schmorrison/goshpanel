package dbmanager

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrWriteDenied is returned when a write query matches the denylist.
var ErrWriteDenied = errors.New("query contains a denied statement (DROP, ALTER, TRUNCATE, GRANT, etc.)")

// ExecResult holds the outcome of a write query.
type ExecResult struct {
	RowsAffected int64
	Elapsed      time.Duration
}

var writeDenylist = []string{
	"drop database", "drop schema", "drop table", "truncate", "alter ", "grant ", "revoke ",
	"create database", "attach ", "detach ", "pragma writable_schema", "vacuum",
}

// Exec runs a write SQL statement with a keyword denylist.
func Exec(ctx context.Context, driver, dsn, sqlText string) (ExecResult, error) {
	trimmed := strings.ToLower(strings.TrimSpace(sqlText))
	if trimmed == "" {
		return ExecResult{}, fmt.Errorf("empty query")
	}
	for _, denied := range writeDenylist {
		if strings.Contains(trimmed, denied) {
			return ExecResult{}, ErrWriteDenied
		}
	}
	allowed := strings.HasPrefix(trimmed, "insert") ||
		strings.HasPrefix(trimmed, "update") ||
		strings.HasPrefix(trimmed, "delete") ||
		strings.HasPrefix(trimmed, "replace") ||
		strings.HasPrefix(trimmed, "create table") ||
		strings.HasPrefix(trimmed, "create index") ||
		strings.HasPrefix(trimmed, "create unique index")
	if !allowed {
		return ExecResult{}, fmt.Errorf("only INSERT, UPDATE, DELETE, REPLACE, CREATE TABLE/INDEX are allowed in write mode")
	}

	db, err := open(ctx, driver, dsn)
	if err != nil {
		return ExecResult{}, err
	}
	defer db.Close()

	start := time.Now()
	execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	res, err := db.ExecContext(execCtx, sqlText)
	if err != nil {
		return ExecResult{}, fmt.Errorf("exec: %w", err)
	}
	affected, _ := res.RowsAffected()
	return ExecResult{RowsAffected: affected, Elapsed: time.Since(start)}, nil
}
