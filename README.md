# GoshPanel

GoshPanel is a Linux web hosting control panel built almost entirely in Go. It aims to be a practical alternative to cPanel for common server administration tasks.

## Stack

- **Backend:** Go 1.22, chi router, viper configuration
- **UI:** [templ](https://templ.guide/) server-rendered HTML, [HTMX](https://htmx.org/) for partial updates, [Tailwind CSS](https://tailwindcss.com/) for styling
- **Deployment:** Single static binary with embedded assets

## Planned modules

- File manager
- Database manager
- Domain and DNS management
- Email (via [go-guerrilla](https://github.com/flashmob/go-guerrilla))
- Logging and monitoring
- Security and certificates
- Reverse proxy integration (via [Caddy](https://caddyserver.com/))
- SSH access (via [Teleport](https://goteleport.com/))

## Quick start

### Prerequisites

- Go 1.22+
- Node.js 20+ (development only, for Tailwind CSS)

### Build and run

```bash
make tidy
npm install
make build
./bin/goshpanel
```

Open http://localhost:4674 — the homepage and `GET /healthz` should respond.

### Development

```bash
npm install
make generate   # regenerate templ files after editing .templ
make css-watch  # rebuild Tailwind CSS on change (separate terminal)
make dev        # run the server
```

### Configuration

Copy the example config and adjust as needed:

```bash
sudo mkdir -p /etc/goshpanel
sudo cp configs/config.example.yaml /etc/goshpanel/config.yaml
```

Environment variables override file settings using the `GOSHPANEL_` prefix (for example `GOSHPANEL_SERVER_PORT=8080`).

### Docker

```bash
docker build -t goshpanel .
docker run --rm -p 4674:4674 goshpanel
```

## Project layout

```
cmd/goshpanel/          Application entry point
internal/config/        Configuration loading
internal/server/        HTTP server and routing
internal/ui/            templ pages and handlers
web/static/             CSS and JavaScript assets (embedded at build time)
configs/                Example configuration
deploy/                 systemd unit file
```

## Status

Phase 1 adds session authentication, SQLite user storage, CSRF-protected login/logout, and an authenticated dashboard shell with sidebar navigation.

### Default credentials

On first startup with an empty database, GoshPanel creates a bootstrap admin user:

- Username: `admin` (override with `auth.bootstrap_username`)
- Password: value of `auth.bootstrap_password`, or a randomly generated password logged to stdout when left empty

Set `auth.session_secret` in production. If omitted, an ephemeral secret is generated for the current process only.

### Routes

| Route | Access | Description |
|-------|--------|-------------|
| `GET /` | Public | Landing page (redirects to dashboard when signed in) |
| `GET /login` | Public | Sign-in form |
| `POST /login` | Public | Authenticate (CSRF protected) |
| `POST /logout` | Authenticated | End session (CSRF protected) |
| `GET /dashboard` | Authenticated | Module overview with sidebar |
| `GET /healthz` | Public | Health check |

Module paths (`/files`, `/database`, etc.) are linked in the sidebar and arrive in later phases.
