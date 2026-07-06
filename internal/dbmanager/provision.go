package dbmanager

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var identRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func validateIdent(name string) error {
	if !identRe.MatchString(name) {
		return fmt.Errorf("invalid identifier %q", name)
	}
	return nil
}

// CreateDatabase creates an empty database on MySQL/MariaDB or PostgreSQL.
func CreateDatabase(ctx context.Context, driver, adminDSN, name string) error {
	if err := validateIdent(name); err != nil {
		return err
	}
	db, err := open(ctx, driver, adminDSN)
	if err != nil {
		return err
	}
	defer db.Close()

	switch driver {
	case "mysql":
		_, err = db.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", name))
	case "postgres":
		// CREATE DATABASE cannot run in a transaction; use simple exec
		_, err = db.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %s", name))
		if err != nil && strings.Contains(strings.ToLower(err.Error()), "already exists") {
			return nil
		}
	default:
		return fmt.Errorf("create database not supported for driver %q", driver)
	}
	return err
}
