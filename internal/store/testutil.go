package store

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// OpenTest opens an in-memory SQLite database for tests.
func OpenTest(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	return db
}
