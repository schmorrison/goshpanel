package installers

import (
	"strings"
	"testing"
)

func TestStackName(t *testing.T) {
	got := StackName("wordpress", "blog.example.com")
	if got != "wordpress-blog-example-com" {
		t.Errorf("StackName = %q", got)
	}
}

func TestComposeYAML(t *testing.T) {
	for _, id := range []string{"wordpress", "ghost", "gitea"} {
		yaml, err := ComposeYAML(id, "app.example.com")
		if err != nil {
			t.Fatalf("ComposeYAML(%q): %v", id, err)
		}
		if !strings.Contains(yaml, "services:") {
			t.Errorf("%s yaml missing services block:\n%s", id, yaml)
		}
	}
	if _, err := ComposeYAML("unknown", "x.com"); err == nil {
		t.Error("unknown installer should error")
	}
}

func TestUpstreamHostPort(t *testing.T) {
	spec, _ := SpecByID("wordpress")
	if UpstreamHostPort(spec) != "localhost:8080" {
		t.Errorf("upstream = %s", UpstreamHostPort(spec))
	}
}

func TestDatabaseConnInfo(t *testing.T) {
	info, ok := DatabaseConnInfo("wordpress", "blog.example.com")
	if !ok || info.Port != 13306 || info.Kind != "mysql" {
		t.Fatalf("DatabaseConnInfo = %+v, ok=%v", info, ok)
	}
	yaml, err := ComposeYAML("wordpress", "blog.example.com")
	if err != nil || !strings.Contains(yaml, "13306:3306") {
		t.Fatalf("wordpress compose should publish mysql: %v\n%s", err, yaml)
	}
}
