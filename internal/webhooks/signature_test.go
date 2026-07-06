package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestVerifySignature(t *testing.T) {
	body := []byte(`{"event":"test"}`)
	secret := "s3cret"
	mac := hmacDigest(secret, body)
	if !VerifySignature(secret, body, mac) {
		t.Fatal("expected valid signature")
	}
	if VerifySignature(secret, body, "bad") {
		t.Fatal("expected invalid signature")
	}
}

func hmacDigest(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
