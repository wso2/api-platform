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
	"log/slog"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	toml "github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/wso2/api-platform/common/collector"
)

const (
	// EnvPrefix is the prefix for environment variables used to configure the policy engine
	EnvPrefix = "APIP_GW_"
)

type Config struct {
	PolicyEngine         PolicyEngine           `koanf:"policy_engine"`
	GatewayController    map[string]interface{} `koanf:"gateway_controller"`
	PolicyConfigurations map[string]interface{} `koanf:"policy_configurations"`
	Collector            CollectorConfig        `koanf:"collector"`
	Analytics            AnalyticsConfig        `koanf:"analytics"`
	TrafficLogging       TrafficLoggingConfig   `koanf:"traffic_logging"`
	TracingConfig        TracingConfig          `koanf:"tracing"`
}

// CollectorConfig holds the data-collection ("collector") configuration. The
// collector is the shared capture pipeline that gathers request/response headers
// and bodies and ships them to the policy-engine over ALS. It underpins every
// consumer of that data (analytics and traffic logging) and is implicitly active
// whenever a consumer is enabled — see Config.IsCollectorEnabled. This section
// tunes capture and transport; it has no on/off flag of its own.
type CollectorConfig struct {
	// RequestBody / ResponseBody attach captured request/response bodies
	// onto the collected event.
	RequestBody  bool `koanf:"request_body"`
	ResponseBody bool `koanf:"response_body"`
	// Server tunes the policy-engine ALS receiver (the gRPC server that ingests
	// collected access logs). It is part of the collector transport and is
	// configured under the shared [collector.server] section (the controller
	// reads the same section to configure Envoy's sender side).
	Server AccessLogsServiceConfig `koanf:"server"`
}

// AnalyticsConfig holds analytics configuration
type AnalyticsConfig struct {
	Enabled            bool                      `koanf:"enabled"`
	EnabledPublishers  []string                  `koanf:"enabled_publishers"`
	Publishers         AnalyticsPublishersConfig `koanf:"publishers"`
	GRPCEventServerCfg map[string]interface{}    `koanf:"grpc_event_server"`
	// AccessLogsServiceCfg is a deprecated alias. ALS receiver tuning moved to
	// [collector.server]; when set here it is migrated onto the collector during
	// validation (with a warning). Prefer [collector.server].
	AccessLogsServiceCfg AccessLogsServiceConfig `koanf:"access_logs_service"`
	// AllowPayloads, SendRequestBody and SendResponseBody are deprecated aliases.
	// Body capture now lives under [collector]. When set, these are mapped onto
	// collector.request_body / collector.response_body during validation
	// (with a warning). Prefer the [collector] fields directly.
	AllowPayloads    bool `koanf:"allow_payloads"`
	SendRequestBody  bool `koanf:"send_request_body"`
	SendResponseBody bool `koanf:"send_response_body"`
}

// AnalyticsPublishersConfig holds configuration for all analytics publishers
type AnalyticsPublishersConfig struct {
	Moesif MoesifPublisherConfig `koanf:"moesif"`
}

// TrafficLoggingConfig holds configuration for the stdout traffic-logging feature,
// which writes each collected event to stdout as a JSON line. It is a consumer of
// the collector; enabling it implicitly activates the collector. There is a single
// mode: when Enabled, a stdout line is emitted for every request to every API, with
// no policy required — including requests denied by an auth policy short-circuit.
type TrafficLoggingConfig struct {
	// Enabled turns stdout JSON traffic logging on.
	Enabled bool `koanf:"enabled"`
	// MaskedHeaders lists header names (case-insensitive) whose values are
	// redacted in the logged requestHeaders/responseHeaders.
	MaskedHeaders []string `koanf:"masked_headers"`
	// MaxPayloadSize caps the number of bytes of request/response payload written
	// to the log line (0 = no limit). Truncation is applied at the publisher, so
	// the collector still captures the full body and other consumers (e.g. Moesif)
	// are unaffected.
	MaxPayloadSize int `koanf:"max_payload_size"`
	// RequestHeaders / RequestBody / ResponseHeaders / ResponseBody select which
	// captured fields are attached to the log line. Each is a no-op if the
	// corresponding [collector] capture flag is off — this directive can only
	// select among what the collector already captured.
	RequestHeaders  bool `koanf:"request_headers"`
	RequestBody     bool `koanf:"request_body"`
	ResponseHeaders bool `koanf:"response_headers"`
	ResponseBody    bool `koanf:"response_body"`
	// Fields is a field-projection layered on top of the flow selection above.
	Fields TrafficLoggingFieldsConfig `koanf:"fields"`
	// Properties adds extra key->value pairs to the emitted line's top-level
	// "properties" object. A value prefixed "$ctx:" is evaluated as a CEL
	// expression against a request-context surface built from the collected
	// event (see publishers.globalPropertyEvaluator), including a real auth.*
	// namespace backed by analytics metadata the collector system policy stamps
	// generically for any authenticated request; other values are emitted as
	// literal strings.
	Properties map[string]string `koanf:"properties"`
}

