package security

import (
	"path/filepath"
	"testing"

	"github.com/schmorrison/goshpanel/internal/store"
)

func TestNormalizeCIDR(t *testing.T) {
	cases := map[string]string{
		"203.0.113.7":    "203.0.113.7/32",
		"203.0.113.0/24": "203.0.113.0/24",
		"203.0.113.9/24": "203.0.113.0/24", // masked
		"2001:db8::1":    "2001:db8::1/128",
	}
	for in, want := range cases {
		got, err := NormalizeCIDR(in)
		if err != nil || got != want {
			t.Errorf("NormalizeCIDR(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "not-an-ip", "300.0.0.1", "1.2.3.4/99"} {
		if _, err := NormalizeCIDR(in); err == nil {
			t.Errorf("NormalizeCIDR(%q) = nil error", in)
		}
	}
}

func TestBlocker(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if _, err := st.CreateIPRule("203.0.113.0/24", "test"); err != nil {
		t.Fatal(err)
	}
	b, err := NewBlocker(st)
	if err != nil {
		t.Fatalf("NewBlocker: %v", err)
	}

	if !b.Blocked("203.0.113.55:12345") {
		t.Error("in-range address not blocked")
	}
	if b.Blocked("198.51.100.1:80") {
		t.Error("out-of-range address blocked")
	}
	if b.Blocked("garbage") {
		t.Error("unparseable address blocked")
	}

	// New rules apply after Reload.
	if _, err := st.CreateIPRule("198.51.100.0/24", ""); err != nil {
		t.Fatal(err)
	}
	if err := b.Reload(); err != nil {
		t.Fatal(err)
	}
	if !b.Blocked("198.51.100.1:80") {
		t.Error("new rule not applied after reload")
	}
}
