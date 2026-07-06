package connector

import (
	"testing"
)

func TestDatabaseDSN(t *testing.T) {
	driver, dsn, err := DatabaseDSN(KindPostgres, DatabaseConfig{
		Host: "db", Port: 5432, AdminUser: "postgres", AdminPassword: "secret", DefaultDatabase: "postgres",
	})
	if err != nil || driver != "postgres" || dsn == "" {
		t.Fatalf("postgres dsn: driver=%q dsn=%q err=%v", driver, dsn, err)
	}
	driver, dsn, err = DatabaseDSN(KindMySQL, DatabaseConfig{
		Host: "127.0.0.1", Port: 3306, AdminUser: "root", AdminPassword: "pw",
	})
	if err != nil || driver != "mysql" {
		t.Fatalf("mysql: %v", err)
	}
	if dsn != "root:pw@tcp(127.0.0.1:3306)/" {
		t.Fatalf("dsn = %q", dsn)
	}
}

func TestParseCaddyConfigDefaults(t *testing.T) {
	cfg, err := ParseCaddyConfig(`{"container":"caddy"}`)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConfigPathContainer != "/etc/caddy/Caddyfile" {
		t.Fatalf("default path = %q", cfg.ConfigPathContainer)
	}
}
