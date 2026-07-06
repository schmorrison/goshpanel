package dns

import (
	"strings"
	"testing"

	"github.com/schmorrison/goshpanel/internal/store"
)

func TestValidate(t *testing.T) {
	valid := []store.DNSRecord{
		{Type: "A", Name: "@", Value: "192.0.2.1"},
		{Type: "AAAA", Name: "www", Value: "2001:db8::1"},
		{Type: "CNAME", Name: "blog", Value: "host.example.com"},
		{Type: "MX", Name: "@", Value: "mail.example.com", Priority: 10},
		{Type: "TXT", Name: "@", Value: "v=spf1 -all"},
	}
	for _, r := range valid {
		if err := Validate(r); err != nil {
			t.Errorf("Validate(%s %s) = %v, want nil", r.Type, r.Name, err)
		}
	}

	invalid := []store.DNSRecord{
		{Type: "A", Name: "@", Value: "not-an-ip"},
		{Type: "A", Name: "@", Value: "2001:db8::1"},
		{Type: "AAAA", Name: "@", Value: "192.0.2.1"},
		{Type: "CNAME", Name: "x", Value: ""},
		{Type: "BOGUS", Name: "x", Value: "y"},
		{Type: "A", Name: "", Value: "192.0.2.1"},
	}
	for _, r := range invalid {
		if err := Validate(r); err == nil {
			t.Errorf("Validate(%s %q %q) = nil, want error", r.Type, r.Name, r.Value)
		}
	}
}

func TestRenderZone(t *testing.T) {
	d := store.Domain{Name: "example.com"}
	out := RenderZone(d, []store.DNSRecord{
		{Type: "A", Name: "@", Value: "192.0.2.1", TTL: 300},
		{Type: "MX", Name: "@", Value: "mail.example.com", Priority: 10},
		{Type: "TXT", Name: "@", Value: "v=spf1 -all"},
	})
	for _, want := range []string{
		"$ORIGIN example.com.",
		"IN\tSOA",
		"@\t300\tIN\tA\t192.0.2.1",
		"IN\tMX\t10 mail.example.com.",
		`IN\tTXT\t"v=spf1 -all"`,
	} {
		want = strings.ReplaceAll(want, `\t`, "\t")
		if !strings.Contains(out, want) {
			t.Errorf("zone missing %q:\n%s", want, out)
		}
	}
}
