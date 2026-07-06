// Package orchestrator writes generated configs from panel state and
// reloads Go-native daemons (Caddy, CoreDNS, maddy) plus systemd units
// via thin process calls.
package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/schmorrison/goshpanel/internal/connector"
	"github.com/schmorrison/goshpanel/internal/dns"
	"github.com/schmorrison/goshpanel/internal/domains"
	"github.com/schmorrison/goshpanel/internal/store"
)

// Paths holds on-disk locations for generated configs.
type Paths struct {
	CaddyConfig     string
	CaddyAccessLog  string
	CoreDNSDir      string
	MaddyConfig     string
	SystemdDir      string
}

// Service regenerates and applies infrastructure configs.
type Service struct {
	store    *store.Store
	paths    Paths
	registry *connector.Registry
}

// New creates an orchestrator service.
func New(st *store.Store, paths Paths, registry *connector.Registry) *Service {
	return &Service{store: st, paths: paths, registry: registry}
}

// Result describes one apply operation.
type Result struct {
	Component  string
	OK         bool
	Message    string
	ConfigPath string
}

// ApplyAll regenerates and reloads every managed component.
func (s *Service) ApplyAll(ctx context.Context) []Result {
	var out []Result
	out = append(out, s.ApplyCaddy(ctx))
	out = append(out, s.ApplyCoreDNS(ctx))
	out = append(out, s.ApplyMaddy(ctx))
	out = append(out, s.ApplySystemd(ctx))
	return out
}

// ApplyCaddy writes the Caddyfile and reloads caddy.
func (s *Service) ApplyCaddy(ctx context.Context) Result {
	res := Result{Component: "caddy", ConfigPath: s.paths.CaddyConfig}
	domainsList, err := s.store.Domains()
	if err != nil {
		res.Message = err.Error()
		s.record("caddy", res.ConfigPath, res.Message)
		return res
	}
	ctxData := s.caddyContext(domainsList)
	content := domains.RenderCaddyfileFull(ctxData)

	if s.registry != nil {
		conn, paths, err := s.registry.EffectiveCaddy()
		if err != nil {
			res.Message = err.Error()
			s.record("caddy", res.ConfigPath, res.Message)
			return res
		}
		client, err := s.registry.Caddy(conn, paths)
		if err != nil {
			res.Message = err.Error()
			s.record("caddy", res.ConfigPath, res.Message)
			return res
		}
		res.ConfigPath = client.ConfigPath()
		if err := client.WriteAndReload(ctx, content); err != nil {
			res.Message = err.Error()
			s.record("caddy", res.ConfigPath, res.Message)
			return res
		}
	} else {
		if err := domains.WriteCaddyfileFull(ctxData, s.paths.CaddyConfig); err != nil {
			res.Message = err.Error()
			s.record("caddy", res.ConfigPath, res.Message)
			return res
		}
		if err := domains.ApplyCaddyfile(ctx, s.paths.CaddyConfig); err != nil {
			res.Message = err.Error()
			s.record("caddy", res.ConfigPath, res.Message)
			return res
		}
	}
	res.OK = true
	res.Message = fmt.Sprintf("reloaded %d domain(s)", len(domainsList))
	s.record("caddy", res.ConfigPath, "")
	return res
}

