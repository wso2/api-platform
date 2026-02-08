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
	"net/url"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	toml "github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	commonconstants "github.com/wso2/api-platform/common/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
)

const (
	// EnvPrefix is the prefix for environment variables used to configure the gateway-controller
	EnvPrefix = "APIP_GW_"
	// DefaultLuaScriptPath is the default path for request transformation lua script
	DefaultLuaScriptPath = "./lua/request_transformation.lua"
)

// Config holds all configuration for the gateway-controller
type Config struct {
	GatewayController    GatewayController      `koanf:"gateway_controller"`
	PolicyEngine         map[string]interface{} `koanf:"policy_engine"`
	PolicyConfigurations map[string]interface{} `koanf:"policy_configurations"`
	Analytics            AnalyticsConfig        `koanf:"analytics"`
	TracingConfig        TracingConfig          `koanf:"tracing"`
}

// AnalyticsConfig holds analytics configuration
type AnalyticsConfig struct {
	Enabled              bool                     `koanf:"enabled"`
	Publishers           []map[string]interface{} `koanf:"publishers"`
	GRPCAccessLogCfg     GRPCAccessLogConfig      `koanf:"grpc_access_logs"`
	AccessLogsServiceCfg AccessLogsServiceConfig  `koanf:"access_logs_service"`
	// AllowPayloads controls whether request and response bodies are captured
	// into analytics metadata and forwarded to analytics publishers.
	AllowPayloads bool `koanf:"allow_payloads"`
}

// AccessLogsServiceConfig holds the access logs service configuration
type AccessLogsServiceConfig struct {
	ALSServerPort   int           `koanf:"als_server_port"`
	ShutdownTimeout time.Duration `koanf:"shutdown_timeout"`
	PublicKeyPath   string        `koanf:"public_key_path"`
	PrivateKeyPath  string        `koanf:"private_key_path"`
	ALSPlainText    bool          `koanf:"als_plain_text"`
	MaxMessageSize  int           `koanf:"max_message_size"`
	MaxHeaderLimit  int           `koanf:"max_header_limit"`
}

// GatewayController holds the main configuration sections for the gateway-controller
type GatewayController struct {
	Server       ServerConfig       `koanf:"server"`
	Storage      StorageConfig      `koanf:"storage"`
	Router       RouterConfig       `koanf:"router"`
	Logging      LoggingConfig      `koanf:"logging"`
	ControlPlane ControlPlaneConfig `koanf:"controlplane"`
	PolicyServer PolicyServerConfig `koanf:"policyserver"`
	Policies     PoliciesConfig     `koanf:"policies"`
	LLM          LLMConfig          `koanf:"llm"`
	Auth         AuthConfig         `koanf:"auth"`
	APIKey       APIKeyConfig       `koanf:"api_key"`
	Metrics      MetricsConfig      `koanf:"metrics"`
}

// MetricsConfig holds Prometheus metrics server configuration
type MetricsConfig struct {
	// Enabled indicates whether the metrics server should be started
	Enabled bool `koanf:"enabled"`

	// Port is the port for the metrics HTTP server
	Port int `koanf:"port"`
}

// AuthConfig holds authentication related configuration
type AuthConfig struct {
	Basic BasicAuth `koanf:"basic"`
	IDP   IDPConfig `koanf:"idp"`
}

// BasicAuth describes basic authentication configuration
type BasicAuth struct {
	Enabled bool       `koanf:"enabled"`
	Users   []AuthUser `koanf:"users"`
}

// AuthUser describes a locally configured user
type AuthUser struct {
	Username       string   `koanf:"username"`
	Password       string   `koanf:"password"`        // plain or hashed value depending on PasswordHashed
	PasswordHashed bool     `koanf:"password_hashed"` // true when Password is a bcrypt hash
	Roles          []string `koanf:"roles"`
}

