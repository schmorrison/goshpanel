// Package shortener generates and validates short link codes.
package shortener

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"regexp"
	"strings"
)

var codeRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{2,32}$`)

// ErrInvalidCode is returned for bad short codes.
var ErrInvalidCode = errors.New("code must be 2-32 alphanumeric characters")

// ErrInvalidURL is returned for bad target URLs.
var ErrInvalidURL = errors.New("target must be an http or https URL")

// NormalizeCode validates or generates a short code.
func NormalizeCode(code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return RandomCode(6)
	}
	if !codeRe.MatchString(code) {
		return "", ErrInvalidCode
	}
	return code, nil
}

// ValidateTarget ensures the destination URL is usable.
func ValidateTarget(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", ErrInvalidURL
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", ErrInvalidURL
	}
	return raw, nil
}

// RandomCode returns a URL-safe random code of n characters.
func RandomCode(n int) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	if n < 2 {
		n = 6
	}
	b := make([]byte, n)
	for i := range b {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", fmt.Errorf("random code: %w", err)
		}
		b[i] = alphabet[idx.Int64()]
	}
	return string(b), nil
}
