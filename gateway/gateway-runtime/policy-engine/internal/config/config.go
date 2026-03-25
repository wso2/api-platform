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
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	toml "github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

const (
	// EnvPrefix is the prefix for environment variables used to configure the policy engine
	EnvPrefix = "APIP_GW_"
)

type Config struct {
	PolicyEngine         PolicyEngine           `koanf:"policy_engine"`
	GatewayController    map[string]interface{} `koanf:"gateway_controller"`
	PolicyConfigurations map[string]interface{} `koanf:"policy_configurations"`
	Analytics            AnalyticsConfig        `koanf:"analytics"`
	TracingConfig        TracingConfig          `koanf:"tracing"`
}

// AnalyticsConfig holds analytics configuration
type AnalyticsConfig struct {
	Enabled              bool                      `koanf:"enabled"`
	EnabledPublishers    []string                  `koanf:"enabled_publishers"`
	Publishers           AnalyticsPublishersConfig `koanf:"publishers"`
	GRPCEventServerCfg   map[string]interface{}    `koanf:"grpc_event_server"`
	AccessLogsServiceCfg AccessLogsServiceConfig   `koanf:"access_logs_service"`
	// AllowPayloads controls whether request and response bodies are captured
	// into analytics metadata and forwarded to analytics publishers.
	AllowPayloads bool `koanf:"allow_payloads"`
}

// AnalyticsPublishersConfig holds configuration for all analytics publishers
type AnalyticsPublishersConfig struct {
	Moesif MoesifPublisherConfig `koanf:"moesif"`
}

// MoesifPublisherConfig holds Moesif-specific configuration
type MoesifPublisherConfig struct {
	ApplicationID      string `koanf:"application_id"`
	BaseURL            string `koanf:"moesif_base_url"`
	PublishInterval    int    `koanf:"publish_interval"`
	EventQueueSize     int    `koanf:"event_queue_size"`
	BatchSize          int    `koanf:"batch_size"`
	TimerWakeupSeconds int    `koanf:"timer_wakeup_seconds"`
}

// Config represents the complete policy engine configuration
type PolicyEngine struct {
	Server     ServerConfig     `koanf:"server"`
	Admin      AdminConfig      `koanf:"admin"`
	Metrics    MetricsConfig    `koanf:"metrics"`
	ConfigMode ConfigModeConfig `koanf:"config_mode"`
	XDS        XDSConfig        `koanf:"xds"`
	FileConfig FileConfigConfig `koanf:"file_config"`
	Logging    LoggingConfig    `koanf:"logging"`
	// Tracing holds OpenTelemetry exporter configuration
	TracingServiceName string `koanf:"tracing_service_name"`

	// RawConfig holds the complete raw configuration map including custom fields
	// This is used for resolving ${config} CEL expressions in policy systemParameters
	// Note: No struct tag - populated manually via k.Raw()
	RawConfig map[string]interface{}
}

// MetricsConfig holds Prometheus metrics server configuration
type MetricsConfig struct {
	// Enabled indicates whether the metrics server should be started
	Enabled bool `koanf:"enabled"`

	// Port is the port for the metrics HTTP server
	Port int `koanf:"port"`
}

// TracingConfig holds OpenTelemetry tracing configuration
type TracingConfig struct {
	// Enabled toggles tracing on/off
	Enabled bool `koanf:"enabled"`

	// Endpoint is the OTLP gRPC endpoint (host:port)
	Endpoint string `koanf:"endpoint"`

	// Insecure indicates whether to use an insecure connection (no TLS)
	Insecure bool `koanf:"insecure"`

	// ServiceVersion is the service version reported to the tracing backend
	ServiceVersion string `koanf:"service_version"`

	// BatchTimeout is the export batch timeout
	BatchTimeout time.Duration `koanf:"batch_timeout"`

	// MaxExportBatchSize is the maximum batch size for exports
	MaxExportBatchSize int `koanf:"max_export_batch_size"`

	// SamplingRate is the ratio of requests to sample (0.0 to 1.0)
	// 1.0 = sample all requests, 0.1 = sample 10% of requests
	// If set to 0 or not specified, defaults to 1.0 (sample all)
	SamplingRate float64 `koanf:"sampling_rate"`
}

// ServerConfig holds ext_proc server configuration
type ServerConfig struct {
	// Mode is the connection mode: "uds" (default) or "tcp"
	// In UDS mode, the socket path is a constant (not configurable)
	Mode string `koanf:"mode"`

	// ExtProcPort is the port for the ext_proc gRPC server (TCP mode only)
	ExtProcPort int `koanf:"extproc_port"`
}

