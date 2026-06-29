package dbmanager

import (
	"context"
	"database/sql"
	"testing"

	"github.com/schmorrison/goshpanel/internal/store"

	_ "modernc.org/sqlite"
)

func newTestRepo(t *testing.T) *store.ConnectionRepository {
	t.Helper()
	return store.NewConnectionRepository(store.OpenTest(t))
}

func storeConnection(name, driver, databaseName string) store.DBConnection {
	return store.DBConnection{
		Name:         name,
		Driver:       driver,
		DatabaseName: databaseName,
	}
}

func sqlOpenSQLite(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE notes (title TEXT); INSERT INTO notes(title) VALUES ('hello')`); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func TestContext(t *testing.T) {
	_ = context.Background()
}
