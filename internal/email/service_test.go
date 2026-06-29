package email

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/schmorrison/goshpanel/internal/store"
)

func TestCreateAndExportMailboxes(t *testing.T) {
	ctx := context.Background()
	db := store.OpenTest(t)
	repo := store.NewMailboxRepository(db)
	path := t.TempDir() + "/guerrilla.json"
	svc := NewService(repo, "test-secret", "example.com", path)

	if _, err := svc.Create(ctx, "hello", "example.com", "mailbox-pass", ""); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	message, err := svc.ExportConfig(ctx)
	if err != nil {
		t.Fatalf("ExportConfig() error = %v", err)
	}
	if message == "" {
		t.Fatal("ExportConfig() returned empty message")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "hello@example.com") {
		t.Fatalf("export missing mailbox address: %s", data)
	}
}
