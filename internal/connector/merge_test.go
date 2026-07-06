package connector

import "testing"

func TestMergeDatabaseConfigKeepsPassword(t *testing.T) {
	existing := DatabaseConfig{
		Host:             "127.0.0.1",
		AdminUser:        "root",
		AdminPasswordEnc: "enc",
	}
	updated := DatabaseConfig{Host: "10.0.0.5", AdminUser: "admin"}
	got := MergeDatabaseConfig(existing, updated)
	if got.AdminPasswordEnc != "enc" {
		t.Fatalf("password enc not preserved: %+v", got)
	}
	if got.Host != "10.0.0.5" {
		t.Fatalf("host not updated: %+v", got)
	}
}

func TestMergeCaddyConfigKeepsToken(t *testing.T) {
	existing := CaddyConfig{AdminToken: "secret"}
	updated := CaddyConfig{AdminURL: "http://127.0.0.1:2019"}
	got := MergeCaddyConfig(existing, updated)
	if got.AdminToken != "secret" || got.AdminURL == "" {
		t.Fatalf("merge failed: %+v", got)
	}
}
