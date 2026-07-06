package dbmanager

import (
	"context"
	"fmt"
)

// ApplyGrant runs CREATE USER and GRANT for MySQL/Postgres.
func ApplyGrant(ctx context.Context, driver, dsn, username, database, privileges string) error {
	db, err := open(ctx, driver, dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	if privileges == "" {
		privileges = "ALL"
	}
	switch driver {
	case "mysql":
		if _, err := db.ExecContext(ctx, fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY 'changeme'", username)); err != nil {
			return err
		}
		_, err = db.ExecContext(ctx, fmt.Sprintf("GRANT %s ON `%s`.* TO '%s'@'%%'", privileges, database, username))
		return err
	case "postgres":
		if _, err := db.ExecContext(ctx, fmt.Sprintf("CREATE USER %s WITH PASSWORD 'changeme'", username)); err != nil {
			// user may exist
		}
		_, err = db.ExecContext(ctx, fmt.Sprintf("GRANT %s ON DATABASE %s TO %s", privileges, database, username))
		return err
	default:
		return fmt.Errorf("grants not supported for driver %q", driver)
	}
}
