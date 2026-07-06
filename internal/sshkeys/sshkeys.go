package sshkeys

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/schmorrison/goshpanel/internal/store"
)

// RenderAuthorizedKeys writes authorized_keys content for a user.
func RenderAuthorizedKeys(keys []store.SSHKey) string {
	var b strings.Builder
	for _, k := range keys {
		line := strings.TrimSpace(k.PublicKey)
		if line == "" {
			continue
		}
		if k.Comment != "" {
			line += " " + k.Comment
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// Apply writes authorized_keys for username under sshDir (e.g. /home/user/.ssh).
func Apply(sshDir string, username string, keys []store.SSHKey) error {
	dir := filepath.Join(sshDir, username, ".ssh")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create .ssh: %w", err)
	}
	path := filepath.Join(dir, "authorized_keys")
	content := RenderAuthorizedKeys(keys)
	return os.WriteFile(path, []byte(content), 0o600)
}
