package domains

import (
	"strings"
	"testing"

	"github.com/schmorrison/goshpanel/internal/store"
)

func TestValidateName(t *testing.T) {
	for _, name := range []string{"example.com", "sub.example.co.uk", "a-b.example.io"} {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", name, err)
		}
	}
	for _, name := range []string{"", "nodot", "-bad.com", "bad-.com", "spaces here.com", "exa_mple.com"} {
		if err := ValidateName(name); err == nil {
			t.Errorf("ValidateName(%q) = nil, want error", name)
		}
	}
}

func TestRenderCaddyfile(t *testing.T) {
	out := RenderCaddyfile([]store.Domain{
		{Name: "static.example.com", Root: "/srv/static"},
		{Name: "app.example.com", Upstream: "localhost:3000"},
		{Name: "both.example.com", Root: "/srv/both", Upstream: "localhost:4000"},
	}, "/var/log/caddy/access.log", store.WebmailSettings{})
	for _, want := range []string{
		"format json",
		"/var/log/caddy/access.log",
		"static.example.com {",
		"root * /srv/static",
		"file_server",
		"app.example.com {",
		"reverse_proxy localhost:3000",
		"reverse_proxy /api/* localhost:4000",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Caddyfile missing %q:\n%s", want, out)
		}
	}
}

func TestRenderCaddyfileWebmail(t *testing.T) {
	out := RenderCaddyfile(nil, "", store.WebmailSettings{
		Enabled: true, Host: "webmail.example.com", Upstream: "localhost:8080",
	})
	if !strings.Contains(out, "webmail.example.com {") || !strings.Contains(out, "reverse_proxy localhost:8080") {
		t.Errorf("webmail block missing:\n%s", out)
	}
}

func TestRenderCaddyfileShortLinks(t *testing.T) {
	out := RenderCaddyfileFull(CaddyContext{
		ShortLinks: []store.ShortLink{
			{Host: "go.example.com", Code: "docs", TargetURL: "https://docs.example.com"},
		},
	})
	if !strings.Contains(out, "go.example.com {") || !strings.Contains(out, "redir /docs https://docs.example.com 302") {
		t.Errorf("short link block missing:\n%s", out)
	}
}