// AdminConfig holds admin HTTP server configuration
type AdminConfig struct {
	// Enabled indicates whether the admin server should be started
	Enabled bool `koanf:"enabled"`

	// Port is the port for the admin HTTP server
	Port int `koanf:"port"`

	// AllowedIPs is a list of IP addresses allowed to access the admin API
	// Defaults to localhost only (127.0.0.1 and ::1)
	AllowedIPs []string `koanf:"allowed_ips"`
}

// ConfigModeConfig specifies how policy chains are configured
type ConfigModeConfig struct {
	// Mode can be "file" or "xds"
	Mode string `koanf:"mode"`
}

// XDSConfig holds xDS client configuration
type XDSConfig struct {
	// ConnectTimeout is the timeout for establishing initial connection
	ConnectTimeout time.Duration `koanf:"connect_timeout"`

	// RequestTimeout is the timeout for individual xDS requests
	RequestTimeout time.Duration `koanf:"request_timeout"`

	// InitialReconnectDelay is the initial delay before reconnecting
	InitialReconnectDelay time.Duration `koanf:"initial_reconnect_delay"`

	// MaxReconnectDelay is the maximum delay between reconnection attempts
	MaxReconnectDelay time.Duration `koanf:"max_reconnect_delay"`

	// TLS configuration
	TLS XDSTLSConfig `koanf:"tls"`
}

// XDSTLSConfig holds TLS configuration for xDS connection
type XDSTLSConfig struct {
	// Enabled indicates whether to use TLS
	Enabled bool `koanf:"enabled"`

	// CertPath is the path to the TLS certificate file
	CertPath string `koanf:"cert_path"`

	// KeyPath is the path to the TLS private key file
	KeyPath string `koanf:"key_path"`

	// CAPath is the path to the CA certificate for server verification
	CAPath string `koanf:"ca_path"`
}

