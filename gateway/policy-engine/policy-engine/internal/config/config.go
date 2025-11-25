package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config represents the complete policy engine configuration
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	ConfigMode ConfigModeConfig `mapstructure:"config_mode"`
	XDS        XDSConfig        `mapstructure:"xds"`
	FileConfig FileConfigConfig `mapstructure:"file_config"`
	Logging    LoggingConfig    `mapstructure:"logging"`
}

// ServerConfig holds ext_proc server configuration
type ServerConfig struct {
	// ExtProcPort is the port for the ext_proc gRPC server
	ExtProcPort int `mapstructure:"extproc_port"`
}

// ConfigModeConfig specifies how policy chains are configured
type ConfigModeConfig struct {
	// Mode can be "file" or "xds"
	Mode string `mapstructure:"mode"`
}

// XDSConfig holds xDS client configuration
type XDSConfig struct {
	// Enabled indicates whether xDS client should be started
	Enabled bool `mapstructure:"enabled"`

	// ServerAddress is the xDS server address (e.g., "localhost:18000")
	ServerAddress string `mapstructure:"server_address"`

	// NodeID identifies this policy engine instance to the xDS server
	NodeID string `mapstructure:"node_id"`

	// Cluster identifies the cluster this policy engine belongs to
	Cluster string `mapstructure:"cluster"`

	// ConnectTimeout is the timeout for establishing initial connection
	ConnectTimeout time.Duration `mapstructure:"connect_timeout"`

	// RequestTimeout is the timeout for individual xDS requests
	RequestTimeout time.Duration `mapstructure:"request_timeout"`

	// InitialReconnectDelay is the initial delay before reconnecting
	InitialReconnectDelay time.Duration `mapstructure:"initial_reconnect_delay"`

	// MaxReconnectDelay is the maximum delay between reconnection attempts
	MaxReconnectDelay time.Duration `mapstructure:"max_reconnect_delay"`

	// TLS configuration
	TLS XDSTLSConfig `mapstructure:"tls"`
}

// XDSTLSConfig holds TLS configuration for xDS connection
type XDSTLSConfig struct {
	// Enabled indicates whether to use TLS
	Enabled bool `mapstructure:"enabled"`

	// CertPath is the path to the TLS certificate file
	CertPath string `mapstructure:"cert_path"`

	// KeyPath is the path to the TLS private key file
	KeyPath string `mapstructure:"key_path"`

	// CAPath is the path to the CA certificate for server verification
	CAPath string `mapstructure:"ca_path"`
}

// FileConfigConfig holds file-based configuration settings
type FileConfigConfig struct {
	// Path is the path to the policy chains YAML file
	Path string `mapstructure:"path"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	// Level can be "debug", "info", "warn", "error"
	Level string `mapstructure:"level"`

	// Format can be "json" or "text"
	Format string `mapstructure:"format"`
}

// Load loads configuration from a YAML file
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Set config file path
	v.SetConfigFile(configPath)

	// Enable environment variable support
	v.AutomaticEnv()
	// Map environment variables to config keys (e.g., XDS_SERVER_ADDRESS -> xds.server_address)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal config
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default configuration values
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.extproc_port", 9001)

	// Config mode defaults
	v.SetDefault("config_mode.mode", "file")

	// xDS defaults
	v.SetDefault("xds.enabled", false)
	v.SetDefault("xds.server_address", "localhost:18000")
	v.SetDefault("xds.node_id", "policy-engine")
	v.SetDefault("xds.cluster", "policy-engine-cluster")
	v.SetDefault("xds.connect_timeout", "10s")
	v.SetDefault("xds.request_timeout", "5s")
	v.SetDefault("xds.initial_reconnect_delay", "1s")
	v.SetDefault("xds.max_reconnect_delay", "60s")
	v.SetDefault("xds.tls.enabled", false)

	// File config defaults
	v.SetDefault("file_config.path", "configs/policy-chains.yaml")

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate server config
	if c.Server.ExtProcPort <= 0 || c.Server.ExtProcPort > 65535 {
		return fmt.Errorf("invalid extproc_port: %d (must be 1-65535)", c.Server.ExtProcPort)
	}

	// Validate config mode
	if c.ConfigMode.Mode != "file" && c.ConfigMode.Mode != "xds" {
		return fmt.Errorf("invalid config_mode.mode: %s (must be 'file' or 'xds')", c.ConfigMode.Mode)
	}

	// Validate based on config mode
	if c.ConfigMode.Mode == "xds" {
		if !c.XDS.Enabled {
			return fmt.Errorf("xds.enabled must be true when config_mode.mode is 'xds'")
		}
		if err := c.validateXDSConfig(); err != nil {
			return err
		}
	} else if c.ConfigMode.Mode == "file" {
		if c.FileConfig.Path == "" {
			return fmt.Errorf("file_config.path is required when config_mode.mode is 'file'")
		}
	}

	// Validate logging
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("invalid logging.level: %s (must be debug, info, warn, or error)", c.Logging.Level)
	}

	validFormats := map[string]bool{"json": true, "text": true}
	if !validFormats[c.Logging.Format] {
		return fmt.Errorf("invalid logging.format: %s (must be json or text)", c.Logging.Format)
	}

	return nil
}

// validateXDSConfig validates xDS configuration
func (c *Config) validateXDSConfig() error {
	if c.XDS.ServerAddress == "" {
		return fmt.Errorf("xds.server_address is required when xDS is enabled")
	}

	if c.XDS.NodeID == "" {
		return fmt.Errorf("xds.node_id is required when xDS is enabled")
	}

	if c.XDS.Cluster == "" {
		return fmt.Errorf("xds.cluster is required when xDS is enabled")
	}

	if c.XDS.ConnectTimeout <= 0 {
		return fmt.Errorf("xds.connect_timeout must be positive")
	}

	if c.XDS.RequestTimeout <= 0 {
		return fmt.Errorf("xds.request_timeout must be positive")
	}

	if c.XDS.InitialReconnectDelay <= 0 {
		return fmt.Errorf("xds.initial_reconnect_delay must be positive")
	}

	if c.XDS.MaxReconnectDelay <= 0 {
		return fmt.Errorf("xds.max_reconnect_delay must be positive")
	}

	if c.XDS.TLS.Enabled {
		if c.XDS.TLS.CertPath == "" {
			return fmt.Errorf("xds.tls.cert_path is required when TLS is enabled")
		}
		if c.XDS.TLS.KeyPath == "" {
			return fmt.Errorf("xds.tls.key_path is required when TLS is enabled")
		}
		if c.XDS.TLS.CAPath == "" {
			return fmt.Errorf("xds.tls.ca_path is required when TLS is enabled")
		}
	}

	return nil
}
