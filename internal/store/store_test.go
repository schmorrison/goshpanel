package store

import (
	"context"
	"testing"
	"time"
)

func TestUserRepositoryCreateAndFind(t *testing.T) {
	db := OpenTest(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user, err := repo.Create(ctx, "admin", "hashed-password")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if user.Username != "admin" {
		t.Fatalf("Username = %q, want %q", user.Username, "admin")
	}

	found, err := repo.FindByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("FindByUsername() error = %v", err)
	}
	if found.ID != user.ID {
		t.Fatalf("ID = %d, want %d", found.ID, user.ID)
	}

	count, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("Count() = %d, want 1", count)
	}
}

func TestSessionStoreRoundTrip(t *testing.T) {
	db := OpenTest(t)
	store := NewSessionStore(db)

	data := []byte("session-data")
	expiry := storeExpiry()
	if err := store.Commit("token-1", data, expiry); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	got, found, err := store.Find("token-1")
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if !found {
		t.Fatal("Find() found = false, want true")
	}
	if string(got) != string(data) {
		t.Fatalf("Find() data = %q, want %q", got, data)
	}

	if err := store.Delete("token-1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, found, err = store.Find("token-1")
	if err != nil {
		t.Fatalf("Find() after delete error = %v", err)
	}
	if found {
		t.Fatal("Find() after delete found = true, want false")
	}
}

func storeExpiry() time.Time {
	return time.Now().Add(time.Hour)
}