// FileConfigConfig holds file-based configuration settings
type FileConfigConfig struct {
	// Path is the path to the policy chains YAML file
	Path string `koanf:"path"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	// Level can be "debug", "info", "warn", "error"
	Level string `koanf:"level"`

	// Format can be "json" or "text"
	Format string `koanf:"format"`
}

// AccessLogsServiceConfig holds access logs service configuration
type AccessLogsServiceConfig struct {
	Mode                  string        `koanf:"mode"` // Connection mode: "uds" (default) or "tcp"
	ServerPort            int           `koanf:"server_port"`
	ShutdownTimeout       time.Duration `koanf:"shutdown_timeout"`
	PublicKeyPath         string        `koanf:"public_key_path"`
	PrivateKeyPath        string        `koanf:"private_key_path"`
	ALSPlainText          bool          `koanf:"als_plain_text"`
	ExtProcMaxMessageSize int           `koanf:"max_message_size"`
	ExtProcMaxHeaderLimit int           `koanf:"max_header_limit"`
}

// Load loads configuration from file, environment variables, and defaults
// Priority: Environment variables > Config file > Defaults
//
// The configuration supports Go-style duration strings (e.g., "10s", "5m", "1h")
// for all duration fields. The DecodeHook automatically converts string durations
// to time.Duration values before assignment.
func Load(configPath string) (*Config, error) {
	cfg := defaultConfig()

	k := koanf.New(".")

	// Load config file if path is provided
	if configPath != "" {
		if err := k.Load(file.Provider(configPath), toml.Parser()); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Load environment variables with the prefix
	// Double underscores (__) preserve literal underscores in field names
	if err := k.Load(env.Provider(EnvPrefix, ".", func(s string) string {
		s = strings.TrimPrefix(s, EnvPrefix)
		s = strings.ToLower(s)

		// Step 1: Preserve literal underscores with placeholder
		s = strings.ReplaceAll(s, "__", "%UNDERSCORE%")
		// Step 2: Convert single underscores to dots (nested paths)
		s = strings.ReplaceAll(s, "_", ".")
		// Step 3: Restore literal underscores
		s = strings.ReplaceAll(s, "%UNDERSCORE%", "_")
		return s
	}), nil); err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	// Unmarshal into pre-populated config struct with defaults
	// Koanf will merge: fields from file/env overwrite defaults, unset fields keep defaults
	if err := k.UnmarshalWithConf("", cfg, koanf.UnmarshalConf{
		DecoderConfig: &mapstructure.DecoderConfig{
			TagName:          "koanf",
			WeaklyTypedInput: true,
			Result:           cfg,
			DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Capture complete raw config for CEL ${config} expression resolution
	cfg.PolicyEngine.RawConfig = k.Raw()

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// defaultConfig returns a Config struct with default configuration values
func defaultConfig() *Config {
	return &Config{
		PolicyEngine: PolicyEngine{
			Server: ServerConfig{
				Mode:        "", // Empty defaults to "uds"
				ExtProcPort: 9001,
			},
			Admin: AdminConfig{
				Enabled:    true,
				Port:       9002,
				AllowedIPs: []string{"*"},
			},
			Metrics: MetricsConfig{
				Enabled: false,
				Port:    9003,
			},
			ConfigMode: ConfigModeConfig{
				Mode: "xds",
			},
			XDS: XDSConfig{
				ConnectTimeout:        10 * time.Second,
				RequestTimeout:        5 * time.Second,
				InitialReconnectDelay: 1 * time.Second,
				MaxReconnectDelay:     60 * time.Second,
				TLS: XDSTLSConfig{
					Enabled: false,
				},
			},
			FileConfig: FileConfigConfig{
				Path: "",
			},
			Logging: LoggingConfig{
				Level:  "info",
				Format: "text",
			},
			TracingServiceName: "policy-engine",
		},
		Analytics: AnalyticsConfig{
			Enabled:           false,
			EnabledPublishers: []string{"moesif"},
			Publishers: AnalyticsPublishersConfig{
				Moesif: MoesifPublisherConfig{
					ApplicationID:      "",
					BaseURL:            "https://api.moesif.net",
					PublishInterval:    5,
					EventQueueSize:     10000,
					BatchSize:          50,
					TimerWakeupSeconds: 3,
				},
			},
			GRPCEventServerCfg: map[string]interface{}{
				"server_port":           18090,
				"buffer_flush_interval": 1000000000,
				"buffer_size_bytes":     16384,
				"grpc_request_timeout":  20000000000,
			},
			AccessLogsServiceCfg: AccessLogsServiceConfig{
				Mode:                  "", // Empty defaults to "uds"
				ServerPort:            18090,
				ShutdownTimeout:       600 * time.Second,
				PublicKeyPath:         "",
				PrivateKeyPath:        "",
				ALSPlainText:          true,
				ExtProcMaxMessageSize: 1000000000,
				ExtProcMaxHeaderLimit: 8192,
			},
			AllowPayloads: false,
		},
		TracingConfig: TracingConfig{
			Enabled:            false,
			Endpoint:           "otel-collector:4317",
			Insecure:           true,
			ServiceVersion:     "1.0.0",
			BatchTimeout:       1 * time.Second,
			MaxExportBatchSize: 512,
			SamplingRate:       1.0,
		},
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate connection mode (same pattern as gateway-controller)
	switch c.PolicyEngine.Server.Mode {
	case "uds", "":
		// UDS mode (default) - socket path is a constant, no additional validation needed
	case "tcp":
		// TCP mode - validate port
		if c.PolicyEngine.Server.ExtProcPort <= 0 || c.PolicyEngine.Server.ExtProcPort > 65535 {
			return fmt.Errorf("invalid extproc_port: %d (must be 1-65535)", c.PolicyEngine.Server.ExtProcPort)
		}
	default:
		return fmt.Errorf("server.mode must be 'uds' or 'tcp', got: %s", c.PolicyEngine.Server.Mode)
	}

	// Validate admin config
	if c.PolicyEngine.Admin.Enabled {
		if c.PolicyEngine.Admin.Port <= 0 || c.PolicyEngine.Admin.Port > 65535 {
			return fmt.Errorf("invalid admin.port: %d (must be 1-65535)", c.PolicyEngine.Admin.Port)
		}
		// Only check port conflict if using TCP mode
		if c.PolicyEngine.Server.Mode == "tcp" && c.PolicyEngine.Admin.Port == c.PolicyEngine.Server.ExtProcPort {
			return fmt.Errorf("admin.port cannot be same as server.extproc_port")
		}
		if len(c.PolicyEngine.Admin.AllowedIPs) == 0 {
			return fmt.Errorf("admin.allowed_ips cannot be empty when admin is enabled")
		}
	}

	// Validate metrics config
	if c.PolicyEngine.Metrics.Enabled {
		if c.PolicyEngine.Metrics.Port <= 0 || c.PolicyEngine.Metrics.Port > 65535 {
			return fmt.Errorf("invalid metrics.port: %d (must be 1-65535)", c.PolicyEngine.Metrics.Port)
		}
		// Only check port conflict if using TCP mode
		if c.PolicyEngine.Server.Mode == "tcp" && c.PolicyEngine.Metrics.Port == c.PolicyEngine.Server.ExtProcPort {
			return fmt.Errorf("metrics.port cannot be same as server.extproc_port")
		}
		if c.PolicyEngine.Metrics.Port == c.PolicyEngine.Admin.Port {
			return fmt.Errorf("metrics.port cannot be same as admin.port")
		}
	}

	// Validate config mode
	if c.PolicyEngine.ConfigMode.Mode != "file" && c.PolicyEngine.ConfigMode.Mode != "xds" {
		return fmt.Errorf("invalid config_mode.mode: %s (must be 'file' or 'xds')", c.PolicyEngine.ConfigMode.Mode)
	}

	// Validate based on config mode
	if c.PolicyEngine.ConfigMode.Mode == "xds" {
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

	if c.Analytics.Enabled {
		if err := c.validateAnalyticsConfig(); err != nil {
			return fmt.Errorf("analytics configuration validation failed: %v", err)
		}
	}
	if c.TracingConfig.Enabled {
		if c.TracingConfig.Endpoint == "" {
			return fmt.Errorf("tracing.endpoint is required when tracing is enabled")
		}
		if c.TracingConfig.BatchTimeout <= 0 {
			return fmt.Errorf("tracing.batch_timeout must be positive")
		}
		if c.TracingConfig.MaxExportBatchSize <= 0 {
			return fmt.Errorf("tracing.max_export_batch_size must be positive")
		}
		if c.TracingConfig.SamplingRate <= 0.0 || c.TracingConfig.SamplingRate > 1.0 {
			return fmt.Errorf("tracing.sampling_rate must be > 0.0 and <= 1.0, got %f", c.TracingConfig.SamplingRate)
		}
	}

	return nil
}

// validateXDSConfig validates xDS configuration
func (c *Config) validateXDSConfig() error {
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

// validateAnalyticsConfig validates the analytics configuration
func (c *Config) validateAnalyticsConfig() error {
	// Validate analytics configuration
	if c.Analytics.Enabled {
		// Validate ALS server config (policy-engine side)
		als := c.Analytics.AccessLogsServiceCfg

		// Validate ALS connection mode
		switch als.Mode {
		case "uds", "":
			// UDS mode (default) - port is unused
		case "tcp":
			// TCP mode - validate port
			if als.ServerPort <= 0 || als.ServerPort > 65535 {
				return fmt.Errorf("analytics.access_logs_service.server_port must be between 1 and 65535, got %d", als.ServerPort)
			}
		default:
			return fmt.Errorf("analytics.access_logs_service.mode must be 'uds' or 'tcp', got: %s", als.Mode)
		}
		if als.ShutdownTimeout <= 0 {
			return fmt.Errorf("analytics.access_logs_service.shutdown_timeout must be positive, got %s", als.ShutdownTimeout)
		}
		if als.ExtProcMaxMessageSize <= 0 {
			return fmt.Errorf("analytics.access_logs_service.max_message_size must be positive, got %d", als.ExtProcMaxMessageSize)
		}
		if als.ExtProcMaxHeaderLimit <= 0 {
			return fmt.Errorf("analytics.access_logs_service.max_header_limit must be positive, got %d", als.ExtProcMaxHeaderLimit)
		}
		if als.ExtProcMaxHeaderLimit > math.MaxUint32 {
			return fmt.Errorf("analytics.access_logs_service.max_header_limit must be <= %d, got %d", uint64(math.MaxUint32), als.ExtProcMaxHeaderLimit)
		}

		// Validate enabled publishers
		for _, publisherName := range c.Analytics.EnabledPublishers {
			switch publisherName {
			case "moesif":
				moesifCfg := c.Analytics.Publishers.Moesif
				if moesifCfg.ApplicationID == "" {
					return fmt.Errorf("analytics.publishers.moesif.application_id is required when moesif is enabled")
				}
				if moesifCfg.PublishInterval <= 0 {
					return fmt.Errorf("analytics.publishers.moesif.publish_interval must be > 0 seconds, got %d", moesifCfg.PublishInterval)
				}
				if moesifCfg.BaseURL != "" {
					if u, err := url.Parse(moesifCfg.BaseURL); err != nil || u.Scheme == "" || u.Host == "" {
						return fmt.Errorf("analytics.publishers.moesif.moesif_base_url must be a valid URL (e.g. https://api.moesif.net), got %q", moesifCfg.BaseURL)
					}
				}
			default:
				return fmt.Errorf("unknown publisher type in enabled_publishers: %s", publisherName)
			}
		}
	}
	return nil
}
