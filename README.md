# GoshPanel

GoshPanel is a Linux web hosting control panel built almost entirely in Go. It aims to be a practical alternative to cPanel for common server administration tasks.

## Stack

- **Backend:** Go 1.22, chi router, viper configuration, SQLite
- **UI:** [templ](https://templ.guide/) server-rendered HTML, [HTMX](https://htmx.org/) for partial updates, [Tailwind CSS](https://tailwindcss.com/) for styling
- **Deployment:** Single static binary with embedded assets

## Quick start

### Prerequisites

- Go 1.22+
- Node.js 20+ (development only, for Tailwind CSS)

### Build and run

```bash
make tidy
npm install
make build
GOSHPANEL_AUTH_BOOTSTRAP_PASSWORD=yourpassword ./bin/goshpanel
```

Open http://localhost:4674/login and sign in with `admin` / `yourpassword`.

### Development

```bash
npm install
make generate
make css-watch
make dev
```

### Configuration

Copy the example config and adjust as needed:

```bash
sudo mkdir -p /etc/goshpanel
sudo cp configs/config.example.yaml /etc/goshpanel/config.yaml
```

Key settings:

- `database.path` — SQLite database location
- `auth.session_secret` — session signing secret (required in production)
- `auth.bootstrap_username` / `auth.bootstrap_password` — first-run admin user
- `files.root` — sandbox root for the file manager (default `data/workspace`)

Environment variables use the `GOSHPANEL_` prefix, for example `GOSHPANEL_FILES_ROOT=/srv/sites`.

## Modules

| Module | Route | Status |
|--------|-------|--------|
| Dashboard | `/dashboard` | Available |
| Files | `/files` | Available — browse, upload, delete within sandbox |
| Database | `/database` | Planned |
| Domains | `/domains` | Planned |
| Email | `/email` | Planned |
| Logging | `/logging` | Planned |
| Security | `/security` | Planned |

## Project layout

```
cmd/goshpanel/          Application entry point
internal/auth/          Sessions, login, CSRF
internal/config/        Configuration loading
internal/files/         Sandboxed file manager service
internal/server/        HTTP server and routing
internal/store/         SQLite repositories and migrations
internal/ui/            templ pages and handlers
web/static/             CSS and JavaScript assets (embedded at build time)
configs/                Example configuration
deploy/                 systemd unit file
```

## Status

Phase 2 adds the file manager: sandboxed listing, HTMX navigation, upload/delete, permission display, and SQLite audit logging.
