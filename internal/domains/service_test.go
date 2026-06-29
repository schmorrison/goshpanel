package domains

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/schmorrison/goshpanel/internal/store"
)

func TestRenderAndApplyCaddyfile(t *testing.T) {
	db := store.OpenTest(t)
	sites := store.NewSiteRepository(db)
	svc := NewService(sites, "", filepath.Join(t.TempDir(), "Caddyfile"))

	ctx := context.Background()
	if _, err := svc.Create(ctx, "example.com", "localhost:8080"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	content, err := svc.RenderCaddyfile(ctx)
	if err != nil {
		t.Fatalf("RenderCaddyfile() error = %v", err)
	}
	if !strings.Contains(content, "example.com") || !strings.Contains(content, "localhost:8080") {
		t.Fatalf("RenderCaddyfile() = %q", content)
	}

	message, err := svc.Apply(ctx)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if message == "" {
		t.Fatal("Apply() returned empty message")
	}

	data, err := os.ReadFile(svc.configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != content {
		t.Fatalf("written caddyfile mismatch")
	}
}
