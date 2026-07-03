# cPanel Feature Parity Matrix

This document lists the features cPanel provides, whether each can be provided in (pure) Go, and the current GoshPanel status. It is the roadmap for reaching feature parity.

Legend:

- **Implemented** — working in GoshPanel today.
- **Config export** — GoshPanel manages the data and generates config for a Go-native companion daemon (Caddy, CoreDNS, maddy) which does the heavy lifting.
- **Planned** — feasible in Go, not yet built.
- **Delegated** — inherently the job of an external system component (e.g. the kernel, systemd); Go orchestrates via process calls.

## Files

| cPanel feature | Go feasibility | GoshPanel status | Notes |
|---|---|---|---|
| File Manager (browse/upload/edit/rename/delete/mkdir) | Pure Go (`os`, `io`) | **Implemented** | Sandboxed to `GOSHPANEL_FILES_ROOT`; in-browser editor (2 MiB limit) |
| File download | Pure Go | **Implemented** | `http.ServeContent` |
| Backup / Backup Wizard | Pure Go (`archive/tar`, `compress/gzip`) | **Implemented** | Create, list, download, restore, delete tar.gz archives |
| Disk Usage | Pure Go (`syscall.Statfs`) | **Implemented** | On dashboard |
| FTP Accounts / FTP Connections | Go FTP/SFTP servers exist (e.g. `pkg/sftp`) | Planned | SFTP preferred over FTP today |
| Images (thumbnailer/converter) | Pure Go (`image` stdlib) | Planned | Low priority |
| Directory Privacy (htpasswd) | Pure Go | Planned | Would render Caddy `basic_auth` blocks |
| Git Version Control | Process calls to `git` | Planned | `go-git` is a pure-Go option |
| Web Disk (WebDAV) | Pure Go (`golang.org/x/net/webdav`) | Planned | |

## Databases

| cPanel feature | Go feasibility | GoshPanel status | Notes |
|---|---|---|---|
| MySQL/MariaDB management | Pure Go driver (`go-sql-driver/mysql`) | **Implemented** | Saved connections, table list, read-only query console |
| PostgreSQL management | Pure Go driver (`jackc/pgx`) | **Implemented** | Same console |
| SQLite management | Pure Go driver (`modernc.org/sqlite`, no cgo) | **Implemented** | Also used for the panel's own state |
| phpMyAdmin equivalent | Pure Go UI | **Implemented (read-only)** | Write queries planned behind an explicit opt-in |
| Database user/privilege management | Pure Go (SQL statements) | Planned | `CREATE USER` / `GRANT` runner |
| Remote MySQL (access hosts) | Pure Go | Planned | |

## Domains

| cPanel feature | Go feasibility | GoshPanel status | Notes |
|---|---|---|---|
| Domains / Addon domains / Subdomains | Pure Go + Caddy | **Implemented** | Domain records with doc root and/or reverse-proxy upstream |
| Zone Editor (DNS records) | Pure Go | **Implemented** | A, AAAA, CNAME, MX, TXT, NS, SRV with validation; RFC 1035 zone file export (serve with CoreDNS) |
| Redirects | Caddy `redir` directive | Planned | Add redirect rules to Caddyfile renderer |
| Aliases (parked domains) | Caddy multi-site blocks | Planned | |
| Dynamic DNS | Pure Go | Planned | |
| Web server vhost config | Config export | **Implemented** | Caddyfile generated & downloadable; `caddy reload` process call when binary present |

## Email

| cPanel feature | Go feasibility | GoshPanel status | Notes |
|---|---|---|---|
| Email Accounts | Pure Go + maddy | **Implemented (definitions + config export)** | Mailboxes with argon2id password hashes and quotas; maddy config export |
| Forwarders | Pure Go | **Implemented** | |
| SMTP/IMAP service | Go mail servers: maddy, go-guerrilla | Config export | maddy is a complete pure-Go MTA + IMAP server |
| Spam filters | Go milters exist | Planned | |
| Autoresponders | maddy/custom Go hook | Planned | |
| Email Deliverability (SPF/DKIM/DMARC) | Pure Go | Planned | Generate TXT records into the Zone Editor |
| Webmail | Go webmail projects exist | Planned | Out of core scope |
| Mailing lists | — | Not planned | Niche; external tools |

## Metrics & Logs

