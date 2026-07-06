package crypto

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("s3cret-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !VerifyPassword(hash, "s3cret-password") {
		t.Error("correct password rejected")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Error("wrong password accepted")
	}
	if VerifyPassword("garbage", "s3cret-password") {
		t.Error("malformed hash accepted")
	}
}

func TestHashPasswordUniqueSalts(t *testing.T) {
	h1, _ := HashPassword("same")
	h2, _ := HashPassword("same")
	if h1 == h2 {
		t.Error("two hashes of the same password are identical (salt reuse)")
	}
}

func TestRandomToken(t *testing.T) {
	t1, err := RandomToken()
	if err != nil {
		t.Fatalf("RandomToken: %v", err)
	}
	t2, _ := RandomToken()
	if t1 == t2 {
		t.Error("tokens are not unique")
	}
	if len(t1) < 40 {
		t.Errorf("token too short: %d chars", len(t1))
	}
}
