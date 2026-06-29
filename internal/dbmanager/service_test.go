package dbmanager

import (
	"context"
	"testing"
)

func TestValidateReadOnlyQuery(t *testing.T) {
	tests := []struct {
		query string
		ok    bool
	}{
		{"SELECT 1", true},
		{"WITH cte AS (SELECT 1) SELECT * FROM cte", true},
		{"DELETE FROM users", false},
		{"SELECT 1; DROP TABLE users", false},
	}

	for _, tc := range tests {
		err := validateReadOnlyQuery(tc.query)
		if tc.ok && err != nil {
			t.Fatalf("validateReadOnlyQuery(%q) error = %v", tc.query, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("validateReadOnlyQuery(%q) error = nil, want error", tc.query)
		}
	}
}

func TestSQLiteConnectionAndQuery(t *testing.T) {
	ctx := context.Background()
	dbPath := t.TempDir() + "/sample.db"
	setup, err := sqlOpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("setup sqlite: %v", err)
	}
	defer setup.Close()

	repo := newTestRepo(t)
	svc := NewService(repo, "test-secret")

	conn, err := svc.CreateConnection(ctx, storeConnection("local", "sqlite", dbPath), "")
	if err != nil {
		t.Fatalf("CreateConnection() error = %v", err)
	}

	tables, err := svc.ListTables(ctx, conn.ID)
	if err != nil {
		t.Fatalf("ListTables() error = %v", err)
	}
	if len(tables) != 1 || tables[0] != "notes" {
		t.Fatalf("ListTables() = %v", tables)
	}

	result, err := svc.RunReadOnlyQuery(ctx, conn.ID, "SELECT title FROM notes")
	if err != nil {
		t.Fatalf("RunReadOnlyQuery() error = %v", err)
	}
	if len(result.Rows) != 1 || result.Rows[0][0] != "hello" {
		t.Fatalf("RunReadOnlyQuery() = %+v", result)
	}
}
