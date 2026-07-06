package dbmanager

import (
	"context"
	"path/filepath"
	"testing"
)

func TestExecDenylist(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	dsn := dbPath
	ctx := context.Background()
	if _, err := Exec(ctx, "sqlite", dsn, "CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	res, err := Exec(ctx, "sqlite", dsn, "INSERT INTO t (name) VALUES ('a')")
	if err != nil || res.RowsAffected != 1 {
		t.Fatalf("insert: res=%+v err=%v", res, err)
	}
	if _, err := Exec(ctx, "sqlite", dsn, "DROP TABLE t"); err == nil {
		t.Fatal("drop should be denied")
	}
}
