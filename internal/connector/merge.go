package connector

import "encoding/json"

// MergeCaddyConfig applies form updates while keeping secrets when left blank.
func MergeCaddyConfig(existing CaddyConfig, updated CaddyConfig) CaddyConfig {
	if updated.AdminToken == "" {
		updated.AdminToken = existing.AdminToken
	}
	return updated
}

// MergeDatabaseConfig applies form updates while keeping passwords when left blank.
func MergeDatabaseConfig(existing, updated DatabaseConfig) DatabaseConfig {
	if updated.AdminPassword == "" {
		updated.AdminPassword = existing.AdminPassword
		updated.AdminPasswordEnc = existing.AdminPasswordEnc
	}
	return updated
}

// MergeConfigJSON merges connector config for an update (kind + existing JSON + new struct).
func MergeConfigJSON(kind string, existingJSON string, updated any) (any, error) {
	switch kind {
	case "caddy":
		existing, err := ParseCaddyConfig(existingJSON)
		if err != nil {
			return nil, err
		}
		cfg, ok := updated.(CaddyConfig)
		if !ok {
			return nil, nil
		}
		return MergeCaddyConfig(existing, cfg), nil
	case "postgres", "mysql":
		existing, err := ParseDatabaseConfig(existingJSON)
		if err != nil {
			return nil, err
		}
		cfg, ok := updated.(DatabaseConfig)
		if !ok {
			return nil, nil
		}
		return MergeDatabaseConfig(existing, cfg), nil
	default:
		return updated, nil
	}
}

// ConfigFromJSON unmarshals connector config by kind.
func ConfigFromJSON(kind, raw string) (any, error) {
	switch kind {
	case "caddy":
		c, err := ParseCaddyConfig(raw)
		return c, err
	case "docker":
		return ParseDockerConfig(raw)
	case "coredns":
		c, err := ParseCoreDNSConfig(raw)
		return c, err
	case "maddy":
		c, err := ParseMaddyConfig(raw)
		return c, err
	case "postgres", "mysql":
		return ParseDatabaseConfig(raw)
	default:
		var m map[string]string
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			return nil, err
		}
		return m, nil
	}
}