| cPanel feature | Go feasibility | GoshPanel status | Notes |
|---|---|---|---|
| Server Information | Pure Go (`/proc`) | **Implemented** | Hostname, kernel, CPU, memory, disk, load, uptime |
| Resource Usage dashboard | Pure Go | **Implemented** | Live snapshot per page load; historical charts planned |
| Errors / raw log access | Pure Go | **Implemented** | Tail viewer over an allowlist (`GOSHPANEL_LOG_SOURCES`) |
| Visitors / Awstats / Analog / Webalizer | Pure Go log parsing | Planned | Parse Caddy JSON access logs |
| Bandwidth | Pure Go | Planned | From access logs / `/proc/net/dev` |

## Security

| cPanel feature | Go feasibility | GoshPanel status | Notes |
|---|---|---|---|
| Panel authentication | Pure Go (argon2id, sessions) | **Implemented** | Bootstrap admin, roles (admin/user), session cookies, CSRF tokens |
| Password & Security (change password) | Pure Go | **Implemented** | |
| User Manager | Pure Go | **Implemented** | Admin-only create/delete panel users |
| IP Blocker | Pure Go (`net/netip`) | **Implemented** | Panel-level deny by IP/CIDR; firewall (nftables) integration planned |
| Audit/access log | Pure Go | **Implemented** | All mutating panel actions recorded |
| SSL/TLS + AutoSSL (Let's Encrypt) | Caddy automatic HTTPS / `certmagic` | Config export | Caddy handles issuance/renewal for managed domains automatically |
| SSH Access (key management) | Pure Go (`golang.org/x/crypto/ssh`) | Planned | authorized_keys management |
| Hotlink / Leech Protection | Caddy directives | Planned | |
| ModSecurity (WAF) | Go WAF: Coraza | Planned | Coraza is a pure-Go OWASP CRS engine |
| Two-Factor Authentication | Pure Go (TOTP) | Planned | |
| Terminal | Process call to `bash` | **Implemented** | Request/response command runner with timeout; PTY/WebSocket upgrade planned |
| Orchestrator (live apply) | Process calls | **Implemented** | Auto-write & reload Caddy, CoreDNS, maddy; systemd unit management |
| Docker | Process calls (`docker` CLI) | **Implemented** | Containers, images, logs, run, compose stack up/down |
| Micro Functions | Pure Go + bash | **Implemented** | HTTP-triggered bash scripts at `/fn/{name}?token=...` |

## Advanced

| cPanel feature | Go feasibility | GoshPanel status | Notes |
|---|---|---|---|
| Cron Jobs | Pure Go validation + `crontab` process call | **Implemented** | Full 5-field expression validation, @shortcuts, crontab export & install |
| Apache Handlers / MIME Types | Caddy equivalents | Not planned | Apache-specific |
| Indexes (directory listing) | Caddy `file_server browse` | Planned | Toggle per domain |
| Error Pages | Caddy `handle_errors` | Planned | |
| Track DNS (dig/traceroute) | Pure Go (`net.Resolver`) | Planned | |
| API / webhooks | Pure Go | Planned | JSON API mirroring the UI actions |

## PHP/Software (cPanel "Software" section)

| cPanel feature | Go feasibility | GoshPanel status | Notes |
|---|---|---|---|
| MultiPHP Manager / INI Editor | Process calls | Not planned | GoshPanel targets Go-first stacks; PHP-FPM can still be proxied via Caddy |
| Site Software / WordPress Manager | — | Not planned | |
| Application Manager (deploy apps) | Pure Go + systemd process calls | Planned | Manage systemd units for Go binaries |

## Summary

Implemented today: **13 core modules** — Dashboard, Files, Domains & DNS, Databases, Email, Cron, Backups, Logs, Terminal, Security, **Orchestrator**, **Docker**, **Micro Functions**.

Everything cPanel does is achievable from Go, in three tiers:

1. **Pure Go in-process** — files, backups, DNS zone data, cron validation, metrics, auth, IP blocking, database console.
2. **Config export to Go-native daemons** — Caddy (web + automatic HTTPS), CoreDNS (DNS serving), maddy (SMTP/IMAP).
3. **Thin process calls** — `crontab`, `caddy reload`, and future `systemctl`/`nft` integration. No cgo anywhere; the only cgo-free exception candidate (SQLite) uses `modernc.org/sqlite`.
