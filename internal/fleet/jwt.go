package fleet

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const enrollIssuer = "goshpanel-fleet"

// enrollClaims are HMAC-signed JWT claims for one-time fleet enrollment.
type enrollClaims struct {
	Iss  string `json:"iss"`
	Exp  int64  `json:"exp"`
	Iat  int64  `json:"iat"`
	Name string `json:"name,omitempty"`
}

// SignEnrollToken returns a compact HMAC-SHA256 JWT for fleet enrollment.
func SignEnrollToken(secret string, ttl time.Duration, suggestedName string) (string, error) {
	if secret == "" {
		return "", errors.New("enroll secret required")
	}
	now := time.Now().UTC()
	claims := enrollClaims{
		Iss:  enrollIssuer,
		Iat:  now.Unix(),
		Exp:  now.Add(ttl).Unix(),
		Name: strings.TrimSpace(suggestedName),
	}
	return signJWT(secret, claims)
}

// VerifyEnrollToken validates an enrollment JWT and returns its claims.
func VerifyEnrollToken(secret, token string) (enrollClaims, error) {
	if secret == "" {
		return enrollClaims{}, errors.New("enroll secret required")
	}
	var claims enrollClaims
	if err := verifyJWT(secret, token, &claims); err != nil {
		return enrollClaims{}, err
	}
	if claims.Iss != enrollIssuer {
		return enrollClaims{}, errors.New("invalid enroll token issuer")
	}
	now := time.Now().UTC().Unix()
	if claims.Exp < now {
		return enrollClaims{}, errors.New("enroll token expired")
	}
	if claims.Iat > now+60 {
		return enrollClaims{}, errors.New("enroll token not yet valid")
	}
	return claims, nil
}

func signJWT(secret string, claims any) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	signingInput := header + "." + encodedPayload
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(signingInput)); err != nil {
		return "", err
	}
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signingInput + "." + sig, nil
}

func verifyJWT(secret, token string, claims any) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return errors.New("malformed JWT")
	}
	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(signingInput)); err != nil {
		return err
	}
	wantSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return errors.New("invalid JWT signature encoding")
	}
	if !hmac.Equal(mac.Sum(nil), wantSig) {
		return errors.New("invalid enroll token signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("decode JWT payload: %w", err)
	}
	if err := json.Unmarshal(payload, claims); err != nil {
		return fmt.Errorf("decode JWT claims: %w", err)
	}
	return nil
}
