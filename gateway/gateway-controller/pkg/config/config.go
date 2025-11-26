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
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
)

// Config holds all configuration for the gateway-controller
type Config struct {
	Server       ServerConfig       `koanf:"server"`
	Storage      StorageConfig      `koanf:"storage"`
	Router       RouterConfig       `koanf:"router"`
	Logging      LoggingConfig      `koanf:"logging"`
	ControlPlane ControlPlaneConfig `koanf:"controlplane"`
	PolicyServer PolicyServerConfig `koanf:"policyserver"`
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	APIPort         int           `koanf:"api_port"`
	XDSPort         int           `koanf:"xds_port"`
	ShutdownTimeout time.Duration `koanf:"shutdown_timeout"`
}

// PolicyServerConfig holds policy xDS server-related configuration
type PolicyServerConfig struct {
	Enabled bool            `koanf:"enabled"`
	Port    int             `koanf:"port"`
	TLS     PolicyServerTLS `koanf:"tls"`
}

// PolicyServerTLS holds TLS configuration for the policy xDS server
type PolicyServerTLS struct {
	Enabled  bool   `koanf:"enabled"`
	CertFile string `koanf:"cert_file"`
	KeyFile  string `koanf:"key_file"`
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
	AccessLogs   AccessLogsConfig   `koanf:"access_logs"`
	ListenerPort int                `koanf:"listener_port"`
	GatewayHost  string             `koanf:"gateway_host"`
	Upstream     envoyUpstream      `koanf:"envoy_upstream"`
	PolicyEngine PolicyEngineConfig `koanf:"policy_engine"`
}

// envoyUpstream holds envoy upstream related configurations
type envoyUpstream struct {
	// UpstreamTLS related Configuration
	TLS      upstreamTLS     `koanf:"tls"`
	Timeouts upstreamTimeout `koanf:"timeouts"`
}

// upstreamTLS holds envoy upstream TLS related configurations
type upstreamTLS struct {
	MinimumProtocolVersion string `koanf:"minimum_protocol_version"`
	MaximumProtocolVersion string `koanf:"maximum_protocol_version"`
	Ciphers                string `koanf:"ciphers"`
	TrustedCertPath        string `koanf:"trusted_cert_path"`
	VerifyHostName         bool   `koanf:"verify_host_name"`
	DisableSslVerification bool   `koanf:"disable_ssl_verification"`
}

// upstreamTimeout holds envoy upstream timeout related configurations
type upstreamTimeout struct {
	RouteTimeoutInSeconds     uint32 `koanf:"route_timeout_in_seconds"`
	MaxRouteTimeoutInSeconds  uint32 `koanf:"max_route_timeout_in_seconds"`
	RouteIdleTimeoutInSeconds uint32 `koanf:"route_idle_timeout_in_seconds"`
}

// PolicyEngineConfig holds policy engine ext_proc filter configuration
type PolicyEngineConfig struct {
	Enabled           bool            `koanf:"enabled"`
	Host              string          `koanf:"host"` // Policy engine hostname/IP
	Port              uint32          `koanf:"port"` // Policy engine ext_proc port
	TimeoutMs         uint32          `koanf:"timeout_ms"`
	FailureModeAllow  bool            `koanf:"failure_mode_allow"`
	RouteCacheAction  string          `koanf:"route_cache_action"`
	AllowModeOverride bool            `koanf:"allow_mode_override"`
	RequestHeaderMode string          `koanf:"request_header_mode"`
	MessageTimeoutMs  uint32          `koanf:"message_timeout_ms"`
	TLS               PolicyEngineTLS `koanf:"tls"` // TLS configuration
}

