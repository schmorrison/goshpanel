package dbmanager

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

// newTestDB creates a throwaway SQLite database with sample data and
// returns its DSN.
func newTestDB(t *testing.T) string {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "sample.db")
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE fruits (id INTEGER PRIMARY KEY, name TEXT);
		INSERT INTO fruits (name) VALUES ('apple'), ('banana');`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return dsn
}

func TestPingAndTables(t *testing.T) {
	dsn := newTestDB(t)
	ctx := context.Background()

	if err := Ping(ctx, "sqlite", dsn); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	tables, err := Tables(ctx, "sqlite", dsn)
	if err != nil || !reflect.DeepEqual(tables, []string{"fruits"}) {
		t.Fatalf("Tables: %v %v", err, tables)
	}
}

func TestQueryReadOnly(t *testing.T) {
	dsn := newTestDB(t)
	ctx := context.Background()

	res, err := Query(ctx, "sqlite", dsn, "SELECT name FROM fruits ORDER BY name", 100)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(res.Rows) != 2 || res.Rows[0][0] != "apple" {
		t.Errorf("rows = %+v", res.Rows)
	}

	if _, err := Query(ctx, "sqlite", dsn, "DELETE FROM fruits", 100); !errors.Is(err, ErrNotReadOnly) {
		t.Errorf("DELETE should be rejected, got %v", err)
	}
	if _, err := Query(ctx, "sqlite", dsn, "  drop table fruits", 100); !errors.Is(err, ErrNotReadOnly) {
		t.Errorf("DROP should be rejected, got %v", err)
	}
}

func TestQueryRowLimit(t *testing.T) {
	dsn := newTestDB(t)
	res, err := Query(context.Background(), "sqlite", dsn, "SELECT * FROM fruits", 1)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(res.Rows) != 1 {
		t.Errorf("expected 1 row (limit), got %d", len(res.Rows))
	}
}

func TestUnsupportedDriver(t *testing.T) {
	if err := Ping(context.Background(), "oracle", "x"); err == nil {
		t.Error("unsupported driver accepted")
	}
}
