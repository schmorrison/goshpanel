package dkim

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"
)

// KeyPair is a generated DKIM signing key.
type KeyPair struct {
	Selector   string
	PrivatePEM string
	PublicDNS  string // TXT record value
}

// Generate creates a 2048-bit RSA DKIM keypair.
func Generate(selector, domain string) (KeyPair, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		selector = "goshpanel"
	}
	domain = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return KeyPair{}, err
	}
	privDER := x509.MarshalPKCS1PrivateKey(key)
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privDER})
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return KeyPair{}, err
	}
	pubB64 := base64.StdEncoding.EncodeToString(pubDER)
	pubDNS := fmt.Sprintf("v=DKIM1; k=rsa; p=%s", pubB64)
	return KeyPair{
		Selector:   selector,
		PrivatePEM: string(privPEM),
		PublicDNS:  pubDNS,
	}, nil
}