// PolicyEngineTLS holds policy engine TLS configuration
type PolicyEngineTLS struct {
	Enabled    bool   `koanf:"enabled"`     // Enable TLS for policy engine connection
	CertPath   string `koanf:"cert_path"`   // Path to client certificate (mTLS)
	KeyPath    string `koanf:"key_path"`    // Path to client private key (mTLS)
	CAPath     string `koanf:"ca_path"`     // Path to CA certificate for server validation
	ServerName string `koanf:"server_name"` // SNI server name (optional, defaults to host)
	SkipVerify bool   `koanf:"skip_verify"` // Skip server certificate verification (insecure, dev only)
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

// ControlPlaneConfig holds control plane connection configuration
type ControlPlaneConfig struct {
	Host               string        `koanf:"host"`                 // Control plane hostname
	Token              string        `koanf:"token"`                // Registration token (api-key)
	ReconnectInitial   time.Duration `koanf:"reconnect_initial"`    // Initial retry delay
	ReconnectMax       time.Duration `koanf:"reconnect_max"`        // Maximum retry delay
	PollingInterval    time.Duration `koanf:"polling_interval"`     // Reconciliation polling interval
	InsecureSkipVerify bool          `koanf:"insecure_skip_verify"` // Skip TLS certificate verification (default: true for dev)
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

	// Load config file if path is provided
	if configPath != "" {
		if _, err := os.Stat(configPath); err != nil {
			return nil, fmt.Errorf("config file not found: %s", configPath)
		}
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

	// Load environment variables with prefix "GATEWAY_"
	// Example: GATEWAY_SERVER_API_PORT=9090 -> server.api_port
	//          GATEWAY_CONTROL_PLANE_URL=wss://... -> controlplane.url
	if err := k.Load(env.Provider("GATEWAY_", ".", func(s string) string {
		s = strings.TrimPrefix(s, "GATEWAY_")
		s = strings.ToLower(s)

		// Custom mappings for control plane variables
		switch s {
		case "control_plane_url":
			return "controlplane.url"
		case "registration_token":
			return "controlplane.token"
		case "reconnect_initial":
			return "controlplane.reconnect_initial"
		case "reconnect_max":
			return "controlplane.reconnect_max"
		case "polling_interval":
			return "controlplane.polling_interval"
		case "insecure_skip_verify":
			return "controlplane.insecure_skip_verify"
		default:
			// For other GATEWAY_ prefixed vars, use standard mapping (underscore to dot)
			s = strings.ReplaceAll(s, "_", ".")
			return s
		}
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
		"policyserver.enabled":       true,
		"policyserver.port":          18001,
		"storage.type":               "memory",
		"storage.sqlite.path":        "./data/gateway.db",
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
		"router.listener_port":                                         8080,
		"router.gateway_host":                                          "*",
		"logging.level":                                                "info",
		"logging.format":                                               "json",
		"controlplane.host":                                            "localhost:8443",
		"controlplane.token":                                           "",
		"controlplane.reconnect_initial":                               "1s",
		"controlplane.reconnect_max":                                   "5m",
		"controlplane.polling_interval":                                "15m",
		"controlplane.insecure_skip_verify":                            true, // Default true for dev environments with self-signed certs
		"router.envoy_upstream.tls.minimum_protocol_version":           "TLS1_2",
		"router.envoy_upstream.tls.maximum_protocol_version":           "TLS1_3",
		"router.envoy_upstream.tls.verify_host_name":                   true,
		"router.envoy_upstream.tls.disable_ssl_verification":           false,
		"router.envoy_upstream.timeouts.route_timeout_in_seconds":      60,
		"router.envoy_upstream.timeouts.max_route_timeout_in_seconds":  60,
		"router.envoy_upstream.timeouts.route_idle_timeout_in_seconds": 300,
		"router.policy_engine.enabled":                                 false,
		"router.policy_engine.host":                                    "localhost",
		"router.policy_engine.port":                                    9001,
		"router.policy_engine.timeout_ms":                              250,
		"router.policy_engine.failure_mode_allow":                      false,
		"router.policy_engine.route_cache_action":                      "RETAIN",
		"router.policy_engine.allow_mode_override":                     true,
		"router.policy_engine.request_header_mode":                     "SEND",
		"router.policy_engine.message_timeout_ms":                      250,
		"router.policy_engine.tls.enabled":                             false,
		"router.policy_engine.tls.cert_path":                           "",
		"router.policy_engine.tls.key_path":                            "",
		"router.policy_engine.tls.ca_path":                             "",
		"router.policy_engine.tls.server_name":                         "",
		"router.policy_engine.tls.skip_verify":                         false,
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
			if len(c.Router.AccessLogs.JSONFields) == 0 {
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

	// Validate control plane configuration
	if err := c.validateControlPlaneConfig(); err != nil {
		return err
	}

	// Validate TLS configuration
	if err := c.validateTLSConfig(); err != nil {
		return err
	}

	// Validate timeout configuration
	if err := c.validateTimeoutConfig(); err != nil {
		return err
	}

	// Validate policy engine configuration
	if err := c.validatePolicyEngineConfig(); err != nil {
		return err
	}

	return nil
}

// validateControlPlaneConfig validates the control plane configuration
func (c *Config) validateControlPlaneConfig() error {
	// Host validation - required if control plane is configured
	if c.ControlPlane.Host == "" {
		return fmt.Errorf("controlplane.host is required")
	}

	// Token is optional - gateway can run without control plane connection
	// If token is empty, connection will not be established

	// Validate reconnection intervals
	if c.ControlPlane.ReconnectInitial <= 0 {
		return fmt.Errorf("controlplane.reconnect_initial must be positive, got: %s", c.ControlPlane.ReconnectInitial)
	}

	if c.ControlPlane.ReconnectMax <= 0 {
		return fmt.Errorf("controlplane.reconnect_max must be positive, got: %s", c.ControlPlane.ReconnectMax)
	}

	if c.ControlPlane.ReconnectInitial > c.ControlPlane.ReconnectMax {
		return fmt.Errorf("controlplane.reconnect_initial (%s) must be <= controlplane.reconnect_max (%s)",
			c.ControlPlane.ReconnectInitial, c.ControlPlane.ReconnectMax)
	}

	// Validate polling interval
	if c.ControlPlane.PollingInterval <= 0 {
		return fmt.Errorf("controlplane.polling_interval must be positive, got: %s", c.ControlPlane.PollingInterval)
	}

	return nil
}

// validateTLSConfig validates the TLS configuration
func (c *Config) validateTLSConfig() error {
	// Validate TLS protocol versions
	validTLSVersions := []string{
		constants.TLSVersion10,
		constants.TLSVersion11,
		constants.TLSVersion12,
		constants.TLSVersion13,
	}

	// Validate minimum TLS version
	minVersion := c.Router.Upstream.TLS.MinimumProtocolVersion
	if minVersion == "" {
		return fmt.Errorf("router.envoy_upstream.tls.minimum_protocol_version is required")
	}

	isValidMinVersion := false
	for _, version := range validTLSVersions {
		if minVersion == version {
			isValidMinVersion = true
			break
		}
	}
	if !isValidMinVersion {
		return fmt.Errorf("router.envoy_upstream.tls.minimum_protocol_version must be one of: %s, got: %s",
			strings.Join(validTLSVersions, ", "), minVersion)
	}

	// Validate maximum TLS version
	maxVersion := c.Router.Upstream.TLS.MaximumProtocolVersion
	if maxVersion == "" {
		return fmt.Errorf("router.envoy_upstream.tls.maximum_protocol_version is required")
	}

	isValidMaxVersion := false
	for _, version := range validTLSVersions {
		if maxVersion == version {
			isValidMaxVersion = true
			break
		}
	}
	if !isValidMaxVersion {
		return fmt.Errorf("router.envoy_upstream.tls.maximum_protocol_version must be one of: %s, got: %s",
			strings.Join(validTLSVersions, ", "), maxVersion)
	}

	// Validate that minimum version is not greater than maximum version
	tlsVersionOrder := map[string]int{
		constants.TLSVersion10: constants.TLSVersionOrderTLS10,
		constants.TLSVersion11: constants.TLSVersionOrderTLS11,
		constants.TLSVersion12: constants.TLSVersionOrderTLS12,
		constants.TLSVersion13: constants.TLSVersionOrderTLS13,
	}

	if tlsVersionOrder[minVersion] > tlsVersionOrder[maxVersion] {
		return fmt.Errorf("router.envoy_upstream.tls.minimum_protocol_version (%s) cannot be greater than maximum_protocol_version (%s)",
			minVersion, maxVersion)
	}

	// Validate cipher suites format (basic validation - ensure it's not empty if provided)
	ciphers := c.Router.Upstream.TLS.Ciphers
	if ciphers != "" {
		// Basic validation: ensure ciphers don't contain invalid characters
		if strings.Contains(ciphers, constants.CipherInvalidChars1) || strings.Contains(ciphers, constants.CipherInvalidChars2) {
			return fmt.Errorf("router.envoy_upstream.tls.ciphers contains invalid characters (use comma-separated values)")
		}

		// Ensure cipher list is not just whitespace
		if strings.TrimSpace(ciphers) == "" {
			return fmt.Errorf("router.envoy_upstream.tls.ciphers cannot be empty or whitespace only")
		}
	}

	// Validate trusted cert path if SSL verification is enabled
	if !c.Router.Upstream.TLS.DisableSslVerification && c.Router.Upstream.TLS.TrustedCertPath == "" {
		return fmt.Errorf("router.envoy_upstream.tls.trusted_cert_path is required when SSL verification is enabled")
	}

	return nil
}

// validateTimeoutConfig validates the timeout configuration
func (c *Config) validateTimeoutConfig() error {
	timeouts := c.Router.Upstream.Timeouts

	// Validate route timeout
	if timeouts.RouteTimeoutInSeconds <= 0 {
		return fmt.Errorf("router.envoy_upstream.timeouts.route_timeout_in_seconds must be positive, got: %d",
			timeouts.RouteTimeoutInSeconds)
	}

	// Validate max route timeout
	if timeouts.MaxRouteTimeoutInSeconds <= 0 {
		return fmt.Errorf("router.envoy_upstream.timeouts.max_route_timeout_in_seconds must be positive, got: %d",
			timeouts.MaxRouteTimeoutInSeconds)
	}

	// Validate idle timeout
	if timeouts.RouteIdleTimeoutInSeconds <= 0 {
		return fmt.Errorf("router.envoy_upstream.timeouts.route_idle_timeout_in_seconds must be positive, got: %d",
			timeouts.RouteIdleTimeoutInSeconds)
	}

	// Validate that route timeout is not greater than max route timeout
	if timeouts.RouteTimeoutInSeconds > timeouts.MaxRouteTimeoutInSeconds {
		return fmt.Errorf("router.envoy_upstream.timeouts.route_timeout_in_seconds (%d) cannot be greater than max_route_timeout_in_seconds (%d)",
			timeouts.RouteTimeoutInSeconds, timeouts.MaxRouteTimeoutInSeconds)
	}

	// Validate reasonable timeout ranges (prevent extremely long timeouts)
	if timeouts.RouteTimeoutInSeconds > constants.MaxReasonableTimeoutSeconds {
		return fmt.Errorf("router.envoy_upstream.timeouts.route_timeout_in_seconds (%d) exceeds maximum reasonable timeout of %d seconds",
			timeouts.RouteTimeoutInSeconds, constants.MaxReasonableTimeoutSeconds)
	}

	if timeouts.MaxRouteTimeoutInSeconds > constants.MaxReasonableTimeoutSeconds {
		return fmt.Errorf("router.envoy_upstream.timeouts.max_route_timeout_in_seconds (%d) exceeds maximum reasonable timeout of %d seconds",
			timeouts.MaxRouteTimeoutInSeconds, constants.MaxReasonableTimeoutSeconds)
	}

	if timeouts.RouteIdleTimeoutInSeconds > constants.MaxReasonableTimeoutSeconds {
		return fmt.Errorf("router.envoy_upstream.timeouts.route_idle_timeout_in_seconds (%d) exceeds maximum reasonable timeout of %d seconds",
			timeouts.RouteIdleTimeoutInSeconds, constants.MaxReasonableTimeoutSeconds)
	}

	return nil
}

// validatePolicyEngineConfig validates the policy engine configuration
func (c *Config) validatePolicyEngineConfig() error {
	policyEngine := c.Router.PolicyEngine

	// If policy engine is disabled, skip validation
	if !policyEngine.Enabled {
		return nil
	}

	// Validate host
	if policyEngine.Host == "" {
		return fmt.Errorf("router.policy_engine.host is required when policy engine is enabled")
	}

	// Validate port
	if policyEngine.Port == 0 {
		return fmt.Errorf("router.policy_engine.port is required when policy engine is enabled")
	}

	if policyEngine.Port > 65535 {
		return fmt.Errorf("router.policy_engine.port must be between 1 and 65535, got: %d", policyEngine.Port)
	}

	// Validate timeout
	if policyEngine.TimeoutMs <= 0 {
		return fmt.Errorf("router.policy_engine.timeout_ms must be positive, got: %d", policyEngine.TimeoutMs)
	}

	if policyEngine.TimeoutMs > constants.MaxReasonablePolicyTimeoutMs {
		return fmt.Errorf("router.policy_engine.timeout_ms (%d) exceeds maximum reasonable timeout of %d ms",
			policyEngine.TimeoutMs, constants.MaxReasonablePolicyTimeoutMs)
	}

	// Validate message timeout
	if policyEngine.MessageTimeoutMs <= 0 {
		return fmt.Errorf("router.policy_engine.message_timeout_ms must be positive, got: %d", policyEngine.MessageTimeoutMs)
	}

	if policyEngine.MessageTimeoutMs > constants.MaxReasonablePolicyTimeoutMs {
		return fmt.Errorf("router.policy_engine.message_timeout_ms (%d) exceeds maximum reasonable timeout of %d ms",
			policyEngine.MessageTimeoutMs, constants.MaxReasonablePolicyTimeoutMs)
	}

	// Validate TLS configuration if enabled
	if policyEngine.TLS.Enabled {
		// For mTLS, both cert and key are required
		if policyEngine.TLS.CertPath != "" && policyEngine.TLS.KeyPath == "" {
			return fmt.Errorf("router.policy_engine.tls.key_path is required when cert_path is provided")
		}
		if policyEngine.TLS.KeyPath != "" && policyEngine.TLS.CertPath == "" {
			return fmt.Errorf("router.policy_engine.tls.cert_path is required when key_path is provided")
		}

		// CA path is optional but recommended for production
		if policyEngine.TLS.CAPath == "" && !policyEngine.TLS.SkipVerify {
			// Warning: No CA provided and not skipping verification
			// This might fail in production with self-signed certs
		}
	}

	// Validate route cache action
	validRouteCacheActions := []string{"DEFAULT", "RETAIN", "CLEAR"}
	isValidAction := false
	for _, action := range validRouteCacheActions {
		if policyEngine.RouteCacheAction == action {
			isValidAction = true
			break
		}
	}
	if !isValidAction {
		return fmt.Errorf("router.policy_engine.route_cache_action must be one of: DEFAULT, RETAIN, CLEAR, got: %s",
			policyEngine.RouteCacheAction)
	}

	// Validate request header mode
	validHeaderModes := []string{"DEFAULT", "SEND", "SKIP"}
	isValidMode := false
	for _, mode := range validHeaderModes {
		if policyEngine.RequestHeaderMode == mode {
			isValidMode = true
			break
		}
	}
	if !isValidMode {
		return fmt.Errorf("router.policy_engine.request_header_mode must be one of: DEFAULT, SEND, SKIP, got: %s",
			policyEngine.RequestHeaderMode)
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

// IsPolicyEngineEnabled returns true if policy engine is enabled
func (c *Config) IsPolicyEngineEnabled() bool {
	return c.Router.PolicyEngine.Enabled
}

// GetPolicyDirectory returns the directory path where policy definition files are stored
// Defaults to "policies" if not configured via environment variable
func (c *Config) GetPolicyDirectory() string {
	policyDir := os.Getenv("POLICY_DIRECTORY")
	if policyDir == "" {
		policyDir = "policies"
	}
	return policyDir
}
