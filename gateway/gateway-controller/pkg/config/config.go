/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
	Type     string         `koanf:"type"`     // "sqlite", "postgres", or "memory"
	SQLite   SQLiteConfig   `koanf:"sqlite"`   // SQLite-specific configuration
	Postgres PostgresConfig `koanf:"postgres"` // PostgreSQL-specific configuration (future)
}

// SQLiteConfig holds SQLite-specific configuration
type SQLiteConfig struct {
	Path string `koanf:"path"` // Path to SQLite database file
}

// PostgresConfig holds PostgreSQL-specific configuration (future support)
type PostgresConfig struct {
	Host     string `koanf:"host"`
	Port     int    `koanf:"port"`
	Database string `koanf:"database"`
	User     string `koanf:"user"`
	Password string `koanf:"password"`
	SSLMode  string `koanf:"sslmode"`
}

// RouterConfig holds router (Envoy) related configuration
type RouterConfig struct {
	AccessLogs   AccessLogsConfig `koanf:"access_logs"`
	ListenerPort int              `koanf:"listener_port"`
}

// AccessLogsConfig holds access log configuration
type AccessLogsConfig struct {
	Enabled    bool              `koanf:"enabled"`
	Format     string            `koanf:"format"`      // "json" or "text"
	JSONFields map[string]string `koanf:"json_fields"` // JSON log format fields
	TextFormat string            `koanf:"text_format"` // Text log format template
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
			// Use WithMergeFunc to prevent merging of maps - config file should fully override defaults
			if err := k.Load(file.Provider(configPath), yaml.Parser(), koanf.WithMergeFunc(func(src, dest map[string]interface{}) error {
				// For nested maps, replace instead of merge
				for k, v := range src {
					dest[k] = v
				}
				return nil
			})); err != nil {
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
		"server.api_port":         9090,
		"server.xds_port":         18000,
		"server.shutdown_timeout": "15s",
		"storage.type":            "memory",
		"storage.sqlite.path":     "/data/gateway.db",
		"router.access_logs.enabled": true,
		"router.access_logs.format":  "json",
		"router.access_logs.json_fields": map[string]interface{}{
			"start_time":            "%START_TIME%",
			"method":                "%REQ(:METHOD)%",
			"path":                  "%REQ(X-ENVOY-ORIGINAL-PATH?:PATH)%",
			"protocol":              "%PROTOCOL%",
			"response_code":         "%RESPONSE_CODE%",
			"response_flags":        "%RESPONSE_FLAGS%",
			"response_flags_long":   "%RESPONSE_FLAGS_LONG%",
			"bytes_received":        "%BYTES_RECEIVED%",
			"bytes_sent":            "%BYTES_SENT%",
			"duration":              "%DURATION%",
			"upstream_service_time": "%RESP(X-ENVOY-UPSTREAM-SERVICE-TIME)%",
			"x_forwarded_for":       "%REQ(X-FORWARDED-FOR)%",
			"user_agent":            "%REQ(USER-AGENT)%",
			"request_id":            "%REQ(X-REQUEST-ID)%",
			"authority":             "%REQ(:AUTHORITY)%",
			"upstream_host":         "%UPSTREAM_HOST%",
			"upstream_cluster":      "%UPSTREAM_CLUSTER%",
		},
		"router.access_logs.text_format": "[%START_TIME%] \"%REQ(:METHOD)% %REQ(X-ENVOY-ORIGINAL-PATH?:PATH)% %PROTOCOL%\" " +
			"%RESPONSE_CODE% %RESPONSE_FLAGS% %BYTES_RECEIVED% %BYTES_SENT% %DURATION% " +
			"\"%REQ(X-FORWARDED-FOR)%\" \"%REQ(USER-AGENT)%\" \"%REQ(X-REQUEST-ID)%\" " +
			"\"%REQ(:AUTHORITY)%\" \"%UPSTREAM_HOST%\"\n",
		"router.listener_port": 8080,
		"logging.level":        "info",
		"logging.format":       "json",
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate storage type
	validStorageTypes := []string{"sqlite", "postgres", "memory"}
	isValidType := false
	for _, t := range validStorageTypes {
		if c.Storage.Type == t {
			isValidType = true
			break
		}
	}
	if !isValidType {
		return fmt.Errorf("storage.type must be one of: sqlite, postgres, memory, got: %s", c.Storage.Type)
	}

	// Validate SQLite configuration
	if c.Storage.Type == "sqlite" && c.Storage.SQLite.Path == "" {
		return fmt.Errorf("storage.sqlite.path is required when storage.type is 'sqlite'")
	}

	// Validate PostgreSQL configuration (future)
	if c.Storage.Type == "postgres" {
		if c.Storage.Postgres.Host == "" {
			return fmt.Errorf("storage.postgres.host is required when storage.type is 'postgres'")
		}
		if c.Storage.Postgres.Database == "" {
			return fmt.Errorf("storage.postgres.database is required when storage.type is 'postgres'")
		}
	}

	// Validate access log format
	if c.Router.AccessLogs.Format != "json" && c.Router.AccessLogs.Format != "text" {
		return fmt.Errorf("router.access_logs.format must be either 'json' or 'text', got: %s", c.Router.AccessLogs.Format)
	}

	// Validate access log fields if access logs are enabled
	if c.Router.AccessLogs.Enabled {
		if c.Router.AccessLogs.Format == "json" {
			if c.Router.AccessLogs.JSONFields == nil || len(c.Router.AccessLogs.JSONFields) == 0 {
				return fmt.Errorf("router.access_logs.json_fields must be configured when format is 'json'")
			}
		} else if c.Router.AccessLogs.Format == "text" {
			if c.Router.AccessLogs.TextFormat == "" {
				return fmt.Errorf("router.access_logs.text_format must be configured when format is 'text'")
			}
		}
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

// IsPersistentMode returns true if storage type is not memory
func (c *Config) IsPersistentMode() bool {
	return c.Storage.Type != "memory"
}

// IsMemoryOnlyMode returns true if storage type is memory
func (c *Config) IsMemoryOnlyMode() bool {
	return c.Storage.Type == "memory"
}

// IsAccessLogsEnabled returns true if access logs are enabled
func (c *Config) IsAccessLogsEnabled() bool {
	return c.Router.AccessLogs.Enabled
}
