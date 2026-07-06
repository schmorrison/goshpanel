package installers

import (
	"fmt"
	"strings"
)

// Spec describes an installable application.
type Spec struct {
	ID          string
	Name        string
	Description string
	Port        int
}

// List returns built-in installer specs.
func List() []Spec {
	return []Spec{
		{ID: "wordpress", Name: "WordPress", Description: "Blog/CMS via Docker (MySQL + WordPress)", Port: 8080},
		{ID: "ghost", Name: "Ghost", Description: "Publishing platform via Docker", Port: 2368},
		{ID: "gitea", Name: "Gitea", Description: "Lightweight Git hosting via Docker", Port: 3000},
		{ID: "nextcloud", Name: "Nextcloud", Description: "Self-hosted cloud storage", Port: 8081},
		{ID: "minio", Name: "MinIO", Description: "S3-compatible object storage", Port: 9000},
		{ID: "plausible", Name: "Plausible", Description: "Privacy-friendly analytics", Port: 8000},
		{ID: "uptime-kuma", Name: "Uptime Kuma", Description: "Uptime monitoring dashboard", Port: 3001},
		{ID: "vaultwarden", Name: "Vaultwarden", Description: "Bitwarden-compatible password manager", Port: 8082},
	}
}

// SpecByID returns one installer spec.
func SpecByID(id string) (Spec, bool) {
	for _, s := range List() {
		if s.ID == id {
			return s, true
		}
	}
	return Spec{}, false
}

// ComposeYAML returns docker-compose content for an installer.
func ComposeYAML(id, siteName string) (string, error) {
	siteName = strings.TrimSpace(siteName)
	if siteName == "" {
		return "", fmt.Errorf("site name required")
	}
	switch id {
	case "wordpress":
		return fmt.Sprintf(`services:
  db:
    image: mysql:8.0
    ports:
      - "13306:3306"
    environment:
      MYSQL_ROOT_PASSWORD: goshpanel
      MYSQL_DATABASE: wordpress
      MYSQL_USER: wordpress
      MYSQL_PASSWORD: goshpanel
    volumes:
      - db_data:/var/lib/mysql
  wordpress:
    image: wordpress:latest
    ports:
      - "8080:80"
    environment:
      WORDPRESS_DB_HOST: db
      WORDPRESS_DB_USER: wordpress
      WORDPRESS_DB_PASSWORD: goshpanel
      WORDPRESS_DB_NAME: wordpress
    depends_on:
      - db
volumes:
  db_data:
`), nil
	case "ghost":
		return fmt.Sprintf(`services:
  ghost:
    image: ghost:5-alpine
    ports:
      - "2368:2368"
    environment:
      url: http://localhost:2368
    volumes:
      - ghost_data:/var/lib/ghost/content
volumes:
  ghost_data:
`), nil
	case "gitea":
		return `services:
  gitea:
    image: gitea/gitea:latest
    ports:
      - "3000:3000"
      - "222:22"
    volumes:
      - gitea_data:/data
volumes:
  gitea_data:
`, nil
	case "nextcloud":
		return `services:
  nextcloud:
    image: nextcloud:latest
    ports:
      - "8081:80"
    volumes:
      - nc_data:/var/www/html
volumes:
  nc_data:
`, nil
	case "minio":
		return `services:
  minio:
    image: minio/minio:latest
    command: server /data --console-address ":9001"
    ports:
      - "9000:9000"
      - "9001:9001"
    volumes:
      - minio_data:/data
volumes:
  minio_data:
`, nil
	case "plausible":
		return `services:
  plausible:
    image: plausible/analytics:latest
    ports:
      - "8000:8000"
    environment:
      BASE_URL: http://localhost:8000
volumes:
  plausible_data:
`, nil
	case "uptime-kuma":
		return `services:
  uptime-kuma:
    image: louislam/uptime-kuma:1
    ports:
      - "3001:3001"
    volumes:
      - uk_data:/app/data
volumes:
  uk_data:
`, nil
	case "vaultwarden":
		return `services:
  vaultwarden:
    image: vaultwarden/server:latest
    ports:
      - "8082:80"
    volumes:
      - vw_data:/data
volumes:
  vw_data:
`, nil
	default:
		return "", fmt.Errorf("unknown installer %q", id)
	}
}

// InstallerDBInfo describes an auto-registered database connector after install.
type InstallerDBInfo struct {
	Kind          string
	Host          string
	Port          int
	AdminUser     string
	AdminPassword string
	Database      string
	ConnectorName string
}

// DatabaseConnInfo returns connection details for installers that ship a database.
func DatabaseConnInfo(installerID, domain string) (InstallerDBInfo, bool) {
	switch installerID {
	case "wordpress":
		slug := strings.ReplaceAll(strings.ToLower(domain), ".", "-")
		return InstallerDBInfo{
			Kind:          "mysql",
			Host:          "127.0.0.1",
			Port:          13306,
			AdminUser:     "root",
			AdminPassword: "goshpanel",
			Database:      "wordpress",
			ConnectorName: "wordpress-" + slug + "-mysql",
		}, true
	default:
		return InstallerDBInfo{}, false
	}
}

func StackName(installerID, domain string) string {
	domain = strings.ReplaceAll(strings.ToLower(domain), ".", "-")
	return installerID + "-" + domain
}

// UpstreamHostPort returns the localhost upstream for Caddy reverse proxy.
func UpstreamHostPort(spec Spec) string {
	return fmt.Sprintf("localhost:%d", spec.Port)
}
