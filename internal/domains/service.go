package domains

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/schmorrison/goshpanel/internal/store"
)

// Service manages virtual hosts and optional Caddy integration.
type Service struct {
	sites      *store.SiteRepository
	adminURL   string
	configPath string
	client     *http.Client
}

// NewService creates a domains service.
func NewService(sites *store.SiteRepository, adminURL, configPath string) *Service {
	return &Service{
		sites:      sites,
		adminURL:   strings.TrimRight(adminURL, "/"),
		configPath: configPath,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// List returns configured sites.
func (s *Service) List(ctx context.Context) ([]store.Site, error) {
	return s.sites.List(ctx)
}

// Create registers a new site.
func (s *Service) Create(ctx context.Context, domain, upstream string) (store.Site, error) {
	domain = strings.TrimSpace(domain)
	upstream = strings.TrimSpace(upstream)
	if domain == "" || upstream == "" {
		return store.Site{}, fmt.Errorf("domain and upstream are required")
	}
	return s.sites.Create(ctx, domain, upstream)
}

// Delete removes a site.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.sites.Delete(ctx, id)
}

// RenderCaddyfile builds a Caddyfile from enabled sites.
func (s *Service) RenderCaddyfile(ctx context.Context) (string, error) {
	sites, err := s.sites.List(ctx)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	for _, site := range sites {
		if !site.Enabled {
			continue
		}
		fmt.Fprintf(&buf, "%s {\n    reverse_proxy %s\n}\n\n", site.Domain, site.Upstream)
	}
	return buf.String(), nil
}

// Apply writes the generated Caddyfile and optionally reloads Caddy.
func (s *Service) Apply(ctx context.Context) (string, error) {
	content, err := s.RenderCaddyfile(ctx)
	if err != nil {
		return "", err
	}

	if s.configPath != "" {
		if err := os.MkdirAll(filepath.Dir(s.configPath), 0o755); err != nil {
			return "", fmt.Errorf("create config directory: %w", err)
		}
		if err := os.WriteFile(s.configPath, []byte(content), 0o644); err != nil {
			return "", fmt.Errorf("write caddyfile: %w", err)
		}
	}

	if s.adminURL == "" {
		return "Caddyfile written to disk. Configure caddy.admin_url to push live reloads.", nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.adminURL+"/load", bytes.NewReader([]byte(content)))
	if err != nil {
		return "", fmt.Errorf("create caddy reload request: %w", err)
	}
	req.Header.Set("Content-Type", "text/caddyfile")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("reload caddy: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("caddy reload failed: %s", strings.TrimSpace(string(body)))
	}

	return "Caddy configuration applied successfully.", nil
}
