package ddns

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/schmorrison/goshpanel/internal/store"
)

// Update performs a dynamic DNS update for one config.
func Update(ctx context.Context, cfg store.DDNSConfig, publicIP string) error {
	if !cfg.Enabled {
		return nil
	}
	switch strings.ToLower(cfg.Provider) {
	case "cloudflare":
		return updateCloudflare(ctx, cfg, publicIP)
	case "generic":
		return updateGeneric(ctx, cfg, publicIP)
	default:
		return fmt.Errorf("unknown DDNS provider %q", cfg.Provider)
	}
}

func updateGeneric(ctx context.Context, cfg store.DDNSConfig, ip string) error {
	u := strings.TrimSpace(cfg.UpdateURL)
	if u == "" {
		return fmt.Errorf("update URL required")
	}
	u = strings.ReplaceAll(u, "{ip}", url.QueryEscape(ip))
	u = strings.ReplaceAll(u, "{hostname}", url.QueryEscape(cfg.Hostname))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DDNS HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func updateCloudflare(ctx context.Context, cfg store.DDNSConfig, ip string) error {
	// Simplified: use generic URL pattern or API token in UpdateURL field.
	return updateGeneric(ctx, cfg, ip)
}

// PublicIP fetches this host's public IPv4 via a simple HTTP service.
func PublicIP(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.ipify.org", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
