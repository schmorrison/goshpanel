package dbmanager

import (
	"context"
	"fmt"
	"strings"
)

// ApplyGrant runs CREATE USER and GRANT for MySQL/Postgres.
func ApplyGrant(ctx context.Context, driver, dsn, username, database, password, privileges string) error {
	db, err := open(ctx, driver, dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	if privileges == "" {
		privileges = "ALL"
	}
	if password == "" {
		password = "changeme"
	}
	switch driver {
	case "mysql":
		if err := validateIdent(username); err != nil {
			return err
		}
		if err := validateIdent(database); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s'", username, escapeSQLString(password))); err != nil {
			return err
		}
		_, err = db.ExecContext(ctx, fmt.Sprintf("GRANT %s ON `%s`.* TO '%s'@'%%'", privileges, database, username))
		return err
	case "postgres":
		if err := validateIdent(username); err != nil {
			return err
		}
		if err := validateIdent(database); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s'", username, escapeSQLString(password))); err != nil {
			// user may exist — try password update
			_, _ = db.ExecContext(ctx, fmt.Sprintf("ALTER USER %s WITH PASSWORD '%s'", username, escapeSQLString(password)))
		}
		_, err = db.ExecContext(ctx, fmt.Sprintf("GRANT %s ON DATABASE %s TO %s", privileges, database, username))
		return err
	default:
		return fmt.Errorf("grants not supported for driver %q", driver)
	}
}

func escapeSQLString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
