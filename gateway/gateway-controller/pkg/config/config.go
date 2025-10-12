package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Config holds all configuration for the gateway-controller
type Config struct {
	Server  ServerConfig  `koanf:"server"`
	Storage StorageConfig `koanf:"storage"`
	Router  RouterConfig  `koanf:"router"`
	Logging LoggingConfig `koanf:"logging"`
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	APIPort         int           `koanf:"api_port"`
	XDSPort         int           `koanf:"xds_port"`
	ShutdownTimeout time.Duration `koanf:"shutdown_timeout"`
}

// StorageConfig holds storage-related configuration
type StorageConfig struct {
	Mode         string `koanf:"mode"`          // "memory-only" or "persistent"
	DatabasePath string `koanf:"database_path"` // Path to bbolt database (used when mode=persistent)
}

// RouterConfig holds router (Envoy) related configuration
type RouterConfig struct {
	AccessLogs   AccessLogsConfig `koanf:"access_logs"`
	ListenerPort int              `koanf:"listener_port"`
}

// AccessLogsConfig holds access log configuration
type AccessLogsConfig struct {
	Enabled bool   `koanf:"enabled"`
	Format  string `koanf:"format"` // "json" or "text"
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `koanf:"level"`  // "debug", "info", "warn", "error"
	Format string `koanf:"format"` // "json" or "console"
}

// LoadConfig loads configuration from file, environment variables, and defaults
// Priority: Environment variables > Config file > Defaults
func LoadConfig(configPath string) (*Config, error) {
	k := koanf.New(".")

	// Load defaults
	defaults := getDefaults()
	if err := k.Load(confmap.Provider(defaults, "."), nil); err != nil {
		return nil, fmt.Errorf("failed to load defaults: %w", err)
	}

	// Load config file if it exists
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			if err := k.Load(file.Provider(configPath), yaml.Parser()); err != nil {
				return nil, fmt.Errorf("failed to load config file: %w", err)
			}
		}
	}

	// Load environment variables with prefix "GC_" (Gateway Controller)
	// Example: GC_SERVER_API_PORT=9090
	// Maps to: server.api_port
	if err := k.Load(env.Provider("GC_", ".", func(s string) string {
		// Remove prefix and convert to lowercase with dots
		s = strings.TrimPrefix(s, "GC_")
		s = strings.ToLower(s)
		s = strings.ReplaceAll(s, "_", ".")
		return s
	}), nil); err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	// Unmarshal into Config struct
	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// getDefaults returns a map with default configuration values
func getDefaults() map[string]interface{} {
	return map[string]interface{}{
		"server.api_port":            9090,
		"server.xds_port":            18000,
		"server.shutdown_timeout":    "15s",
		"storage.mode":               "memory-only",
		"storage.database_path":      "/data/gateway-controller.db",
		"router.access_logs.enabled": true,
		"router.access_logs.format":  "json",
		"router.listener_port":       8080,
		"logging.level":              "info",
		"logging.format":             "json",
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate storage mode
	if c.Storage.Mode != "memory-only" && c.Storage.Mode != "persistent" {
		return fmt.Errorf("storage.mode must be either 'memory-only' or 'persistent', got: %s", c.Storage.Mode)
	}

	// Validate database path for persistent mode
	if c.Storage.Mode == "persistent" && c.Storage.DatabasePath == "" {
		return fmt.Errorf("storage.database_path is required when storage.mode is 'persistent'")
	}

	// Validate access log format
	if c.Router.AccessLogs.Format != "json" && c.Router.AccessLogs.Format != "text" {
		return fmt.Errorf("router.access_logs.format must be either 'json' or 'text', got: %s", c.Router.AccessLogs.Format)
	}

	// Validate log level
	validLevels := []string{"debug", "info", "warn", "warning", "error"}
	isValidLevel := false
	for _, level := range validLevels {
		if strings.ToLower(c.Logging.Level) == level {
			isValidLevel = true
			break
		}
	}
	if !isValidLevel {
		return fmt.Errorf("logging.level must be one of: debug, info, warn, error, got: %s", c.Logging.Level)
	}

	// Validate log format
	if c.Logging.Format != "json" && c.Logging.Format != "console" {
		return fmt.Errorf("logging.format must be either 'json' or 'console', got: %s", c.Logging.Format)
	}

	// Validate ports
	if c.Server.APIPort < 1 || c.Server.APIPort > 65535 {
		return fmt.Errorf("server.api_port must be between 1 and 65535, got: %d", c.Server.APIPort)
	}

	if c.Server.XDSPort < 1 || c.Server.XDSPort > 65535 {
		return fmt.Errorf("server.xds_port must be between 1 and 65535, got: %d", c.Server.XDSPort)
	}

	if c.Router.ListenerPort < 1 || c.Router.ListenerPort > 65535 {
		return fmt.Errorf("router.listener_port must be between 1 and 65535, got: %d", c.Router.ListenerPort)
	}

	return nil
}

// IsPersistentMode returns true if storage mode is persistent
func (c *Config) IsPersistentMode() bool {
	return c.Storage.Mode == "persistent"
}

// IsAccessLogsEnabled returns true if access logs are enabled
func (c *Config) IsAccessLogsEnabled() bool {
	return c.Router.AccessLogs.Enabled
}
