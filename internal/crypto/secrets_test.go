package crypto

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	secret := "test-secret-material"
	plaintext := "super-secret-password"

	encoded, err := Encrypt(secret, plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	got, err := Decrypt(secret, encoded)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if got != plaintext {
		t.Fatalf("Decrypt() = %q, want %q", got, plaintext)
	}
}
