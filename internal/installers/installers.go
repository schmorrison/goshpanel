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
	default:
		return "", fmt.Errorf("unknown installer %q", id)
	}
}

// StackName returns the docker stack name for a site.
func StackName(installerID, domain string) string {
	domain = strings.ReplaceAll(strings.ToLower(domain), ".", "-")
	return installerID + "-" + domain
}

// UpstreamHostPort returns the localhost upstream for Caddy reverse proxy.
func UpstreamHostPort(spec Spec) string {
	return fmt.Sprintf("localhost:%d", spec.Port)
}