// IDPConfig describes an external identity provider for JWT validation
type IDPConfig struct {
	Enabled     bool                `koanf:"enabled"`
	JWKSURL     string              `koanf:"jwks_url"`
	Issuer      string              `koanf:"issuer"`
	RolesClaim  string              `koanf:"roles_claim"`
	RoleMapping map[string][]string `koanf:"role_mapping"` // local role -> idp roles
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

// PoliciesConfig holds policy-related configuration
type PoliciesConfig struct {
	DefinitionsPath string `koanf:"definitions_path"` // Directory containing policy definitions
}

type LLMConfig struct {
	TemplateDefinitionsPath string `koanf:"template_definitions_path"`
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
	AccessLogs    AccessLogsConfig `koanf:"access_logs"`
	ListenerPort  int              `koanf:"listener_port"`
	HTTPSEnabled  bool             `koanf:"https_enabled"`
	HTTPSPort     int              `koanf:"https_port"`
	GatewayHost   string           `koanf:"gateway_host"`
	Lua           RouterLuaConfig  `koanf:"lua"`
	LuaScriptPath string           `koanf:"lua_script_path"` // Deprecated: use router.lua.request_transformation.script_path
	// Downstream holds downstream-side configuration (TLS and route timeouts)
	Downstream envoyDownstream `koanf:"envoy_downstream"`
	// EnvoyUpstreamCluster holds upstream cluster-level settings (connect timeout)
	EnvoyUpstreamCluster EnvoyUpstreamClusterConfig `koanf:"envoy_upstream"`
	PolicyEngine         PolicyEngineConfig         `koanf:"policy_engine"`
	DownstreamTLS        DownstreamTLS              `koanf:"downstream_tls"`
	EventGateway         EventGatewayConfig         `koanf:"event_gateway"`
	VHosts               VHostsConfig               `koanf:"vhosts"`
	// Tracing holds OpenTelemetry exporter configuration
	TracingServiceName string `koanf:"tracing_service_name"`

	// HTTPListener configuration
	HTTPListener HTTPListenerConfig `koanf:"http_listener"`
}

// EnvoyUpstreamClusterConfig holds default cluster-level settings for API upstream clusters (e.g. connect timeout).
type EnvoyUpstreamClusterConfig struct {
	ConnectTimeoutInMs uint32 `koanf:"connect_timeout_in_ms"`
}

// RouterLuaConfig holds Lua related configurations.
type RouterLuaConfig struct {
	RequestTransformation LuaScriptConfig `koanf:"request_transformation"`
}

// LuaScriptConfig holds Lua script path configuration.
type LuaScriptConfig struct {
	ScriptPath string `koanf:"script_path"`
}

// EventGatewayConfig holds event gateway specific configurations
type EventGatewayConfig struct {
	Enabled               bool   `koanf:"enabled"`
	WebSubHubURL          string `koanf:"websub_hub_url"`
	WebSubHubPort         int    `koanf:"websub_hub_port"`
	RouterHost            string `koanf:"router_host"`
	WebSubHubListenerPort int    `koanf:"websub_hub_listener_port"`
	TimeoutSeconds        int    `koanf:"timeout_seconds"`
}

// DownstreamTLS holds downstream (listener) TLS configuration
type DownstreamTLS struct {
	CertPath               string `koanf:"cert_path"`
	KeyPath                string `koanf:"key_path"`
	MinimumProtocolVersion string `koanf:"minimum_protocol_version"`
	MaximumProtocolVersion string `koanf:"maximum_protocol_version"`
	Ciphers                string `koanf:"ciphers"`
}

// envoyDownstream holds downstream-side configurations (TLS and route timeouts)
type envoyDownstream struct {
	// TLS holds downstream TLS-related configuration for upstream connections
	TLS downstreamTLS `koanf:"tls"`
	// Timeouts holds route timeout configurations (in milliseconds)
	Timeouts routeTimeout `koanf:"timeouts"`
}

// downstreamTLS holds TLS configuration for upstream connections
type downstreamTLS struct {
	MinimumProtocolVersion string `koanf:"minimum_protocol_version"`
	MaximumProtocolVersion string `koanf:"maximum_protocol_version"`
	Ciphers                string `koanf:"ciphers"`
	TrustedCertPath        string `koanf:"trusted_cert_path"`
	CustomCertsPath        string `koanf:"custom_certs_path"` // Directory containing custom trusted certificates
	VerifyHostName         bool   `koanf:"verify_host_name"`
	DisableSslVerification bool   `koanf:"disable_ssl_verification"`
}

// routeTimeout holds route-level timeout configurations (values in milliseconds)
type routeTimeout struct {
	RouteTimeoutInMs     uint32 `koanf:"route_timeout_in_ms"`
	MaxRouteTimeoutInMs  uint32 `koanf:"max_route_timeout_in_ms"`
	RouteIdleTimeoutInMs uint32 `koanf:"route_idle_timeout_in_ms"`
}

// VHostsConfig for vhosts configuration
type VHostsConfig struct {
	Main    VHostEntry `koanf:"main"`
	Sandbox VHostEntry `koanf:"sandbox"`
}

type VHostEntry struct {
	// Optional explicit domain list for the vhost (examples: "*.wso2.com", "api.example.com").
	// If empty, the router will rely on the default pattern.
	Domains []string `koanf:"domains"`
	Default string   `koanf:"default"`
}

// HTTPListenerConfig holds HTTP listener related configuration of an API
type HTTPListenerConfig struct {
	ServerHeaderTransformation string `koanf:"server_header_transformation"` // Options: "APPEND_IF_ABSENT", "OVERWRITE", "PASS_THROUGH"
	ServerHeaderValue          string `koanf:"server_header_value"`          // Custom value for the Server header
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

// GRPCAccessLogConfig holds configuration for gRPC Access Log Service
type GRPCAccessLogConfig struct {
	Host                string `koanf:"host"`
	LogName             string `koanf:"log_name"`
	BufferFlushInterval int    `koanf:"buffer_flush_interval"`
	BufferSizeBytes     int    `koanf:"buffer_size_bytes"`
	GRPCRequestTimeout  int    `koanf:"grpc_request_timeout"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `koanf:"level"`  // "debug", "info", "warn", "error"
	Format string `koanf:"format"` // "json" (default) or "text"
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

// APIKeyConfig represents the configuration for API keys
type APIKeyConfig struct {
	APIKeysPerUserPerAPI int    `koanf:"api_keys_per_user_per_api"` // Number of API keys allowed per user per API
	Algorithm            string `koanf:"algorithm"`                 // Hashing algorithm to use
	MinKeyLength         int    `koanf:"min_key_length"`            // Minimum length for external API key values
	MaxKeyLength         int    `koanf:"max_key_length"`            // Maximum length for external API key values
}

// LoadConfig loads configuration from file, environment variables, and defaults
// Priority: Environment variables > Config file > Defaults
func LoadConfig(configPath string) (*Config, error) {
	cfg := defaultConfig()

	k := koanf.New(".")

	// Load config file if path is provided
	if err := k.Load(file.Provider(configPath), toml.Parser()); err != nil {
		return nil, fmt.Errorf("failed to load config file: %w", err)
	}

	// Load environment variables with prefix
	if err := k.Load(env.Provider(EnvPrefix, ".", func(s string) string {
		s = strings.TrimPrefix(s, EnvPrefix)
		s = strings.ToLower(s)

		// Custom mappings for control plane variables
		switch s {
		case "control_plane_url":
			return "gateway_controller.controlplane.url"
		case "registration_token":
			return "gateway_controller.controlplane.token"
		case "reconnect_initial":
			return "gateway_controller.controlplane.reconnect_initial"
		case "reconnect_max":
			return "gateway_controller.controlplane.reconnect_max"
		case "polling_interval":
			return "gateway_controller.controlplane.polling_interval"
		case "insecure_skip_verify":
			return "gateway_controller.controlplane.insecure_skip_verify"
		default:
			// For other GATEWAY_ prefixed vars, use standard mapping (underscore to dot)
			// Step 1: Convert double underscore "__" into a temporary placeholder
			s = strings.ReplaceAll(s, "__", "%UNDERSCORE%")
			// Step 2: Convert single "_" into "."
			s = strings.ReplaceAll(s, "_", ".")
			// Step 3: Convert placeholder back into literal "_"
			s = strings.ReplaceAll(s, "%UNDERSCORE%", "_")
			return s
		}
	}), nil); err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	// Unmarshal into Config struct with DecodeHook for duration strings
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

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// defaultConfig returns a Config struct with default configuration values
func defaultConfig() *Config {
	return &Config{
		GatewayController: GatewayController{
			Server: ServerConfig{
				APIPort:         9090,
				XDSPort:         18000,
				ShutdownTimeout: 15 * time.Second,
			},
			PolicyServer: PolicyServerConfig{
				Enabled: true,
				Port:    18001,
				TLS: PolicyServerTLS{
					Enabled:  false,
					CertFile: "./certs/server.crt",
					KeyFile:  "./certs/server.key",
				},
			},
			Policies: PoliciesConfig{
				DefinitionsPath: "./default-policies",
			},
			LLM: LLMConfig{
				TemplateDefinitionsPath: "./default-llm-provider-templates",
			},
			Storage: StorageConfig{
				Type: "sqlite",
				SQLite: SQLiteConfig{
					Path: "./data/gateway.db",
				},
			},
			Router: RouterConfig{
				EventGateway: EventGatewayConfig{
					Enabled:               false,
					WebSubHubURL:          "http://host.docker.internal",
					WebSubHubPort:         9098,
					RouterHost:            "localhost",
					WebSubHubListenerPort: 8083,
					TimeoutSeconds:        30,
				},
				AccessLogs: AccessLogsConfig{
					Enabled: true,
					Format:  "json",
					JSONFields: map[string]string{
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
					TextFormat: "[%START_TIME%] \"%REQ(:METHOD)% %REQ(X-ENVOY-ORIGINAL-PATH?:PATH)% %PROTOCOL%\" " +
						"%RESPONSE_CODE% %RESPONSE_FLAGS% %BYTES_RECEIVED% %BYTES_SENT% %DURATION% " +
						"\"%REQ(X-FORWARDED-FOR)%\" \"%REQ(USER-AGENT)%\" \"%REQ(X-REQUEST-ID)%\" " +
						"\"%REQ(:AUTHORITY)%\" \"%UPSTREAM_HOST%\"\n",
				},
				ListenerPort: 8080,
				HTTPSEnabled: true,
				HTTPSPort:    8443,
				Lua: RouterLuaConfig{
					RequestTransformation: LuaScriptConfig{
						ScriptPath: DefaultLuaScriptPath,
					},
				},
				LuaScriptPath: DefaultLuaScriptPath,
				DownstreamTLS: DownstreamTLS{
					CertPath:               "./listener-certs/default-listener.crt",
					KeyPath:                "./listener-certs/default-listener.key",
					MinimumProtocolVersion: "TLS1_2",
					MaximumProtocolVersion: "TLS1_3",
					Ciphers:                "ECDHE-ECDSA-AES128-GCM-SHA256,ECDHE-RSA-AES128-GCM-SHA256,ECDHE-ECDSA-AES128-SHA,ECDHE-RSA-AES128-SHA,AES128-GCM-SHA256,AES128-SHA,ECDHE-ECDSA-AES256-GCM-SHA384,ECDHE-RSA-AES256-GCM-SHA384,ECDHE-ECDSA-AES256-SHA,ECDHE-RSA-AES256-SHA,AES256-GCM-SHA384,AES256-SHA",
				},
				GatewayHost: "*",
				Downstream: envoyDownstream{
					TLS: downstreamTLS{
						MinimumProtocolVersion: "TLS1_2",
						MaximumProtocolVersion: "TLS1_3",
						Ciphers:                "ECDHE-ECDSA-AES128-GCM-SHA256,ECDHE-RSA-AES128-GCM-SHA256,ECDHE-ECDSA-AES128-SHA,ECDHE-RSA-AES128-SHA,AES128-GCM-SHA256,AES128-SHA,ECDHE-ECDSA-AES256-GCM-SHA384,ECDHE-RSA-AES256-GCM-SHA384,ECDHE-ECDSA-AES256-SHA,ECDHE-RSA-AES256-SHA,AES256-GCM-SHA384,AES256-SHA",
						TrustedCertPath:        "/etc/ssl/certs/ca-certificates.crt",
						CustomCertsPath:        "./certificates",
						VerifyHostName:         true,
						DisableSslVerification: false,
					},
					Timeouts: routeTimeout{
						RouteTimeoutInMs:     60000,
						MaxRouteTimeoutInMs:  60000,
						RouteIdleTimeoutInMs: 300000,
					},
				},
				EnvoyUpstreamCluster: EnvoyUpstreamClusterConfig{
					ConnectTimeoutInMs: 5000,
				},
				PolicyEngine: PolicyEngineConfig{
					Enabled:           true,
					Host:              "policy-engine",
					Port:              9001,
					TimeoutMs:         60000,
					FailureModeAllow:  false,
					RouteCacheAction:  "RETAIN",
					AllowModeOverride: true,
					RequestHeaderMode: "SEND",
					MessageTimeoutMs:  60000,
					TLS: PolicyEngineTLS{
						Enabled:    false,
						CertPath:   "",
						KeyPath:    "",
						CAPath:     "",
						ServerName: "",
						SkipVerify: false,
					},
				},
				VHosts: VHostsConfig{
					Main:    VHostEntry{Default: "*"},
					Sandbox: VHostEntry{Default: "sandbox-*"},
				},
				TracingServiceName: "router",
				HTTPListener: HTTPListenerConfig{
					ServerHeaderTransformation: commonconstants.OVERWRITE,
					ServerHeaderValue:          commonconstants.ServerName,
				},
			},
			Auth: AuthConfig{
				Basic: BasicAuth{
					Enabled: true,
					Users:   []AuthUser{},
				},
				IDP: IDPConfig{
					Enabled:     false,
					JWKSURL:     "",
					Issuer:      "",
					RolesClaim:  "",
					RoleMapping: map[string][]string{},
				},
			},
			Logging: LoggingConfig{
				Level:  "info",
				Format: "json",
			},
			Metrics: MetricsConfig{
				Enabled: false,
				Port:    9091,
			},
			ControlPlane: ControlPlaneConfig{
				Host:               "localhost:9243",
				Token:              "",
				ReconnectInitial:   1 * time.Second,
				ReconnectMax:       5 * time.Minute,
				PollingInterval:    15 * time.Minute,
				InsecureSkipVerify: true,
			},
			APIKey: APIKeyConfig{
				APIKeysPerUserPerAPI: 10,
				Algorithm:            constants.HashingAlgorithmSHA256,
				MinKeyLength:         constants.DefaultMinAPIKeyLength,
				MaxKeyLength:         constants.DefaultMaxAPIKeyLength,
			},
		},
		Analytics: AnalyticsConfig{
			Enabled:    false,
			Publishers: make([]map[string]interface{}, 0),
			GRPCAccessLogCfg: GRPCAccessLogConfig{
				Host:                "policy-engine",
				LogName:             "envoy_access_log",
				BufferFlushInterval: 1000000000,
				BufferSizeBytes:     16384,
				GRPCRequestTimeout:  20000000000,
			},
			AccessLogsServiceCfg: AccessLogsServiceConfig{
				ALSServerPort:   18090,
				ShutdownTimeout: 600 * time.Second,
				PublicKeyPath:   "",
				PrivateKeyPath:  "",
				ALSPlainText:    true,
				MaxMessageSize:  1000000000,
				MaxHeaderLimit:  8192,
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
	// Validate storage type
	validStorageTypes := []string{"sqlite", "postgres", "memory"}
	isValidType := false
	for _, t := range validStorageTypes {
		if c.GatewayController.Storage.Type == t {
			isValidType = true
			break
		}
	}
	if !isValidType {
		return fmt.Errorf("storage.type must be one of: sqlite, postgres, memory, got: %s", c.GatewayController.Storage.Type)
	}

	// Validate SQLite configuration
	if c.GatewayController.Storage.Type == "sqlite" && c.GatewayController.Storage.SQLite.Path == "" {
		return fmt.Errorf("storage.sqlite.path is required when storage.type is 'sqlite'")
	}

	// Validate PostgreSQL configuration (future)
	if c.GatewayController.Storage.Type == "postgres" {
		if c.GatewayController.Storage.Postgres.Host == "" {
			return fmt.Errorf("storage.postgres.host is required when storage.type is 'postgres'")
		}
		if c.GatewayController.Storage.Postgres.Database == "" {
			return fmt.Errorf("storage.postgres.database is required when storage.type is 'postgres'")
		}
	}

	// Validate access log format
	if c.GatewayController.Router.AccessLogs.Format != "json" && c.GatewayController.Router.AccessLogs.Format != "text" {
		return fmt.Errorf("router.access_logs.format must be either 'json' or 'text', got: %s", c.GatewayController.Router.AccessLogs.Format)
	}

	// Validate access log fields if access logs are enabled
	if c.GatewayController.Router.AccessLogs.Enabled {
		if c.GatewayController.Router.AccessLogs.Format == "json" {
			if len(c.GatewayController.Router.AccessLogs.JSONFields) == 0 {
				return fmt.Errorf("router.access_logs.json_fields must be configured when format is 'json'")
			}
		} else if c.GatewayController.Router.AccessLogs.Format == "text" {
			if c.GatewayController.Router.AccessLogs.TextFormat == "" {
				return fmt.Errorf("router.access_logs.text_format must be configured when format is 'text'")
			}
		}
	}

	// Validate log level
	validLevels := []string{"debug", "info", "warn", "warning", "error"}
	isValidLevel := false
	for _, level := range validLevels {
		if strings.ToLower(c.GatewayController.Logging.Level) == level {
			isValidLevel = true
			break
		}
	}
	if !isValidLevel {
		return fmt.Errorf("logging.level must be one of: debug, info, warn, error, got: %s", c.GatewayController.Logging.Level)
	}

	// Validate log format
	if c.GatewayController.Logging.Format != "json" && c.GatewayController.Logging.Format != "text" {
		return fmt.Errorf("logging.format must be either 'json' or 'text', got: %s", c.GatewayController.Logging.Format)
	}

	// Validate ports
	if c.GatewayController.Server.APIPort < 1 || c.GatewayController.Server.APIPort > 65535 {
		return fmt.Errorf("server.api_port must be between 1 and 65535, got: %d", c.GatewayController.Server.APIPort)
	}

	if c.GatewayController.Server.XDSPort < 1 || c.GatewayController.Server.XDSPort > 65535 {
		return fmt.Errorf("server.xds_port must be between 1 and 65535, got: %d", c.GatewayController.Server.XDSPort)
	}

	// Validate metrics config
	if c.GatewayController.Metrics.Enabled {
		if c.GatewayController.Metrics.Port < 1 || c.GatewayController.Metrics.Port > 65535 {
			return fmt.Errorf("metrics.port must be between 1 and 65535, got: %d", c.GatewayController.Metrics.Port)
		}
		if c.GatewayController.Metrics.Port == c.GatewayController.Server.APIPort {
			return fmt.Errorf("metrics.port cannot be same as server.api_port")
		}
		if c.GatewayController.Metrics.Port == c.GatewayController.Server.XDSPort {
			return fmt.Errorf("metrics.port cannot be same as server.xds_port")
		}
	}

	if c.GatewayController.Router.ListenerPort < 1 || c.GatewayController.Router.ListenerPort > 65535 {
		return fmt.Errorf("router.listener_port must be between 1 and 65535, got: %d", c.GatewayController.Router.ListenerPort)
	}

	// Validate HTTPS port if HTTPS is enabled
	if c.GatewayController.Router.HTTPSEnabled {
		if c.GatewayController.Router.HTTPSPort < 1 || c.GatewayController.Router.HTTPSPort > 65535 {
			return fmt.Errorf("router.https_port must be between 1 and 65535, got: %d", c.GatewayController.Router.HTTPSPort)
		}
	}

	// Validate event gateway configuration if enabled
	if c.GatewayController.Router.EventGateway.Enabled {
		if err := c.validateEventGatewayConfig(); err != nil {
			return err
		}
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

	// Validate envoy upstream cluster configuration
	if err := c.validateEnvoyUpstreamClusterConfig(); err != nil {
		return err
	}

	// Validate policy engine configuration
	if err := c.validatePolicyEngineConfig(); err != nil {
		return err
	}

	// Validate vhost configuration
	if err := c.validateVHostsConfig(); err != nil {
		return err
	}

	if err := c.validateAnalyticsConfig(); err != nil {
		return err
	}

	// Validate authentication configuration
	if err := c.validateAuthConfig(); err != nil {
		return err
	}

	if err := c.validateHTTPListenerConfig(); err != nil {
		return err
	}

	// Validate API key configuration
	if err := c.validateAPIKeyConfig(); err != nil {
		return err
	}

	return nil
}

func (c *Config) validateEventGatewayConfig() error {
	if c.GatewayController.Router.EventGateway.WebSubHubPort < 1 || c.GatewayController.Router.EventGateway.WebSubHubPort > 65535 {
		return fmt.Errorf("router.event_gateway.websub_hub_port must be between 1 and 65535, got: %d", c.GatewayController.Router.EventGateway.WebSubHubPort)
	}
	if c.GatewayController.Router.EventGateway.WebSubHubListenerPort < 1 || c.GatewayController.Router.EventGateway.WebSubHubListenerPort > 65535 {
		return fmt.Errorf("router.event_gateway.websub_hub_listener_port must be between 1 and 65535, got: %d", c.GatewayController.Router.EventGateway.WebSubHubListenerPort)
	}

	// Validate WebSubHubURL if provided - must be a valid http(s) URL
	if strings.TrimSpace(c.GatewayController.Router.EventGateway.WebSubHubURL) != "" {
		u, err := url.Parse(c.GatewayController.Router.EventGateway.WebSubHubURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("router.event_gateway.websub_hub_url must be a valid URL with http or https scheme, got: %s", c.GatewayController.Router.EventGateway.WebSubHubURL)
		}
		if u.Host == "" {
			return fmt.Errorf("router.event_gateway.websub_hub_url must include a valid host, got: %s", c.GatewayController.Router.EventGateway.WebSubHubURL)
		}
	}
	if c.GatewayController.Router.EventGateway.TimeoutSeconds <= 0 {
		return fmt.Errorf("router.event_gateway.timeout_seconds must be positive, got: %d", c.GatewayController.Router.EventGateway.TimeoutSeconds)
	}
	return nil
}

// validateControlPlaneConfig validates the control plane configuration
func (c *Config) validateControlPlaneConfig() error {
	// Host validation - required if control plane is configured
	if c.GatewayController.ControlPlane.Host == "" {
		return fmt.Errorf("controlplane.host is required")
	}

	// Token is optional - gateway can run without control plane connection
	// If token is empty, connection will not be established

	// Validate reconnection intervals
	if c.GatewayController.ControlPlane.ReconnectInitial <= 0 {
		return fmt.Errorf("controlplane.reconnect_initial must be positive, got: %s", c.GatewayController.ControlPlane.ReconnectInitial)
	}

	if c.GatewayController.ControlPlane.ReconnectMax <= 0 {
		return fmt.Errorf("controlplane.reconnect_max must be positive, got: %s", c.GatewayController.ControlPlane.ReconnectMax)
	}

	if c.GatewayController.ControlPlane.ReconnectInitial > c.GatewayController.ControlPlane.ReconnectMax {
		return fmt.Errorf("controlplane.reconnect_initial (%s) must be <= controlplane.reconnect_max (%s)",
			c.GatewayController.ControlPlane.ReconnectInitial, c.GatewayController.ControlPlane.ReconnectMax)
	}

	// Validate polling interval
	if c.GatewayController.ControlPlane.PollingInterval <= 0 {
		return fmt.Errorf("controlplane.polling_interval must be positive, got: %s", c.GatewayController.ControlPlane.PollingInterval)
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
	minVersion := c.GatewayController.Router.Downstream.TLS.MinimumProtocolVersion
	if minVersion == "" {
		return fmt.Errorf("router.envoy_downstream.tls.minimum_protocol_version is required")
	}

	isValidMinVersion := false
	for _, version := range validTLSVersions {
		if minVersion == version {
			isValidMinVersion = true
			break
		}
	}
	if !isValidMinVersion {
		return fmt.Errorf("router.envoy_downstream.tls.minimum_protocol_version must be one of: %s, got: %s",
			strings.Join(validTLSVersions, ", "), minVersion)
	}

	// Validate maximum TLS version
	maxVersion := c.GatewayController.Router.Downstream.TLS.MaximumProtocolVersion
	if maxVersion == "" {
		return fmt.Errorf("router.envoy_downstream.tls.maximum_protocol_version is required")
	}

	isValidMaxVersion := false
	for _, version := range validTLSVersions {
		if maxVersion == version {
			isValidMaxVersion = true
			break
		}
	}
	if !isValidMaxVersion {
		return fmt.Errorf("router.envoy_downstream.tls.maximum_protocol_version must be one of: %s, got: %s",
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
		return fmt.Errorf("router.envoy_downstream.tls.minimum_protocol_version (%s) cannot be greater than maximum_protocol_version (%s)",
			minVersion, maxVersion)
	}

	// Validate cipher suites format (basic validation - ensure it's not empty if provided)
	ciphers := c.GatewayController.Router.Downstream.TLS.Ciphers
	if ciphers != "" {
		// Basic validation: ensure ciphers don't contain invalid characters
		if strings.Contains(ciphers, constants.CipherInvalidChars1) || strings.Contains(ciphers, constants.CipherInvalidChars2) {
			return fmt.Errorf("router.envoy_downstream.tls.ciphers contains invalid characters (use comma-separated values)")
		}

		// Ensure cipher list is not just whitespace
		if strings.TrimSpace(ciphers) == "" {
			return fmt.Errorf("router.envoy_downstream.tls.ciphers cannot be empty or whitespace only")
		}
	}

	// Validate trusted cert path if SSL verification is enabled
	if !c.GatewayController.Router.Downstream.TLS.DisableSslVerification && c.GatewayController.Router.Downstream.TLS.TrustedCertPath == "" {
		return fmt.Errorf("router.envoy_downstream.tls.trusted_cert_path is required when SSL verification is enabled")
	}

	// Validate downstream TLS configuration if HTTPS is enabled
	if c.GatewayController.Router.HTTPSEnabled {
		if err := c.validateDownstreamTLSConfig(); err != nil {
			return err
		}
	}

	return nil
}

// validateDownstreamTLSConfig validates the downstream (listener) TLS configuration
func (c *Config) validateDownstreamTLSConfig() error {
	// Validate TLS protocol versions
	validTLSVersions := []string{
		constants.TLSVersion10,
		constants.TLSVersion11,
		constants.TLSVersion12,
		constants.TLSVersion13,
	}

	// Validate certificate and key paths
	if c.GatewayController.Router.DownstreamTLS.CertPath == "" {
		return fmt.Errorf("router.downstream_tls.cert_path is required when HTTPS is enabled")
	}

	if c.GatewayController.Router.DownstreamTLS.KeyPath == "" {
		return fmt.Errorf("router.downstream_tls.key_path is required when HTTPS is enabled")
	}

	// Validate minimum TLS version
	minVersion := c.GatewayController.Router.DownstreamTLS.MinimumProtocolVersion
	if minVersion == "" {
		return fmt.Errorf("router.downstream_tls.minimum_protocol_version is required")
	}

	isValidMinVersion := false
	for _, version := range validTLSVersions {
		if minVersion == version {
			isValidMinVersion = true
			break
		}
	}
	if !isValidMinVersion {
		return fmt.Errorf("router.downstream_tls.minimum_protocol_version must be one of: %s, got: %s",
			strings.Join(validTLSVersions, ", "), minVersion)
	}

	// Validate maximum TLS version
	maxVersion := c.GatewayController.Router.DownstreamTLS.MaximumProtocolVersion
	if maxVersion == "" {
		return fmt.Errorf("router.downstream_tls.maximum_protocol_version is required")
	}

	isValidMaxVersion := false
	for _, version := range validTLSVersions {
		if maxVersion == version {
			isValidMaxVersion = true
			break
		}
	}
	if !isValidMaxVersion {
		return fmt.Errorf("router.downstream_tls.maximum_protocol_version must be one of: %s, got: %s",
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
		return fmt.Errorf("router.downstream_tls.minimum_protocol_version (%s) cannot be greater than maximum_protocol_version (%s)",
			minVersion, maxVersion)
	}

	// Validate cipher suites format
	ciphers := c.GatewayController.Router.DownstreamTLS.Ciphers
	if ciphers != "" {
		// Basic validation: ensure ciphers don't contain invalid characters
		if strings.Contains(ciphers, constants.CipherInvalidChars1) || strings.Contains(ciphers, constants.CipherInvalidChars2) {
			return fmt.Errorf("router.downstream_tls.ciphers contains invalid characters (use comma-separated values)")
		}

		// Ensure cipher list is not just whitespace
		if strings.TrimSpace(ciphers) == "" {
			return fmt.Errorf("router.downstream_tls.ciphers cannot be empty or whitespace only")
		}
	}

	return nil
}

// validateTimeoutConfig validates the timeout configuration
func (c *Config) validateTimeoutConfig() error {
	timeouts := c.GatewayController.Router.Downstream.Timeouts

	// Validate route timeout
	if timeouts.RouteTimeoutInMs <= 0 {
		return fmt.Errorf("router.envoy_downstream.timeouts.route_timeout_in_ms must be positive, got: %d",
			timeouts.RouteTimeoutInMs)
	}

	// Validate max route timeout
	if timeouts.MaxRouteTimeoutInMs <= 0 {
		return fmt.Errorf("router.envoy_downstream.timeouts.max_route_timeout_in_ms must be positive, got: %d",
			timeouts.MaxRouteTimeoutInMs)
	}

	// Validate idle timeout
	if timeouts.RouteIdleTimeoutInMs <= 0 {
		return fmt.Errorf("router.envoy_downstream.timeouts.route_idle_timeout_in_ms must be positive, got: %d",
			timeouts.RouteIdleTimeoutInMs)
	}

	// Validate that route timeout is not greater than max route timeout
	if timeouts.RouteTimeoutInMs > timeouts.MaxRouteTimeoutInMs {
		return fmt.Errorf("router.envoy_downstream.timeouts.route_timeout_in_ms (%d) cannot be greater than max_route_timeout_in_ms (%d)",
			timeouts.RouteTimeoutInMs, timeouts.MaxRouteTimeoutInMs)
	}

	// Validate reasonable timeout ranges (prevent extremely long timeouts)
	if timeouts.RouteTimeoutInMs > constants.MaxReasonableTimeoutMs {
		return fmt.Errorf("router.envoy_downstream.timeouts.route_timeout_in_ms (%d) exceeds maximum reasonable timeout of %d ms",
			timeouts.RouteTimeoutInMs, constants.MaxReasonableTimeoutMs)
	}

	if timeouts.MaxRouteTimeoutInMs > constants.MaxReasonableTimeoutMs {
		return fmt.Errorf("router.envoy_downstream.timeouts.max_route_timeout_in_ms (%d) exceeds maximum reasonable timeout of %d ms",
			timeouts.MaxRouteTimeoutInMs, constants.MaxReasonableTimeoutMs)
	}

	if timeouts.RouteIdleTimeoutInMs > constants.MaxReasonableTimeoutMs {
		return fmt.Errorf("router.envoy_downstream.timeouts.route_idle_timeout_in_ms (%d) exceeds maximum reasonable timeout of %d ms",
			timeouts.RouteIdleTimeoutInMs, constants.MaxReasonableTimeoutMs)
	}

	return nil
}

// validateEnvoyUpstreamClusterConfig validates the envoy upstream cluster configuration
func (c *Config) validateEnvoyUpstreamClusterConfig() error {
	clusterCfg := c.GatewayController.Router.EnvoyUpstreamCluster
	if clusterCfg.ConnectTimeoutInMs <= 0 {
		return fmt.Errorf("router.envoy_upstream.connect_timeout_in_ms must be positive, got: %d",
			clusterCfg.ConnectTimeoutInMs)
	}
	if clusterCfg.ConnectTimeoutInMs > constants.MaxReasonableTimeoutMs {
		return fmt.Errorf("router.envoy_upstream.connect_timeout_in_ms (%d) exceeds maximum reasonable timeout of %d ms",
			clusterCfg.ConnectTimeoutInMs, constants.MaxReasonableTimeoutMs)
	}
	return nil
}

// validatePolicyEngineConfig validates the policy engine configuration
func (c *Config) validatePolicyEngineConfig() error {
	policyEngine := c.GatewayController.Router.PolicyEngine

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

// validateVHostsConfig validates the vhosts configuration
func (c *Config) validateVHostsConfig() error {
	if strings.TrimSpace(c.GatewayController.Router.VHosts.Main.Default) == "" {
		return fmt.Errorf("router.vhosts.main.default must be a non-empty string")
	}
	if strings.TrimSpace(c.GatewayController.Router.VHosts.Sandbox.Default) == "" {
		return fmt.Errorf("router.vhosts.sandbox.default must be a non-empty string")
	}

	// Validate main.domains (only if not nil)
	if err := validateDomains("router.vhosts.main.domains", c.GatewayController.Router.VHosts.Main.Domains); err != nil {
		return err
	}

	// Validate sandbox.domains (only if not nil)
	if err := validateDomains("router.vhosts.sandbox.domains", c.GatewayController.Router.VHosts.Sandbox.Domains); err != nil {
		return err
	}

	return nil
}

func validateDomains(field string, domains []string) error {
	if domains == nil {
		// nil means "not configured" → valid
		return nil
	}

	for i, d := range domains {
		if strings.TrimSpace(d) == "" {
			return fmt.Errorf("%s[%d] must be a non-empty string", field, i)
		}
	}
	return nil
}

// validateAnalyticsConfig validates the analytics configuration
func (c *Config) validateAnalyticsConfig() error {
	// Validate analytics configuration
	if c.Analytics.Enabled {
		// Validate gRPC access log configuration
		grpcAccessLogCfg := c.Analytics.GRPCAccessLogCfg
		alsServerPort := c.Analytics.AccessLogsServiceCfg.ALSServerPort
		if alsServerPort <= 0 || alsServerPort > 65535 {
			return fmt.Errorf("analytics.access_logs_service.als_server_port must be an integer between 1 and 65535, got %d", alsServerPort)
		}
		if grpcAccessLogCfg.Host == "" {
			return fmt.Errorf("analytics.grpc_access_logs.host is required when analytics.enabled is true")
		}
		if grpcAccessLogCfg.LogName == "" {
			return fmt.Errorf("analytics.grpc_access_logs.log_name is required when analytics.enabled is true")
		}
		if grpcAccessLogCfg.BufferFlushInterval <= 0 || grpcAccessLogCfg.BufferSizeBytes <= 0 || grpcAccessLogCfg.GRPCRequestTimeout <= 0 {
			return fmt.Errorf(
				"invalid gRPC access log configuration: bufferFlushInterval=%d, bufferSizeBytes=%d, grpcRequestTimeout=%d (all must be > 0)",
				grpcAccessLogCfg.BufferFlushInterval,
				grpcAccessLogCfg.BufferSizeBytes,
				grpcAccessLogCfg.GRPCRequestTimeout,
			)
		}

	}
	return nil
}

// validateAuthConfig validates the authentication configuration
func (c *Config) validateAuthConfig() error {
	// Validate IDP role mapping for multiple wildcards
	if c.GatewayController.Auth.IDP.Enabled && len(c.GatewayController.Auth.IDP.RoleMapping) > 0 {
		wildcardRoles := []string{}
		for localRole, idpRoles := range c.GatewayController.Auth.IDP.RoleMapping {
			for _, idpRole := range idpRoles {
				if idpRole == "*" {
					wildcardRoles = append(wildcardRoles, localRole)
					break
				}
			}
		}

		if len(wildcardRoles) > 1 {
			return fmt.Errorf(
				"auth.idp.role_mapping: multiple wildcard ('*') mappings detected for roles %v. "+
					"Due to Go's non-deterministic map iteration, only one wildcard mapping will be used unpredictably. "+
					"Configure only ONE role with wildcard mapping, or use explicit role mappings instead",
				wildcardRoles,
			)
		}
	}

	return nil
}

// validateAPIKeyConfig validates the API key configuration
func (c *Config) validateAPIKeyConfig() error {
	// If number of api keys per user is not provided or negative throw error
	if c.GatewayController.APIKey.APIKeysPerUserPerAPI <= 0 {
		return fmt.Errorf("api_key.api_keys_per_user_per_api must be a positive integer, got: %d",
			c.GatewayController.APIKey.APIKeysPerUserPerAPI)
	}

	// Default min/max key lengths if not configured
	if c.GatewayController.APIKey.MinKeyLength <= 0 {
		c.GatewayController.APIKey.MinKeyLength = constants.DefaultMinAPIKeyLength
	}
	if c.GatewayController.APIKey.MaxKeyLength <= 0 {
		c.GatewayController.APIKey.MaxKeyLength = constants.DefaultMaxAPIKeyLength
	}
	if c.GatewayController.APIKey.MinKeyLength > c.GatewayController.APIKey.MaxKeyLength {
		return fmt.Errorf("api_key.min_key_length (%d) must not exceed api_key.max_key_length (%d)",
			c.GatewayController.APIKey.MinKeyLength, c.GatewayController.APIKey.MaxKeyLength)
	}

	// If hashing is enabled but no algorithm is provided, default to SHA256
	if c.GatewayController.APIKey.Algorithm == "" {
		c.GatewayController.APIKey.Algorithm = constants.HashingAlgorithmSHA256
		return nil
	}

	// If hashing is enabled and algorithm is provided, validate it's one of the supported ones
	validAlgorithms := []string{
		constants.HashingAlgorithmSHA256,
		constants.HashingAlgorithmBcrypt,
		constants.HashingAlgorithmArgon2ID,
	}
	isValidAlgorithm := false
	for _, alg := range validAlgorithms {
		if strings.ToLower(c.GatewayController.APIKey.Algorithm) == alg {
			isValidAlgorithm = true
			break
		}
	}
	if !isValidAlgorithm {
		return fmt.Errorf("api_key.algorithm must be one of: %s, got: %s",
			strings.Join(validAlgorithms, ", "), c.GatewayController.APIKey.Algorithm)
	}
	return nil
}

// IsPersistentMode returns true if storage type is not memory
func (c *Config) IsPersistentMode() bool {
	return c.GatewayController.Storage.Type != "memory"
}

// IsMemoryOnlyMode returns true if storage type is memory
func (c *Config) IsMemoryOnlyMode() bool {
	return c.GatewayController.Storage.Type == "memory"
}

// IsAccessLogsEnabled returns true if access logs are enabled
func (c *Config) IsAccessLogsEnabled() bool {
	return c.GatewayController.Router.AccessLogs.Enabled
}

// IsPolicyEngineEnabled returns true if policy engine is enabled
func (c *Config) IsPolicyEngineEnabled() bool {
	return c.GatewayController.Router.PolicyEngine.Enabled
}

// validateHTTPListenerConfig validates the HTTP listener configuration
func (c *Config) validateHTTPListenerConfig() error {
	httpListener := &c.GatewayController.Router.HTTPListener

	// Set default values if not provided
	if httpListener.ServerHeaderTransformation == "" {
		httpListener.ServerHeaderTransformation = commonconstants.OVERWRITE
	}

	// Validate ServerHeaderTransformation value
	validTransformations := []string{
		commonconstants.APPEND_IF_ABSENT,
		commonconstants.OVERWRITE,
		commonconstants.PASS_THROUGH,
	}

	isValid := false
	for _, valid := range validTransformations {
		if httpListener.ServerHeaderTransformation == valid {
			isValid = true
			break
		}
	}

	if !isValid {
		return fmt.Errorf("http_listener.server_header_transformation must be one of: %s, %s, %s. Got: %s",
			commonconstants.APPEND_IF_ABSENT,
			commonconstants.OVERWRITE,
			commonconstants.PASS_THROUGH,
			httpListener.ServerHeaderTransformation)
	}

	return nil
}
