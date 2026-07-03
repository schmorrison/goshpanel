// Package security implements the panel-level IP blocker (cPanel "IP
// Blocker" equivalent) as an HTTP middleware concern plus CIDR helpers.
package security

import (
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"

	"github.com/schmorrison/goshpanel/internal/store"
)

// NormalizeCIDR accepts "1.2.3.4" or "1.2.3.0/24" and returns canonical
// CIDR notation.
func NormalizeCIDR(input string) (string, error) {
	input = strings.TrimSpace(input)
	if !strings.Contains(input, "/") {
		addr, err := netip.ParseAddr(input)
		if err != nil {
			return "", fmt.Errorf("invalid IP address %q", input)
		}
		bits := 32
		if addr.Is6() {
			bits = 128
		}
		input = fmt.Sprintf("%s/%d", addr, bits)
	}
	prefix, err := netip.ParsePrefix(input)
	if err != nil {
		return "", fmt.Errorf("invalid CIDR %q", input)
	}
	return prefix.Masked().String(), nil
}

// Blocker answers "is this remote address denied?" using rules from the
// store, cached until Reload is called.
type Blocker struct {
	store *store.Store

	mu       sync.RWMutex
	prefixes []netip.Prefix
}

// NewBlocker builds a blocker and loads current rules.
func NewBlocker(st *store.Store) (*Blocker, error) {
	b := &Blocker{store: st}
	if err := b.Reload(); err != nil {
		return nil, err
	}
	return b, nil
}

// Reload refreshes the in-memory rule set from the store.
func (b *Blocker) Reload() error {
	rules, err := b.store.IPRules()
	if err != nil {
		return err
	}
	prefixes := make([]netip.Prefix, 0, len(rules))
	for _, r := range rules {
		p, err := netip.ParsePrefix(r.CIDR)
		if err != nil {
			continue // ignore malformed stored rules
		}
		prefixes = append(prefixes, p)
	}
	b.mu.Lock()
	b.prefixes = prefixes
	b.mu.Unlock()
	return nil
}

// Blocked reports whether remoteAddr ("ip:port" or bare IP) is denied.
func (b *Blocker) Blocked(remoteAddr string) bool {
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, p := range b.prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}
