package email

import (
	"reflect"
	"strings"
	"testing"

	"github.com/schmorrison/goshpanel/internal/store"
)

func TestValidateAddress(t *testing.T) {
	if err := ValidateAddress("user@example.com"); err != nil {
		t.Errorf("valid address rejected: %v", err)
	}
	for _, addr := range []string{"", "nope", "a b@example.com", "Display <u@example.com>"} {
		if err := ValidateAddress(addr); err == nil {
			t.Errorf("ValidateAddress(%q) = nil, want error", addr)
		}
	}
}

func TestDomains(t *testing.T) {
	got := Domains([]store.Mailbox{
		{Address: "a@x.com"}, {Address: "b@x.com"}, {Address: "c@y.org"},
	})
	if !reflect.DeepEqual(got, []string{"x.com", "y.org"}) {
		t.Errorf("Domains = %v", got)
	}
}

func TestRenderMaddyConfig(t *testing.T) {
	out := RenderMaddyConfig(
		[]store.Mailbox{{Address: "a@x.com"}},
		[]store.EmailForwarder{{From: "f@x.com", To: "t@y.com"}},
		"mail.x.com")
	for _, want := range []string{
		"$(hostname) = mail.x.com",
		"$(local_domains) = x.com",
		"smtp tcp://0.0.0.0:25",
		"f@x.com -> t@y.com",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("maddy config missing %q", want)
		}
	}
}