// ApplyCoreDNS writes zone files and a Corefile, then reloads CoreDNS.
func (s *Service) ApplyCoreDNS(ctx context.Context) Result {
	res := Result{Component: "coredns", ConfigPath: filepath.Join(s.paths.CoreDNSDir, "Corefile")}
	bundle, err := s.buildCoreDNSBundle()
	if err != nil {
		res.Message = err.Error()
		s.record("coredns", res.ConfigPath, res.Message)
		return res
	}

	if s.registry != nil {
		conn, paths, err := s.registry.EffectiveCoreDNS()
		if err != nil {
			res.Message = err.Error()
			s.record("coredns", res.ConfigPath, res.Message)
			return res
		}
		client, err := s.registry.CoreDNS(conn, paths)
		if err != nil {
			res.Message = err.Error()
			s.record("coredns", res.ConfigPath, res.Message)
			return res
		}
		res.ConfigPath = client.ConfigPath()
		if err := client.WriteBundle(ctx, bundle); err != nil {
			res.Message = err.Error()
			s.record("coredns", res.ConfigPath, res.Message)
			return res
		}
	} else {
		zonesDir := filepath.Join(s.paths.CoreDNSDir, "zones")
		if err := os.MkdirAll(zonesDir, 0o755); err != nil {
			res.Message = err.Error()
			s.record("coredns", res.ConfigPath, res.Message)
			return res
		}
		for name, content := range bundle.Zones {
			if err := os.WriteFile(filepath.Join(zonesDir, name), []byte(content), 0o644); err != nil {
				res.Message = err.Error()
				s.record("coredns", res.ConfigPath, res.Message)
				return res
			}
		}
		if err := os.WriteFile(res.ConfigPath, []byte(bundle.Corefile), 0o644); err != nil {
			res.Message = err.Error()
			s.record("coredns", res.ConfigPath, res.Message)
			return res
		}
		if err := reloadBinary(ctx, "coredns", []string{"-conf", res.ConfigPath}); err != nil {
			res.Message = err.Error()
			s.record("coredns", res.ConfigPath, res.Message)
			return res
		}
	}
	res.OK = true
	res.Message = fmt.Sprintf("wrote %d zone(s)", len(bundle.Zones))
	s.record("coredns", res.ConfigPath, "")
	return res
}

// ApplyMaddy writes maddy.conf and reloads maddy.
func (s *Service) ApplyMaddy(ctx context.Context) Result {
	res := Result{Component: "maddy", ConfigPath: s.paths.MaddyConfig}
	hostname, _ := os.Hostname()
	content, err := connector.RenderMaddyFromStore(s.store, hostname)
	if err != nil {
		res.Message = err.Error()
		s.record("maddy", res.ConfigPath, res.Message)
		return res
	}

	if s.registry != nil {
		conn, paths, err := s.registry.EffectiveMaddy()
		if err != nil {
			res.Message = err.Error()
			s.record("maddy", res.ConfigPath, res.Message)
			return res
		}
		client, err := s.registry.Maddy(conn, paths)
		if err != nil {
			res.Message = err.Error()
			s.record("maddy", res.ConfigPath, res.Message)
			return res
		}
		res.ConfigPath = client.ConfigPath()
		if err := client.WriteAndReload(ctx, content); err != nil {
			res.Message = err.Error()
			s.record("maddy", res.ConfigPath, res.Message)
			return res
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(s.paths.MaddyConfig), 0o755); err != nil {
			res.Message = err.Error()
			s.record("maddy", res.ConfigPath, res.Message)
			return res
		}
		if err := os.WriteFile(s.paths.MaddyConfig, []byte(content), 0o644); err != nil {
			res.Message = err.Error()
			s.record("maddy", res.ConfigPath, res.Message)
			return res
		}
		if err := reloadBinary(ctx, "maddy", []string{"reload", "-config", s.paths.MaddyConfig}); err != nil {
			res.Message = "config written; reload skipped: " + err.Error()
			s.record("maddy", res.ConfigPath, res.Message)
			return res
		}
	}
	boxes, _ := s.store.Mailboxes()
	res.OK = true
	res.Message = fmt.Sprintf("reloaded (%d mailboxes)", len(boxes))
	s.record("maddy", res.ConfigPath, "")
	return res
}

func (s *Service) buildCoreDNSBundle() (connector.CoreDNSBundle, error) {
	zonesDir := filepath.Join(s.paths.CoreDNSDir, "zones")
	domainList, err := s.store.Domains()
	if err != nil {
		return connector.CoreDNSBundle{}, err
	}
	bundle := connector.CoreDNSBundle{Zones: map[string]string{}}
	var corefile strings.Builder
	corefile.WriteString("# Generated by GoshPanel — do not edit by hand.\n")
	for _, d := range domainList {
		recs, err := s.store.DNSRecords(d.ID)
		if err != nil {
			return connector.CoreDNSBundle{}, err
		}
		filename := "db." + d.Name
		bundle.Zones[filename] = dns.RenderZone(d, recs)
		zonePath := filepath.Join(zonesDir, filename)
		fmt.Fprintf(&corefile, "%s {\n\tfile %s %s\n\tlog\n\terrors\n}\n\n", d.Name, zonePath, d.Name)
	}
	if len(domainList) == 0 {
		corefile.WriteString(". {\n\tlog\n\terrors\n\twhoami\n}\n")
	}
	bundle.Corefile = corefile.String()
	return bundle, nil
}

