package email

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/schmorrison/goshpanel/internal/crypto"
	"github.com/schmorrison/goshpanel/internal/store"
)

// Service manages mailbox definitions for SMTP integration.
type Service struct {
	repo          *store.MailboxRepository
	secret        string
	defaultDomain string
	configPath    string
}

// NewService creates an email management service.
func NewService(repo *store.MailboxRepository, secret, defaultDomain, configPath string) *Service {
	return &Service{
		repo:          repo,
		secret:        secret,
		defaultDomain: strings.TrimSpace(defaultDomain),
		configPath:    configPath,
	}
}

// List returns configured mailboxes.
func (s *Service) List(ctx context.Context) ([]store.Mailbox, error) {
	return s.repo.List(ctx)
}

// Create adds a mailbox.
func (s *Service) Create(ctx context.Context, localPart, domain, password, forwardTo string) (store.Mailbox, error) {
	localPart = strings.TrimSpace(localPart)
	domain = strings.TrimSpace(domain)
	forwardTo = strings.TrimSpace(forwardTo)
	if domain == "" {
		domain = s.defaultDomain
	}
	if localPart == "" || domain == "" || password == "" {
		return store.Mailbox{}, fmt.Errorf("local part, domain, and password are required")
	}

	encrypted, err := crypto.Encrypt(s.secret, password)
	if err != nil {
		return store.Mailbox{}, fmt.Errorf("encrypt password: %w", err)
	}

	return s.repo.Create(ctx, store.Mailbox{
		LocalPart:         localPart,
		Domain:            domain,
		PasswordEncrypted: encrypted,
		ForwardTo:         forwardTo,
		Enabled:           true,
	})
}

// Delete removes a mailbox.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

// ExportConfig writes a go-guerrilla-friendly JSON config to disk.
func (s *Service) ExportConfig(ctx context.Context) (string, error) {
	mailboxes, err := s.repo.List(ctx)
	if err != nil {
		return "", err
	}

	type exportedMailbox struct {
		Address  string `json:"address"`
		Password string `json:"password"`
		Forward  string `json:"forward,omitempty"`
		Enabled  bool   `json:"enabled"`
	}

	domains := map[string]struct{}{}
	exported := make([]exportedMailbox, 0, len(mailboxes))
	for _, mailbox := range mailboxes {
		password, err := crypto.Decrypt(s.secret, mailbox.PasswordEncrypted)
		if err != nil {
			return "", fmt.Errorf("decrypt mailbox password: %w", err)
		}
		domains[mailbox.Domain] = struct{}{}
		exported = append(exported, exportedMailbox{
			Address:  mailbox.LocalPart + "@" + mailbox.Domain,
			Password: password,
			Forward:  mailbox.ForwardTo,
			Enabled:  mailbox.Enabled,
		})
	}

	domainList := make([]string, 0, len(domains))
	for domain := range domains {
		domainList = append(domainList, domain)
	}

	payload := map[string]any{
		"domains":   domainList,
		"mailboxes": exported,
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(s.configPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(s.configPath, data, 0o600); err != nil {
		return "", err
	}

	return fmt.Sprintf("Exported %d mailbox(es) to %s", len(exported), s.configPath), nil
}

// Address returns the full mailbox address.
func Address(mailbox store.Mailbox) string {
	return mailbox.LocalPart + "@" + mailbox.Domain
}
