package connector

import (
	"context"

	"github.com/schmorrison/goshpanel/internal/dbmanager"
)

func pingDB(ctx context.Context, driver, dsn string) error {
	return dbmanager.Ping(ctx, driver, dsn)
}

// ProvisionDatabase creates a database and optional user on a connector.
func ProvisionDatabase(ctx context.Context, reg *Registry, kind Kind, rawConfig string, dbName, username, password, privileges string) error {
	cfg, err := reg.ResolveDatabaseConfig(rawConfig)
	if err != nil {
		return err
	}
	driver, dsn, err := DatabaseDSN(kind, cfg)
	if err != nil {
		return err
	}
	if err := dbmanager.CreateDatabase(ctx, driver, dsn, dbName); err != nil {
		return err
	}
	if username != "" {
		return dbmanager.ApplyGrant(ctx, driver, dsn, username, dbName, password, privileges)
	}
	return nil
}