// TrafficLoggingFieldsConfig selects which fields appear in the traffic-log line.
// Exactly one of Only or Exclude should be set.
type TrafficLoggingFieldsConfig struct {
	Only    []string `koanf:"only"`
	Exclude []string `koanf:"exclude"`
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
	Server         ServerConfig         `koanf:"server"`
	Admin          AdminConfig          `koanf:"admin"`
	Metrics        MetricsConfig        `koanf:"metrics"`
	ConfigMode     ConfigModeConfig     `koanf:"config_mode"`
	XDS            XDSConfig            `koanf:"xds"`
	FileConfig     FileConfigConfig     `koanf:"file_config"`
	Logging        LoggingConfig        `koanf:"logging"`
	PythonExecutor PythonExecutorConfig `koanf:"python_executor"`
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

// PythonExecutorConfig holds configuration for the Python executor bridge.
// The Policy Engine uses this to connect to the Python executor process.
type PythonExecutorConfig struct {
	Server  PythonExecutorServerConfig `koanf:"server"`
	Timeout time.Duration              `koanf:"timeout"`
}

// PythonExecutorServerConfig holds Python executor connection configuration
type PythonExecutorServerConfig struct {
	// Mode is the connection mode: "uds" (default) or "tcp"
	Mode string `koanf:"mode"`

	// Port is the TCP port for the Python executor gRPC server (TCP mode only)
	Port int `koanf:"port"`

	// Host is the TCP host for the Python executor (TCP mode only, default: "localhost")
	Host string `koanf:"host"`

	// Path is the Unix Domain Socket path (UDS mode only)
	Path string `koanf:"path"`
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
	Mode string `koanf:"mode"` // Connection mode: "uds" (default) or "tcp"
	// ServerPort overrides the fixed ALS listener port (collector.ServerPort, 18090),
	// used only in "tcp" mode. Deprecated: no longer defaulted here or documented in
	// config-template.toml/Helm charts, so new deployments have no way to discover or
	// set it. Kept solely so a config that already sets it explicitly keeps working;
	// leave unset (0) to use the fixed port. Must match the gateway-controller's
	// collector.server.port override, or the two sides will fail to connect.
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

// defaultAccessLogsServiceConfig returns the default policy-engine ALS receiver tuning.
// Shared by the collector (canonical) and the deprecated [analytics].access_logs_service
// alias so a partial alias override migrates cleanly.
func defaultAccessLogsServiceConfig() AccessLogsServiceConfig {
	return AccessLogsServiceConfig{
		Mode:                  "",
		ShutdownTimeout:       600 * time.Second,
		PublicKeyPath:         "",
		PrivateKeyPath:        "",
		ALSPlainText:          true,
		ExtProcMaxMessageSize: 1000000000,
		ExtProcMaxHeaderLimit: 8192,
	}
}

// defaultConfig returns a Config struct with default configuration values
func defaultConfig() *Config {
	return &Config{
		PolicyEngine: PolicyEngine{
			Server: ServerConfig{
				Mode:        "",
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
			PythonExecutor: PythonExecutorConfig{
				Server: PythonExecutorServerConfig{
					Mode: "",
					Port: 9010,
					Host: "localhost",
				},
				Timeout: 30 * time.Second,
			},
			TracingServiceName: "policy-engine",
		},
		Collector: CollectorConfig{
			RequestBody:  false,
			ResponseBody: false,
			Server:       defaultAccessLogsServiceConfig(),
		},
		TrafficLogging: TrafficLoggingConfig{
			Enabled:         false,
			MaskedHeaders:   []string{},
			MaxPayloadSize:  0,
			RequestHeaders:  false,
			RequestBody:     false,
			ResponseHeaders: false,
			ResponseBody:    false,
			Fields: TrafficLoggingFieldsConfig{
				Only:    []string{},
				Exclude: []string{},
			},
			Properties: map[string]string{},
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
			// Deprecated alias: default mirrors the collector so a partial
			// [analytics.access_logs_service] override migrates cleanly.
			AccessLogsServiceCfg: defaultAccessLogsServiceConfig(),
			AllowPayloads:        false,
			SendRequestBody:      false,
			SendResponseBody:     false,
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
	// Validate policy engine connection mode
	switch c.PolicyEngine.Server.Mode {
	case "uds", "":
	case "tcp":
		if c.PolicyEngine.Server.ExtProcPort <= 0 || c.PolicyEngine.Server.ExtProcPort > 65535 {
			return fmt.Errorf("invalid extproc_port: %d (must be 1-65535)", c.PolicyEngine.Server.ExtProcPort)
		}
	default:
		return fmt.Errorf("server.mode must be 'uds' or 'tcp', got: %s", c.PolicyEngine.Server.Mode)
	}

	// Validate python executor config
	switch c.PolicyEngine.PythonExecutor.Server.Mode {
	case "uds", "":
	case "tcp":
		if c.PolicyEngine.PythonExecutor.Server.Host == "" {
			return fmt.Errorf("invalid policy_engine.python_executor.server.host: must be non-empty when mode = 'tcp'")
		}
		if c.PolicyEngine.PythonExecutor.Server.Port <= 0 || c.PolicyEngine.PythonExecutor.Server.Port > 65535 {
			return fmt.Errorf("invalid policy_engine.python_executor.server.port: %d (must be 1-65535)", c.PolicyEngine.PythonExecutor.Server.Port)
		}
	default:
		return fmt.Errorf("policy_engine.python_executor.server.mode must be 'uds' or 'tcp', got: %s", c.PolicyEngine.PythonExecutor.Server.Mode)
	}
	if c.PolicyEngine.PythonExecutor.Timeout <= 0 {
		return fmt.Errorf("policy_engine.python_executor.timeout must be positive")
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

	if err := c.validateCollectorConfig(); err != nil {
		return err
	}
	if c.TrafficLogging.MaxPayloadSize < 0 {
		return fmt.Errorf("traffic_logging.max_payload_size must be >= 0, got %d", c.TrafficLogging.MaxPayloadSize)
	}
	if err := c.validateTrafficLoggingConfig(); err != nil {
		return err
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

// validateCollectorConfig migrates deprecated analytics capture aliases onto the
// collector and enforces the collector prerequisite: a consumer (analytics or
// traffic logging) requires the collector that feeds it. The collector has no
// on/off flag of its own: it is implicitly active whenever a consumer is enabled
// (see IsCollectorEnabled), so its transport is validated only in that case.
func (c *Config) validateCollectorConfig() error {
	c.migrateDeprecatedAnalyticsCapture()
	c.migrateDeprecatedAnalyticsTransport()

	if c.IsCollectorEnabled() {
		if err := validateAccessLogsServiceConfig(c.Collector.Server); err != nil {
			return err
		}
	}
	return nil
}

// IsCollectorEnabled reports whether the collector should run. The collector is
// implicit: it is active whenever any consumer of the collected data is enabled
// (analytics or stdout traffic logging), and off otherwise.
func (c *Config) IsCollectorEnabled() bool {
	return collector.IsEnabled(c.Analytics.Enabled, c.TrafficLogging.Enabled)
}

// migrateDeprecatedAnalyticsTransport maps a deprecated [analytics].access_logs_service
// override onto the collector when the collector's receiver tuning is still at its
// default, so existing configs keep working after the transport moved to [collector].
// See collector.MigrateDeprecatedTransport for the shared (with the gateway-controller)
// migration logic and its guarding-while-analytics-enabled rationale.
func (c *Config) migrateDeprecatedAnalyticsTransport() {
	collector.MigrateDeprecatedTransport(
		c.Analytics.Enabled,
		c.Analytics.AccessLogsServiceCfg,
		&c.Collector.Server,
		defaultAccessLogsServiceConfig(),
		"analytics.access_logs_service",
	)
}

// validateAccessLogsServiceConfig validates the policy-engine ALS receiver tuning.
// The transport port is normally the fixed, non-configurable collector.ServerPort
// constant (see collector.ServerPort); als.ServerPort is a deprecated override honored
// only for backward compatibility with configs that already set it (see its doc comment).
func validateAccessLogsServiceConfig(als AccessLogsServiceConfig) error {
	switch als.Mode {
	case "uds", "tcp", "":
	default:
		return fmt.Errorf("collector.server.mode must be 'uds' or 'tcp', got: %s", als.Mode)
	}
	if als.ServerPort != 0 {
		slog.Warn("collector.server.server_port is deprecated and no longer documented; the ALS port is fixed at " +
			strconv.Itoa(collector.ServerPort) + " by default. Honoring the configured override for backward " +
			"compatibility — ensure the gateway-controller's collector.server.port matches, or the two sides will fail to connect.")
		if als.ServerPort < 0 || als.ServerPort > 65535 {
			return fmt.Errorf("collector.server.server_port must be between 1 and 65535, got %d", als.ServerPort)
		}
	}
	if als.ShutdownTimeout <= 0 {
		return fmt.Errorf("collector.server.shutdown_timeout must be positive, got %s", als.ShutdownTimeout)
	}
	if als.ExtProcMaxMessageSize <= 0 {
		return fmt.Errorf("collector.server.max_message_size must be positive, got %d", als.ExtProcMaxMessageSize)
	}
	if als.ExtProcMaxHeaderLimit <= 0 {
		return fmt.Errorf("collector.server.max_header_limit must be positive, got %d", als.ExtProcMaxHeaderLimit)
	}
	if als.ExtProcMaxHeaderLimit > math.MaxUint32 {
		return fmt.Errorf("collector.server.max_header_limit must be <= %d, got %d", uint64(math.MaxUint32), als.ExtProcMaxHeaderLimit)
	}
	return nil
}

// migrateDeprecatedAnalyticsCapture maps the deprecated analytics.allow_payloads /
// analytics.send_request_body / analytics.send_response_body onto the collector's
// body-capture flags, so existing configs keep working after capture settings
// moved under [collector]. See collector.MigrateDeprecatedCapture for the shared
// (with the gateway-controller) migration logic and its guarding-while-analytics-
// enabled rationale.
func (c *Config) migrateDeprecatedAnalyticsCapture() {
	collector.MigrateDeprecatedCapture(
		c.Analytics.Enabled,
		collector.CaptureFlags{
			SendRequestBody:  c.Analytics.SendRequestBody,
			SendResponseBody: c.Analytics.SendResponseBody,
			AllowPayloads:    c.Analytics.AllowPayloads,
		},
		&c.Collector.RequestBody,
		&c.Collector.ResponseBody,
	)
}

// validateAnalyticsConfig validates the analytics consumer configuration (publishers).
// ALS transport validation lives in validateCollectorConfig.
func (c *Config) validateAnalyticsConfig() error {
	if c.Analytics.Enabled {
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

// validateTrafficLoggingConfig validates the traffic-logging config and warns
// about settings that have no effect.
func (c *Config) validateTrafficLoggingConfig() error {
	tl := c.TrafficLogging

	if len(tl.Fields.Only) > 0 && len(tl.Fields.Exclude) > 0 {
		return fmt.Errorf("traffic_logging.fields: set either 'only' or 'exclude', not both")
	}

	if !tl.Enabled {
		if len(tl.Properties) > 0 {
			slog.Warn("traffic_logging.properties is set but traffic_logging.enabled is false; it has no effect")
		}
		return nil
	}

	if tl.RequestBody && !c.Collector.RequestBody {
		slog.Warn("traffic_logging.request_body is true but collector.request_body is false; " +
			"traffic logging can only select among what the collector captured, so no request body will be logged")
	}
	if tl.ResponseBody && !c.Collector.ResponseBody {
		slog.Warn("traffic_logging.response_body is true but collector.response_body is false; " +
			"traffic logging can only select among what the collector captured, so no response body will be logged")
	}

	return nil
}
