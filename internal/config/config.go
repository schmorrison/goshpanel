package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

const (
	defaultHost = "0.0.0.0"
	defaultPort = 4674
)

// Config holds GoshPanel runtime settings.
type Config struct {
	Server ServerConfig `mapstructure:"server"`
}

// ServerConfig controls the HTTP listener.
type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
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
