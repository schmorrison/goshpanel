package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

const (
	defaultHost          = "0.0.0.0"
	defaultPort          = 4674
	defaultDatabasePath  = "data/goshpanel.db"
	defaultBootstrapUser = "admin"
	defaultFilesRoot     = "data/workspace"
)

// Config holds GoshPanel runtime settings.
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Files    FilesConfig    `mapstructure:"files"`
}

// ServerConfig controls the HTTP listener.
type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

// DatabaseConfig controls SQLite storage.
type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

// AuthConfig controls sessions and bootstrap credentials.
type AuthConfig struct {
	SessionSecret     string `mapstructure:"session_secret"`
	BootstrapUsername string `mapstructure:"bootstrap_username"`
	BootstrapPassword string `mapstructure:"bootstrap_password"`
}

// FilesConfig controls the sandboxed file manager.
type FilesConfig struct {
	Root string `mapstructure:"root"`
}

// Addr returns the host:port listen address.
func (c Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// Load reads configuration from defaults, environment variables, and an optional YAML file.
func Load() (Config, error) {
	v := viper.New()

	v.SetDefault("server.host", defaultHost)
	v.SetDefault("server.port", defaultPort)
	v.SetDefault("database.path", defaultDatabasePath)
	v.SetDefault("auth.bootstrap_username", defaultBootstrapUser)
	v.SetDefault("files.root", defaultFilesRoot)

	_ = v.BindEnv("server.host", "GOSHPANEL_SERVER_HOST")
	_ = v.BindEnv("server.port", "GOSHPANEL_SERVER_PORT")
	_ = v.BindEnv("database.path", "GOSHPANEL_DATABASE_PATH")
	_ = v.BindEnv("auth.session_secret", "GOSHPANEL_AUTH_SESSION_SECRET")
	_ = v.BindEnv("auth.bootstrap_username", "GOSHPANEL_AUTH_BOOTSTRAP_USERNAME")
	_ = v.BindEnv("auth.bootstrap_password", "GOSHPANEL_AUTH_BOOTSTRAP_PASSWORD")
	_ = v.BindEnv("files.root", "GOSHPANEL_FILES_ROOT")

	v.SetEnvPrefix("GOSHPANEL")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("/etc/goshpanel")
	if home, err := os.UserHomeDir(); err == nil {
		v.AddConfigPath(filepath.Join(home, ".goshpanel"))
	}
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return Config{}, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}

	return cfg, nil
}

// SessionSecret returns the configured session secret or generates an ephemeral one.
func (c Config) SessionSecret() (string, bool, error) {
	if c.Auth.SessionSecret != "" {
		return c.Auth.SessionSecret, false, nil
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", false, fmt.Errorf("generate session secret: %w", err)
	}
	return hex.EncodeToString(buf), true, nil
}
