package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	PolicyEngine PolicyEngine `mapstructure:"policy_engine"`
	GatewayController map[string]interface{} `mapstructure:"gateway_controller"`
	PolicyConfigurations map[string]interface{} `mapstructure:"policy_configurations"`
}

// Config represents the complete policy engine configuration
type PolicyEngine struct {
	Server     ServerConfig     `mapstructure:"server"`
	Admin      AdminConfig      `mapstructure:"admin"`
	ConfigMode ConfigModeConfig `mapstructure:"config_mode"`
	XDS        XDSConfig        `mapstructure:"xds"`
	FileConfig FileConfigConfig `mapstructure:"file_config"`
	Logging    LoggingConfig    `mapstructure:"logging"`

	// RawConfig holds the complete raw configuration map including custom fields
	// This is used for resolving ${config} CEL expressions in policy systemParameters
	RawConfig map[string]interface{} `mapstructure:",remain"`
	AccessLogsService AccessLogsServiceConfig `mapstructure:"access_logs_service"`
	Analytics  AnalyticsConfig  `mapstructure:"analytics"`
}

// ServerConfig holds ext_proc server configuration
type ServerConfig struct {
	// ExtProcPort is the port for the ext_proc gRPC server
	ExtProcPort int `mapstructure:"extproc_port"`
}

