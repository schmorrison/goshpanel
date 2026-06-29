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

Phase 0 (foundation) is complete: unified module, health check, styled homepage, embedded static assets, CI, and container packaging. Authentication and feature modules are next.
