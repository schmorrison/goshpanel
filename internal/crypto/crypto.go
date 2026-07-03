// Package crypto provides password hashing (argon2id) and random token
// generation. golang.org/x/crypto/argon2 is pure Go — no cgo.
package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// argon2id parameters per OWASP recommendations.
const (
	timeCost   = 3
	memoryKiB  = 64 * 1024
	threads    = 4
	saltLen    = 16
	keyLen     = 32
	hashPrefix = "argon2id"
)

// HashPassword derives a salted argon2id hash encoded as
// "argon2id$<time>$<memoryKiB>$<threads>$<salt-b64>$<key-b64>".
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, timeCost, memoryKiB, threads, keyLen)
	return fmt.Sprintf("%s$%d$%d$%d$%s$%s",
		hashPrefix, timeCost, memoryKiB, threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// VerifyPassword reports whether password matches the encoded hash.
func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != hashPrefix {
		return false
	}
	var t, m, p uint32
	if _, err := fmt.Sscanf(parts[1]+" "+parts[2]+" "+parts[3], "%d %d %d", &t, &m, &p); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, t, m, uint8(p), uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// RandomToken returns a URL-safe random token with 256 bits of entropy.
func RandomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
