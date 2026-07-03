package auth

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"

	"github.com/pquerna/otp/totp"
)

// GenerateTOTPSecret returns a new base32-encoded TOTP secret.
func GenerateTOTPSecret() (string, error) {
	secret := make([]byte, 20)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("generate totp secret: %w", err)
	}
	return strings.TrimRight(base32.StdEncoding.EncodeToString(secret), "="), nil
}

// TOTPProvisioningURI returns an otpauth:// URI for authenticator apps.
func TOTPProvisioningURI(secret, account, issuer string) string {
	return fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s&digits=6",
		issuer, account, secret, issuer)
}

// ValidateTOTP checks a 6-digit code against a secret.
func ValidateTOTP(secret, code string) bool {
	code = strings.TrimSpace(code)
	if secret == "" || code == "" {
		return false
	}
	return totp.Validate(code, secret)
}
