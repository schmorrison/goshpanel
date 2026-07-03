# GoshPanel

GoshPanel is a Linux web-hosting control panel written in Go, refactored from first principles to be a practical cPanel alternative. It compiles to a single static binary with all assets embedded — no cgo, no Node build step, no config files.

See [docs/FEATURES.md](docs/FEATURES.md) for the full cPanel feature-parity matrix.

## Design principles

- **High-purity Go.** Standard library first: `net/http` (Go 1.22 pattern routing), `html/template`, `archive/tar`, `net/netip`. External dependencies are pure Go only — `modernc.org/sqlite` (no cgo), `golang.org/x/crypto` (argon2id), and the MySQL/PostgreSQL drivers.
- **Process calls over embedding.** Heavy infrastructure is delegated to Go-native daemons via generated config and thin `exec` calls: Caddy (web serving + automatic HTTPS), CoreDNS (DNS), maddy (SMTP/IMAP), `crontab`, `bash`.
- **One binary, one database.** Panel state lives in a single SQLite file. Templates and CSS are embedded with `go:embed`.

## Modules

| Module | Route | What it does |
|---|---|---|
| Dashboard | `/` | Load, memory, disk, uptime, host info from `/proc` |
| Files | `/files` | Sandboxed file manager: browse, upload, edit, rename, delete, download |
| Domains & DNS | `/domains` | Site definitions, DNS zone editor, Caddyfile + RFC 1035 zone file export |
| Databases | `/databases` | Saved MySQL/PostgreSQL/SQLite connections, table browser, read-only SQL console |
| Email | `/email` | Mailboxes (argon2id hashes, quotas), forwarders, maddy config export |
| Cron | `/cron` | Validated cron jobs, crontab export, one-click `crontab` install |
| Backups | `/backups` | tar.gz create/download/restore/delete of the files sandbox |
| Logs | `/logs` | Tail viewer over an allowlist of log files |
| Terminal | `/terminal` | One-shot `bash -c` command runner with timeout |
| Security | `/security` | Panel users & roles, password changes, IP blocker, audit log |
| Orchestrator | `/orchestrator` | Live apply: Caddy, CoreDNS, maddy reload + systemd units |
| Docker | `/docker` | Container/image management, logs, `docker run`, compose stacks |
| Functions | `/functions` | HTTP-triggered micro functions (bash scripts at `/fn/{name}`) |
| Fleet | `/fleet` | Multi-instance telemetry and remote control (controller/worker) |
| Metrics | `/metrics` | Local host metrics history (load, memory, disk) |
| SSL / TLS | `/ssl` | PEM certificate scanner with expiry status |

## Quick start

Requires Go 1.25+ (older toolchains auto-download it via the `go` directive).

```bash
go build -o goshpanel ./cmd/goshpanel
GOSHPANEL_BOOTSTRAP_PASSWORD=yourpassword ./goshpanel
```

Open http://localhost:4674 and sign in with `admin` / `yourpassword`.

## Configuration

Everything is an environment variable:

| Variable | Default | Purpose |
|---|---|---|
| `GOSHPANEL_ADDR` | `:4674` | HTTP listen address |
| `GOSHPANEL_DATA_DIR` | `data` | SQLite DB, backups, generated configs |
| `GOSHPANEL_FILES_ROOT` | `data/workspace` | File manager sandbox root |
| `GOSHPANEL_BOOTSTRAP_USER` | `admin` | First-run admin username |
| `GOSHPANEL_BOOTSTRAP_PASSWORD` | — | First-run admin password (required until a user exists) |
| `GOSHPANEL_SESSION_TTL_MINUTES` | `720` | Login session lifetime |
| `GOSHPANEL_LOG_SOURCES` | — | Comma-separated allowlist of log files for the viewer |
| `GOSHPANEL_COMMAND_RUNNER` | `true` | Enable/disable the terminal module |
| `GOSHPANEL_ORCHESTRATOR` | `true` | Enable live config apply (Caddy/CoreDNS/maddy/systemd) |
| `GOSHPANEL_AUTO_APPLY` | `false` | Auto-reload daemons after domain/DNS/email changes |
| `GOSHPANEL_CADDY_CONFIG` | `data/generated/Caddyfile` | Caddyfile output path |
| `GOSHPANEL_COREDNS_DIR` | `data/generated/coredns` | CoreDNS config directory |
| `GOSHPANEL_MADDY_CONFIG` | `data/generated/maddy.conf` | maddy config output path |
| `GOSHPANEL_SYSTEMD_UNIT_DIR` | `data/generated/systemd` | systemd unit file directory |
| `GOSHPANEL_DOCKER` | `true` | Enable Docker module (requires `docker` CLI) |
| `GOSHPANEL_FUNCTIONS` | `true` | Enable micro functions module |

## Development

```bash
go test ./...
go vet ./...
```

## Project layout

```
cmd/goshpanel/       Entry point
internal/auth/       Sessions, login, user management
internal/backups/    tar.gz archive service
internal/config/     Environment configuration
internal/cron/       Cron validation, crontab render/apply
internal/crypto/     argon2id hashing, token generation
internal/dbmanager/  Multi-driver database console
internal/dns/        DNS record validation, zone file rendering
internal/domains/    Domain validation, Caddyfile rendering
internal/email/      Mailboxes, forwarders, maddy config export
internal/files/      Sandboxed file manager
internal/logs/       Allowlisted log tailing
internal/runner/     bash command runner
internal/security/   IP blocker (net/netip)
internal/store/      SQLite persistence (modernc.org/sqlite)
internal/system/     /proc metrics
internal/web/        HTTP server, handlers, templates, CSS
internal/orchestrator/ Live apply for Caddy, CoreDNS, maddy, systemd
internal/docker/       Docker CLI wrapper (containers, compose)
internal/fn/           HTTP-triggered micro functions
docs/FEATURES.md     cPanel feature-parity matrix
```