// ApplySystemd writes unit files and runs systemctl daemon-reload + restart.
func (s *Service) ApplySystemd(ctx context.Context) Result {
	res := Result{Component: "systemd", ConfigPath: s.paths.SystemdDir}
	units, err := s.store.SystemdUnits()
	if err != nil {
		res.Message = err.Error()
		s.record("systemd", res.ConfigPath, res.Message)
		return res
	}
	if err := os.MkdirAll(s.paths.SystemdDir, 0o755); err != nil {
		res.Message = err.Error()
		s.record("systemd", res.ConfigPath, res.Message)
		return res
	}
	for _, u := range units {
		name := u.Name
		if !strings.HasSuffix(name, ".service") {
			name += ".service"
		}
		path := filepath.Join(s.paths.SystemdDir, name)
		if err := os.WriteFile(path, []byte(u.UnitContent), 0o644); err != nil {
			res.Message = err.Error()
			s.record("systemd", res.ConfigPath, res.Message)
			return res
		}
	}

	systemctl, err := exec.LookPath("systemctl")
	if err != nil {
		res.Message = "systemctl not found; unit files written to " + s.paths.SystemdDir
		s.record("systemd", res.ConfigPath, res.Message)
		return res
	}
	if out, err := exec.CommandContext(ctx, systemctl, "daemon-reload").CombinedOutput(); err != nil {
		res.Message = fmt.Sprintf("daemon-reload: %s: %v", strings.TrimSpace(string(out)), err)
		s.record("systemd", res.ConfigPath, res.Message)
		return res
	}
	restarted := 0
	for _, u := range units {
		if !u.Enabled {
			continue
		}
		name := u.Name
		if !strings.HasSuffix(name, ".service") {
			name += ".service"
		}
		// Try user units first if writing to user dir, else system units.
		args := []string{"restart", name}
		if out, err := exec.CommandContext(ctx, systemctl, args...).CombinedOutput(); err != nil {
			res.Message = fmt.Sprintf("restart %s: %s: %v", name, strings.TrimSpace(string(out)), err)
			s.record("systemd", res.ConfigPath, res.Message)
			return res
		}
		restarted++
	}
	res.OK = true
	res.Message = fmt.Sprintf("applied %d unit(s), restarted %d", len(units), restarted)
	s.record("systemd", res.ConfigPath, "")
	return res
}

func reloadBinary(ctx context.Context, name string, args []string) error {
	bin, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("%s binary not found in PATH; config written for manual reload", name)
	}
	out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s reload failed: %s: %w", name, strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (s *Service) record(component, path, errMsg string) {
	_ = s.store.SetOrchestratorStatus(component, path, errMsg, time.Now())
}

func (s *Service) webmailSettings() store.WebmailSettings {
	ws, err := s.store.WebmailSettings()
	if err != nil {
		return store.WebmailSettings{}
	}
	return ws
}

func (s *Service) caddyContext(domainsList []store.Domain) domains.CaddyContext {
	redirects, _ := s.store.RedirectRules()
	aliases, _ := s.store.DomainAliases()
	waf, _ := s.store.WAFSites()
	shortLinks, _ := s.store.ShortLinks()
	return domains.CaddyContext{
		Domains:       domainsList,
		AccessLogPath: s.paths.CaddyAccessLog,
		Webmail:       s.webmailSettings(),
		Redirects:     redirects,
		Aliases:       aliases,
		WAFSites:      waf,
		ShortLinks:    shortLinks,
	}
}