// AdminConfig holds admin HTTP server configuration
type AdminConfig struct {
	// Enabled indicates whether the admin server should be started
	Enabled bool `mapstructure:"enabled"`

	// Port is the port for the admin HTTP server
	Port int `mapstructure:"port"`

	// AllowedIPs is a list of IP addresses allowed to access the admin API
	// Defaults to localhost only (127.0.0.1 and ::1)
	AllowedIPs []string `mapstructure:"allowed_ips"`
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

// AccessLogsServiceConfig holds access logs service configuration
type AccessLogsServiceConfig struct {
	Enabled bool `mapstructure:"enabled"`
	ALSServerPort int `mapstructure:"als_server_port"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	PublicKeyPath string `mapstructure:"public_key_path"`
	PrivateKeyPath string `mapstructure:"private_key_path"`
	ALSPlainText bool `mapstructure:"als_plain_text"`
	ExtProcMaxMessageSize int `mapstructure:"max_message_size"`
	ExtProcMaxHeaderLimit int `mapstructure:"max_header_limit"`
}

// AnalyticsConfig holds analytics related configuration
type AnalyticsConfig struct {
	Enabled bool `mapstructure:"enabled"`
	Publishers []PublisherConfig `mapstructure:"publishers"`
}

// PublisherConfig holds analyticspublisher specific configuration
type PublisherConfig struct {
	Enabled bool `mapstructure:"enabled"`
	Type string `mapstructure:"type"`
	Settings map[string]interface{} `mapstructure:"settings"`
}

// Load loads configuration from a YAML file
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Set config file path
	v.SetConfigFile(configPath)

	// Enable environment variable support with PE prefix
	v.SetEnvPrefix("PE")
	v.AutomaticEnv()
	// Map environment variables to config keys (e.g., PE_XDS_SERVER_ADDRESS -> xds.server_address)
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

	// Capture complete raw config map for ${config} CEL resolution
	cfg.PolicyEngine.RawConfig = v.AllSettings()

	// Validate config
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default configuration values
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("policy_engine.server.extproc_port", 9001)

	// Admin defaults
	v.SetDefault("policy_engine.admin.enabled", true)
	v.SetDefault("policy_engine.admin.port", 9002)
	v.SetDefault("policy_engine.admin.allowed_ips", []string{"127.0.0.1", "::1"})

	// Config mode defaults
	v.SetDefault("policy_engine.config_mode.mode", "file")

	// xDS defaults
	v.SetDefault("policy_engine.xds.enabled", false)
	v.SetDefault("policy_engine.xds.server_address", "localhost:18000")
	v.SetDefault("policy_engine.xds.node_id", "policy-engine")
	v.SetDefault("policy_engine.xds.cluster", "policy-engine-cluster")
	v.SetDefault("policy_engine.xds.connect_timeout", "10s")
	v.SetDefault("policy_engine.xds.request_timeout", "5s")
	v.SetDefault("policy_engine.xds.initial_reconnect_delay", "1s")
	v.SetDefault("policy_engine.xds.max_reconnect_delay", "60s")
	v.SetDefault("policy_engine.xds.tls.enabled", false)

	// File config defaults
	v.SetDefault("policy_engine.file_config.path", "configs/policy-chains.yaml")

	// Logging defaults
	v.SetDefault("policy_engine.logging.level", "info")
	v.SetDefault("policy_engine.logging.format", "json")
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate server config
	if c.PolicyEngine.Server.ExtProcPort <= 0 || c.PolicyEngine.Server.ExtProcPort > 65535 {
		return fmt.Errorf("invalid extproc_port: %d (must be 1-65535)", c.PolicyEngine.Server.ExtProcPort)
	}

	// Validate admin config
	if c.PolicyEngine.Admin.Enabled {
		if c.PolicyEngine.Admin.Port <= 0 || c.PolicyEngine.Admin.Port > 65535 {
			return fmt.Errorf("invalid admin.port: %d (must be 1-65535)", c.PolicyEngine.Admin.Port)
		}
		if c.PolicyEngine.Admin.Port == c.PolicyEngine.Server.ExtProcPort {
			return fmt.Errorf("admin.port cannot be same as server.extproc_port")
		}
		if len(c.PolicyEngine.Admin.AllowedIPs) == 0 {
			return fmt.Errorf("admin.allowed_ips cannot be empty when admin is enabled")
		}
	}

	// Validate config mode
	if c.PolicyEngine.ConfigMode.Mode != "file" && c.PolicyEngine.ConfigMode.Mode != "xds" {
		return fmt.Errorf("invalid config_mode.mode: %s (must be 'file' or 'xds')", c.PolicyEngine.ConfigMode.Mode)
	}

	// Validate based on config mode
	if c.PolicyEngine.ConfigMode.Mode == "xds" {
		if !c.PolicyEngine.XDS.Enabled {
			return fmt.Errorf("xds.enabled must be true when config_mode.mode is 'xds'")
		}
		if err := c.validateXDSConfig(); err != nil {
			return err
		}
	} else if c.PolicyEngine.ConfigMode.Mode == "file" {
		if c.PolicyEngine.FileConfig.Path == "" {
			return fmt.Errorf("file_config.path is required when config_mode.mode is 'file'")
		}
	}

	// Validate logging
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.PolicyEngine.Logging.Level] {
		return fmt.Errorf("invalid logging.level: %s (must be debug, info, warn, or error)", c.PolicyEngine.Logging.Level)
	}

	validFormats := map[string]bool{"json": true, "text": true}
	if !validFormats[c.PolicyEngine.Logging.Format] {
		return fmt.Errorf("invalid logging.format: %s (must be json or text)", c.PolicyEngine.Logging.Format)
	}

	return nil
}

// validateXDSConfig validates xDS configuration
func (c *Config) validateXDSConfig() error {
	if c.PolicyEngine.XDS.ServerAddress == "" {
		return fmt.Errorf("xds.server_address is required when xDS is enabled")
	}

	if c.PolicyEngine.XDS.NodeID == "" {
		return fmt.Errorf("xds.node_id is required when xDS is enabled")
	}

	if c.PolicyEngine.XDS.Cluster == "" {
		return fmt.Errorf("xds.cluster is required when xDS is enabled")
	}

	if c.PolicyEngine.XDS.ConnectTimeout <= 0 {
		return fmt.Errorf("xds.connect_timeout must be positive")
	}

	if c.PolicyEngine.XDS.RequestTimeout <= 0 {
		return fmt.Errorf("xds.request_timeout must be positive")
	}

	if c.PolicyEngine.XDS.InitialReconnectDelay <= 0 {
		return fmt.Errorf("xds.initial_reconnect_delay must be positive")
	}

	if c.PolicyEngine.XDS.MaxReconnectDelay <= 0 {
		return fmt.Errorf("xds.max_reconnect_delay must be positive")
	}

	if c.PolicyEngine.XDS.TLS.Enabled {
		if c.PolicyEngine.XDS.TLS.CertPath == "" {
			return fmt.Errorf("xds.tls.cert_path is required when TLS is enabled")
		}
		if c.PolicyEngine.XDS.TLS.KeyPath == "" {
			return fmt.Errorf("xds.tls.key_path is required when TLS is enabled")
		}
		if c.PolicyEngine.XDS.TLS.CAPath == "" {
			return fmt.Errorf("xds.tls.ca_path is required when TLS is enabled")
		}
	}

	return nil
}
