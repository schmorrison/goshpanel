// Package secrets provides AES-GCM encryption for panel secrets at rest.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

// Vault encrypts and decrypts values with a master key.
type Vault struct {
	aead cipher.AEAD
}

// NewVault derives a 256-bit key from the master secret string.
func NewVault(master string) (*Vault, error) {
	if master == "" {
		return nil, fmt.Errorf("secrets master key required (GOSHPANEL_SECRETS_KEY)")
	}
	sum := sha256.Sum256([]byte(master))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Vault{aead: aead}, nil
}

// Encrypt returns base64(nonce|ciphertext).
func (v *Vault) Encrypt(plain string) (string, error) {
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := v.aead.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

// Decrypt reverses Encrypt.
func (v *Vault) Decrypt(encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	if len(raw) < v.aead.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ct := raw[:v.aead.NonceSize()], raw[v.aead.NonceSize():]
	plain, err := v.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
