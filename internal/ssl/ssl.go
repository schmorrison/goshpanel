// Package ssl scans PEM certificate files and reports expiry — the cPanel
// SSL/TLS status equivalent. Uses crypto/x509 only (pure Go).
package ssl

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CertInfo describes one parsed certificate.
type CertInfo struct {
	File       string
	Subject    string
	Issuer     string
	NotBefore  time.Time
	NotAfter   time.Time
	DNSNames   []string
	DaysLeft   int
	Expired    bool
	ExpiringSoon bool // within 30 days
}

// ScanDir reads all .pem/.crt files in dir (non-recursive).
func ScanDir(dir string) ([]CertInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read cert dir: %w", err)
	}
	var out []CertInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		if !strings.HasSuffix(name, ".pem") && !strings.HasSuffix(name, ".crt") && !strings.HasSuffix(name, ".cer") {
			continue
		}
		certs, err := parseFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue // skip unreadable
		}
		out = append(out, certs...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NotAfter.Before(out[j].NotAfter) })
	return out, nil
}

func parseFile(path string) ([]CertInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []CertInfo
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		days := int(time.Until(cert.NotAfter).Hours() / 24)
		out = append(out, CertInfo{
			File:         path,
			Subject:      cert.Subject.CommonName,
			Issuer:       cert.Issuer.CommonName,
			NotBefore:    cert.NotBefore,
			NotAfter:     cert.NotAfter,
			DNSNames:     cert.DNSNames,
			DaysLeft:     days,
			Expired:      time.Now().After(cert.NotAfter),
			ExpiringSoon: days >= 0 && days <= 30,
		})
	}
	return out, nil
}
