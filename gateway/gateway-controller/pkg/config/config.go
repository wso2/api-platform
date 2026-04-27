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
	Controller           Controller             `koanf:"controller"`
	Router               RouterConfig           `koanf:"router"`
	PolicyEngine         map[string]interface{} `koanf:"policy_engine"`
	PolicyConfigurations map[string]interface{} `koanf:"policy_configurations"`
	Analytics            AnalyticsConfig        `koanf:"analytics"`
	TracingConfig        TracingConfig          `koanf:"tracing"`
	APIKey               APIKeyConfig           `koanf:"api_key"`
	// Subscriptions controls application-level subscription behaviour for APIs.
	// When nil, subscription validation system policy remains disabled.
	Subscriptions    *SubscriptionsConfig   `koanf:"subscriptions"`
	ImmutableGateway ImmutableGatewayConfig `koanf:"immutable_gateway"`
}

// AnalyticsConfig holds analytics configuration
type AnalyticsConfig struct {
	Enabled            bool                      `koanf:"enabled"`
	EnabledPublishers  []string                  `koanf:"enabled_publishers"`
	Publishers         AnalyticsPublishersConfig `koanf:"publishers"`
	GRPCEventServerCfg GRPCEventServerConfig     `koanf:"grpc_event_server"`
	// AllowPayloads controls whether request and response bodies are captured
	// into analytics metadata and forwarded to analytics publishers.
	AllowPayloads bool `koanf:"allow_payloads"`
}

// SubscriptionsConfig holds configuration for application-level subscriptions.
type SubscriptionsConfig struct {
	// EnableValidation toggles automatic injection of the subscriptionValidation
	// system policy into API policy chains.
	EnableValidation bool `koanf:"enable_validation"`
}

// ImmutableGatewayConfig holds configuration for immutable gateway mode.
// When enabled, the gateway loads all API artifacts from the filesystem on startup
// and rejects all mutating management API operations (POST, PUT, DELETE) at runtime.
type ImmutableGatewayConfig struct {
	Enabled      bool   `koanf:"enabled"`
	ArtifactsDir string `koanf:"artifacts_dir"`
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

// GRPCEventServerConfig holds configuration for gRPC event server (combines access log service and ALS server config)
type GRPCEventServerConfig struct {
	Mode                string        `koanf:"mode"`                  // Connection mode: "uds" (default) or "tcp"
	Port                int           `koanf:"port"`                  // ALS port for Envoy connection (TCP mode only)
	ServerPort          int           `koanf:"server_port"`           // gRPC server port for ALS server
	BufferFlushInterval int           `koanf:"buffer_flush_interval"` // Envoy buffer flush interval (nanoseconds)
	BufferSizeBytes     int           `koanf:"buffer_size_bytes"`     // Envoy buffer size
	GRPCRequestTimeout  int           `koanf:"grpc_request_timeout"`  // Envoy gRPC timeout (nanoseconds)
	ShutdownTimeout     time.Duration `koanf:"shutdown_timeout"`      // ALS server shutdown timeout
	PublicKeyPath       string        `koanf:"public_key_path"`       // TLS public key path
	PrivateKeyPath      string        `koanf:"private_key_path"`      // TLS private key path
	ALSPlainText        bool          `koanf:"als_plain_text"`        // Use plaintext gRPC
	MaxMessageSize      int           `koanf:"max_message_size"`      // Max gRPC message size
	MaxHeaderLimit      int           `koanf:"max_header_limit"`      // Max header size
}

// Controller holds the main configuration sections for the gateway-controller
type Controller struct {
	Server       ServerConfig       `koanf:"server"`
	AdminServer  AdminServerConfig  `koanf:"admin_server"`
	Storage      StorageConfig      `koanf:"storage"`
	Logging      LoggingConfig      `koanf:"logging"`
	ControlPlane ControlPlaneConfig `koanf:"controlplane"`
	PolicyServer PolicyServerConfig `koanf:"policy_server"`
	Policies     PoliciesConfig     `koanf:"policies"`
	LLM          LLMConfig          `koanf:"llm"`
	Auth         AuthConfig         `koanf:"auth"`
	Metrics      MetricsConfig      `koanf:"metrics"`
	Encryption   EncryptionConfig   `koanf:"encryption"`
	EventHub     EventHubConfig     `koanf:"event_hub"`
}

// MetricsConfig holds Prometheus metrics server configuration
type MetricsConfig struct {
	// Enabled indicates whether the metrics server should be started
	Enabled bool `koanf:"enabled"`

	// Port is the port for the metrics HTTP server
	Port int `koanf:"port"`
}

// EventHubConfig holds EventHub configuration for multi-replica sync
type EventHubConfig struct {
	PollInterval    time.Duration          `koanf:"poll_interval"`
	CleanupInterval time.Duration          `koanf:"cleanup_interval"`
	RetentionPeriod time.Duration          `koanf:"retention_period"`
	Database        EventHubDatabaseConfig `koanf:"database"`
}

// EventHubDatabaseConfig holds connection pool settings for the EventHub database connection
type EventHubDatabaseConfig struct {
	MaxOpenConns    int           `koanf:"max_open_conns"`
	MaxIdleConns    int           `koanf:"max_idle_conns"`
	ConnMaxLifetime time.Duration `koanf:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `koanf:"conn_max_idle_time"`
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
	APIPort                         int           `koanf:"api_port"`
	XDSPort                         int           `koanf:"xds_port"`
	ShutdownTimeout                 time.Duration `koanf:"shutdown_timeout"`
	GatewayID                       string        `koanf:"gateway_id"`
	SkipInvalidDeploymentsOnStartup bool          `koanf:"skip_invalid_deployments_on_startup"`
}

// AdminServerConfig holds controller admin HTTP server configuration.
type AdminServerConfig struct {
	Enabled    bool     `koanf:"enabled"`
	Port       int      `koanf:"port"`
	AllowedIPs []string `koanf:"allowed_ips"`
}

// PolicyServerConfig holds policy xDS server-related configuration
type PolicyServerConfig struct {
	Port int             `koanf:"port"`
	TLS  PolicyServerTLS `koanf:"tls"`
}

// PolicyServerTLS holds TLS configuration for the policy xDS server
type PolicyServerTLS struct {
	Enabled  bool   `koanf:"enabled"`
	CertFile string `koanf:"cert_file"`
	KeyFile  string `koanf:"key_file"`
}

// PoliciesConfig holds policy-related configuration
type PoliciesConfig struct {
	DefinitionsPath   string `koanf:"definitions_path"`    // Directory containing policy definitions
	BuildManifestPath string `koanf:"build_manifest_path"` // Path to build-manifest.yaml for custom policy detection
}

type LLMConfig struct {
	TemplateDefinitionsPath string `koanf:"template_definitions_path"`
}

// StorageConfig holds storage-related configuration
type StorageConfig struct {
	Type     string         `koanf:"type"`     // "sqlite", "postgres", or "memory"
	SQLite   SQLiteConfig   `koanf:"sqlite"`   // SQLite-specific configuration
	Postgres PostgresConfig `koanf:"postgres"` // PostgreSQL-specific configuration
}

// SQLiteConfig holds SQLite-specific configuration
type SQLiteConfig struct {
	Path string `koanf:"path"` // Path to SQLite database file
}

// PostgresConfig holds PostgreSQL-specific configuration.
type PostgresConfig struct {
	DSN             string        `koanf:"dsn"`
	Host            string        `koanf:"host"`
	Port            int           `koanf:"port"`
	Database        string        `koanf:"database"`
	User            string        `koanf:"user"`
	Password        string        `koanf:"password"`
	SSLMode         string        `koanf:"sslmode"`
	ConnectTimeout  time.Duration `koanf:"connect_timeout"`
	MaxOpenConns    int           `koanf:"max_open_conns"`
	MaxIdleConns    int           `koanf:"max_idle_conns"`
	ConnMaxLifetime time.Duration `koanf:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `koanf:"conn_max_idle_time"`
	ApplicationName string        `koanf:"application_name"`
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
	// Upstream holds upstream-side configuration (TLS and timeouts: route, idle, connect)
	Upstream           RouterUpstream     `koanf:"upstream"`
	PolicyEngine       PolicyEngineConfig `koanf:"policy_engine"`
	DownstreamTLS      DownstreamTLS      `koanf:"downstream_tls"`
	EventGateway       EventGatewayConfig `koanf:"event_gateway"`
	VHosts             VHostsConfig       `koanf:"vhosts"`
	TracingServiceName string             `koanf:"tracing_service_name"`

	// HTTPListener configuration
	HTTPListener HTTPListenerConfig `koanf:"http_listener"`
}

// RouterUpstream holds upstream-side configuration (TLS and timeouts for Envoy upstream).
type RouterUpstream struct {
	TLS      UpstreamTLS      `koanf:"tls"`
	Timeouts UpstreamTimeouts `koanf:"timeouts"`
}

// UpstreamTLS holds TLS configuration for upstream connections.
type UpstreamTLS struct {
	MinimumProtocolVersion string `koanf:"minimum_protocol_version"`
	MaximumProtocolVersion string `koanf:"maximum_protocol_version"`
	Ciphers                string `koanf:"ciphers"`
	TrustedCertPath        string `koanf:"trusted_cert_path"`
	CustomCertsPath        string `koanf:"custom_certs_path"` // Directory containing custom trusted certificates
	VerifyHostName         bool   `koanf:"verify_host_name"`
	DisableSslVerification bool   `koanf:"disable_ssl_verification"`
}

// UpstreamTimeouts holds upstream timeout configurations (values in milliseconds).
type UpstreamTimeouts struct {
	RouteTimeoutMs     uint32 `koanf:"route_timeout_ms"`
	RouteIdleTimeoutMs uint32 `koanf:"route_idle_timeout_ms"`
	ConnectTimeoutMs   uint32 `koanf:"connect_timeout_ms"`
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
	Mode              string          `koanf:"mode"` // Connection mode: "uds" (default) or "tcp"
	Host              string          `koanf:"host"` // Policy engine hostname/IP (TCP mode only)
	Port              uint32          `koanf:"port"` // Policy engine ext_proc port (TCP mode only)
	TimeoutMs         uint32          `koanf:"timeout_ms"`
	FailureModeAllow  bool            `koanf:"failure_mode_allow"`
	AllowModeOverride bool            `koanf:"allow_mode_override"`
	MessageTimeoutMs  uint32          `koanf:"message_timeout_ms"`
	TLS               PolicyEngineTLS `koanf:"tls"` // TLS configuration (TCP mode only)
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
	Format string `koanf:"format"` // "json" (default) or "text"
}

// ControlPlaneConfig holds control plane connection configuration
type ControlPlaneConfig struct {
	Host                  string        `koanf:"host"`                    // Control plane hostname
	Token                 string        `koanf:"token"`                   // Registration token (api-key)
	ReconnectInitial      time.Duration `koanf:"reconnect_initial"`       // Initial retry delay
	ReconnectMax          time.Duration `koanf:"reconnect_max"`           // Maximum retry delay
	PollingInterval       time.Duration `koanf:"polling_interval"`        // Reconciliation polling interval
	InsecureSkipVerify    bool          `koanf:"insecure_skip_verify"`    // Skip TLS certificate verification (insecure, dev/test only)
	DeploymentPushEnabled bool          `koanf:"deployment_push_enabled"` // Push API deployments to control plane (default: false)
	SyncBatchSize         int           `koanf:"sync_batch_size"`         // Number of deployments to fetch per batch request during startup sync (default: 50)
	// OAuth2 credentials for on-prem APIM API import (for bottom-up API deployment)
	ApimOAuth2ClientID     string `koanf:"apim_oauth2_client_id"`     // APIM OAuth2 client ID
	ApimOAuth2ClientSecret string `koanf:"apim_oauth2_client_secret"` // APIM OAuth2 client secret
	ApimOAuth2Username     string `koanf:"apim_oauth2_username"`      // APIM resource owner username
	ApimOAuth2Password     string `koanf:"apim_oauth2_password"`      // APIM resource owner password
	GatewayName            string `koanf:"gateway_name"`              // Name of the gateway for deployment configuration
}

// APIKeyConfig represents the configuration for API keys
type APIKeyConfig struct {
	APIKeysPerUserPerAPI int    `koanf:"api_keys_per_user_per_api"` // Number of API keys allowed per user per API
	Algorithm            string `koanf:"algorithm"`                 // Hashing algorithm to use
	MinKeyLength         int    `koanf:"min_key_length"`            // Minimum length for external API key values
	MaxKeyLength         int    `koanf:"max_key_length"`            // Maximum length for external API key values
	// Issuer identifies this gateway's portal; when non-empty, only API keys whose
	// issuer field matches (or is null) will be accepted by the api-key-auth policy.
	Issuer string `koanf:"issuer"`
}

// EncryptionConfig holds encryption provider configuration
type EncryptionConfig struct {
	Providers []ProviderConfig `koanf:"providers"`
}

// ProviderConfig defines configuration for a single encryption provider
type ProviderConfig struct {
	Type string                `koanf:"type"` // "aesgcm"
	Keys []EncryptionKeyConfig `koanf:"keys"`
}

// EncryptionKeyConfig defines a single encryption key
type EncryptionKeyConfig struct {
	Version  string `koanf:"version"` // Key identifier (e.g., "key-v1")
	FilePath string `koanf:"file"`    // Path to raw binary key file
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
		case "controlplane_host":
			return "controller.controlplane.host"
		case "gateway_registration_token":
			return "controller.controlplane.token"
		case "reconnect_initial":
			return "controller.controlplane.reconnect_initial"
		case "reconnect_max":
			return "controller.controlplane.reconnect_max"
		case "polling_interval":
			return "controller.controlplane.polling_interval"
		case "insecure_skip_verify":
			return "controller.controlplane.insecure_skip_verify"
		// APIP_GW_ + CONTROLLER_CONTROLPLANE_* (underscore-to-dot would split insecure_skip_verify)
		case "controller_controlplane_host":
			return "controller.controlplane.host"
		case "controller_controlplane_token":
			return "controller.controlplane.token"
		case "controller_controlplane_reconnect_initial":
			return "controller.controlplane.reconnect_initial"
		case "controller_controlplane_reconnect_max":
			return "controller.controlplane.reconnect_max"
		case "controller_controlplane_polling_interval":
			return "controller.controlplane.polling_interval"
		case "controller_controlplane_insecure_skip_verify":
			return "controller.controlplane.insecure_skip_verify"
		case "controller_controlplane_deployment_push_enabled":
			return "controller.controlplane.deployment_push_enabled"
		case "controller_controlplane_sync_batch_size":
			return "controller.controlplane.sync_batch_size"
		case "immutable_gateway_enabled":
			return "immutable_gateway.enabled"
		case "controller_controlplane_gateway_name":
			return "controller.controlplane.gateway_name"
		default:
			// For other env vars, use standard mapping (underscore to dot)
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
		Controller: Controller{
			Server: ServerConfig{
				APIPort:                         9090,
				XDSPort:                         18000,
				ShutdownTimeout:                 15 * time.Second,
				GatewayID:                       constants.PlatformGatewayId,
				SkipInvalidDeploymentsOnStartup: false,
			},
			AdminServer: AdminServerConfig{
				Enabled:    true,
				Port:       9092,
				AllowedIPs: []string{"*"},
			},
			PolicyServer: PolicyServerConfig{
				Port: 18001,
				TLS: PolicyServerTLS{
					Enabled:  false,
					CertFile: "./certs/server.crt",
					KeyFile:  "./certs/server.key",
				},
			},
			Policies: PoliciesConfig{
				DefinitionsPath:   "./default-policies",
				BuildManifestPath: "./build-manifest.yaml",
			},
			LLM: LLMConfig{
				TemplateDefinitionsPath: "./default-llm-provider-templates",
			},
			Storage: StorageConfig{
				Type: "sqlite",
				SQLite: SQLiteConfig{
					Path: "./data/gateway.db",
				},
				Postgres: PostgresConfig{
					Port:            5432,
					SSLMode:         "require",
					ConnectTimeout:  5 * time.Second,
					MaxOpenConns:    25,
					MaxIdleConns:    5,
					ConnMaxLifetime: 30 * time.Minute,
					ConnMaxIdleTime: 5 * time.Minute,
					ApplicationName: "gateway-controller",
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
				Format: "text",
			},
			Metrics: MetricsConfig{
				Enabled: false,
				Port:    9091,
			},
			ControlPlane: ControlPlaneConfig{
				Host:                  "",
				Token:                 "",
				ReconnectInitial:      1 * time.Second,
				ReconnectMax:          5 * time.Minute,
				PollingInterval:       15 * time.Minute,
				InsecureSkipVerify:    false,
				DeploymentPushEnabled: false,
				SyncBatchSize:         50,
			},
			EventHub: EventHubConfig{
				PollInterval:    3 * time.Second,
				CleanupInterval: 10 * time.Minute,
				RetentionPeriod: 1 * time.Hour,
				Database: EventHubDatabaseConfig{
					MaxOpenConns:    5,
					MaxIdleConns:    2,
					ConnMaxLifetime: 30 * time.Minute,
					ConnMaxIdleTime: 5 * time.Minute,
				},
			},
			Encryption: EncryptionConfig{
				Providers: []ProviderConfig{
					{
						Type: "aesgcm",
						Keys: []EncryptionKeyConfig{
							{
								Version:  "aesgcm256-v1",
								FilePath: "./data/aesgcm-keys/default-aesgcm256-v1.bin",
							},
						},
					},
				},
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
				Format:  "text",
				JSONFields: map[string]string{
					"t":          "%START_TIME%",
					"meth":       "%REQ(:METHOD)%",
					"path":       "%REQ(X-ENVOY-ORIGINAL-PATH?:PATH)%",
					"proto":      "%PROTOCOL%",
					"respCd":     "%RESPONSE_CODE%",
					"respFlg":    "%RESPONSE_FLAGS%",
					"bytesRx":    "%BYTES_RECEIVED%",
					"bytesTx":    "%BYTES_SENT%",
					"dur":        "%DURATION%",
					"upSvcT":     "%RESP(X-ENVOY-UPSTREAM-SERVICE-TIME)%",
					"xff":        "%REQ(X-FORWARDED-FOR)%",
					"ua":         "%REQ(USER-AGENT)%",
					"reqId":      "%REQ(X-REQUEST-ID)%",
					"host":       "%REQ(:AUTHORITY)%",
					"upHost":     "%UPSTREAM_HOST%",
					"upProto":    "%UPSTREAM_PROTOCOL%",
					"upPath":     "%REQ(:PATH)%",
					"respCdDtl":  "%RESPONSE_CODE_DETAILS%",
					"connTrmDtl": "%CONNECTION_TERMINATION_DETAILS%",
					"reqTxDur":   "%REQUEST_TX_DURATION%",
					"respTxDur":  "%RESPONSE_TX_DURATION%",
					"reqDur":     "%REQUEST_DURATION%",
					"respDur":    "%RESPONSE_DURATION%",
				},
				TextFormat: "[%START_TIME%] \"%REQ(:METHOD)% %REQ(X-ENVOY-ORIGINAL-PATH?:PATH)% %PROTOCOL%\" " +
					"%REQ(:PATH)% %UPSTREAM_PROTOCOL% %RESPONSE_CODE% %RESPONSE_FLAGS% %RESPONSE_CODE_DETAILS% " +
					"%CONNECTION_TERMINATION_DETAILS% %BYTES_RECEIVED% %BYTES_SENT% %DURATION% " +
					"%REQUEST_TX_DURATION% %RESPONSE_TX_DURATION% %REQUEST_DURATION% %RESPONSE_DURATION% " +
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
			Upstream: RouterUpstream{
				TLS: UpstreamTLS{
					MinimumProtocolVersion: "TLS1_2",
					MaximumProtocolVersion: "TLS1_3",
					Ciphers:                "ECDHE-ECDSA-AES128-GCM-SHA256,ECDHE-RSA-AES128-GCM-SHA256,ECDHE-ECDSA-AES128-SHA,ECDHE-RSA-AES128-SHA,AES128-GCM-SHA256,AES128-SHA,ECDHE-ECDSA-AES256-GCM-SHA384,ECDHE-RSA-AES256-GCM-SHA384,ECDHE-ECDSA-AES256-SHA,ECDHE-RSA-AES256-SHA,AES256-GCM-SHA384,AES256-SHA",
					TrustedCertPath:        "/etc/ssl/certs/ca-certificates.crt",
					CustomCertsPath:        "./certificates",
					VerifyHostName:         true,
					DisableSslVerification: false,
				},
				Timeouts: UpstreamTimeouts{
					RouteTimeoutMs:     60000,
					RouteIdleTimeoutMs: 300000,
					ConnectTimeoutMs:   5000,
				},
			},
			PolicyEngine: PolicyEngineConfig{
				Mode:              "uds",           // UDS mode by default
				Host:              "policy-engine", // Only used in TCP mode
				Port:              9001,            // Only used in TCP mode
				TimeoutMs:         60000,
				FailureModeAllow:  false,
				AllowModeOverride: true,
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
			GRPCEventServerCfg: GRPCEventServerConfig{
				Mode:                "uds",       // UDS mode by default
				Port:                18090,       // Only used in TCP mode
				ServerPort:          18090,       // ALS server port
				BufferFlushInterval: 1000000000,  // 1 second
				BufferSizeBytes:     16384,       // 16 KiB
				GRPCRequestTimeout:  20000000000, // 20 seconds
				ShutdownTimeout:     600 * time.Second,
				PublicKeyPath:       "",
				PrivateKeyPath:      "",
				ALSPlainText:        true,
				MaxMessageSize:      1000000000,
				MaxHeaderLimit:      8192,
			},
			AllowPayloads: false,
		},
		TracingConfig: TracingConfig{
			Enabled:        false,
			Endpoint:       "otel-collector:4317",
			Insecure:       true,
			ServiceVersion: "1.0.0", BatchTimeout: 1 * time.Second,
			MaxExportBatchSize: 512,
			SamplingRate:       1.0,
		},
		APIKey: APIKeyConfig{
			APIKeysPerUserPerAPI: 10,
			Algorithm:            constants.HashingAlgorithmSHA256,
			MinKeyLength:         constants.DefaultMinAPIKeyLength,
			MaxKeyLength:         constants.DefaultMaxAPIKeyLength,
		},
		ImmutableGateway: ImmutableGatewayConfig{
			Enabled:      false,
			ArtifactsDir: "/etc/api-platform-gateway/immutable_gateway/artifacts",
		},
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Immutable mode requires SQLite — it starts with a fresh in-container DB on every boot,
	// which is incompatible with external shared databases like Postgres.
	if c.ImmutableGateway.Enabled && !strings.EqualFold(c.Controller.Storage.Type, "sqlite") {
		return fmt.Errorf("immutable_gateway.enabled=true requires storage.type=sqlite; got %q. "+
			"Immutable mode starts with a fresh in-container database on every boot and is incompatible with external databases",
			c.Controller.Storage.Type)
	}

	// Validate storage type
	validStorageTypes := []string{"sqlite", "postgres"}
	isValidType := false
	for _, t := range validStorageTypes {
		if c.Controller.Storage.Type == t {
			isValidType = true
			break
		}
	}
	if !isValidType {
		return fmt.Errorf("storage.type must be one of: sqlite, postgres, got: %s", c.Controller.Storage.Type)
	}

	// Validate SQLite configuration
	if c.Controller.Storage.Type == "sqlite" && c.Controller.Storage.SQLite.Path == "" {
		return fmt.Errorf("storage.sqlite.path is required when storage.type is 'sqlite'")
	}

	// Validate PostgreSQL configuration
	if c.Controller.Storage.Type == "postgres" {
		pg := &c.Controller.Storage.Postgres

		if pg.DSN == "" {
			if pg.Host == "" {
				return fmt.Errorf("storage.postgres.host is required when storage.type is 'postgres' and storage.postgres.dsn is empty")
			}
			if pg.Database == "" {
				return fmt.Errorf("storage.postgres.database is required when storage.type is 'postgres' and storage.postgres.dsn is empty")
			}
			if pg.User == "" {
				return fmt.Errorf("storage.postgres.user is required when storage.type is 'postgres' and storage.postgres.dsn is empty")
			}
		}

		if pg.Port <= 0 {
			pg.Port = 5432
		}
		if pg.Port > 65535 {
			return fmt.Errorf("storage.postgres.port must be between 1 and 65535, got: %d", pg.Port)
		}

		if pg.SSLMode == "" {
			pg.SSLMode = "require"
		}
		validSSLModes := []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}
		isValidSSLMode := false
		for _, mode := range validSSLModes {
			if strings.EqualFold(pg.SSLMode, mode) {
				pg.SSLMode = mode
				isValidSSLMode = true
				break
			}
		}
		if !isValidSSLMode {
			return fmt.Errorf("storage.postgres.sslmode must be one of: disable, allow, prefer, require, verify-ca, verify-full, got: %s", pg.SSLMode)
		}

		if pg.ConnectTimeout <= 0 {
			pg.ConnectTimeout = 5 * time.Second
		}

		if pg.MaxOpenConns == 0 {
			pg.MaxOpenConns = 25
		}
		if pg.MaxOpenConns < 1 {
			return fmt.Errorf("storage.postgres.max_open_conns must be >= 1, got: %d", pg.MaxOpenConns)
		}

		if pg.MaxIdleConns == 0 {
			pg.MaxIdleConns = 5
		}
		if pg.MaxIdleConns < 0 {
			return fmt.Errorf("storage.postgres.max_idle_conns must be >= 0, got: %d", pg.MaxIdleConns)
		}
		if pg.MaxIdleConns > pg.MaxOpenConns {
			pg.MaxIdleConns = pg.MaxOpenConns
		}

		if pg.ConnMaxLifetime == 0 {
			pg.ConnMaxLifetime = 30 * time.Minute
		}
		if pg.ConnMaxLifetime < 0 {
			return fmt.Errorf("storage.postgres.conn_max_lifetime must be >= 0, got: %s", pg.ConnMaxLifetime)
		}

		if pg.ConnMaxIdleTime == 0 {
			pg.ConnMaxIdleTime = 5 * time.Minute
		}
		if pg.ConnMaxIdleTime < 0 {
			return fmt.Errorf("storage.postgres.conn_max_idle_time must be >= 0, got: %s", pg.ConnMaxIdleTime)
		}

		if pg.ApplicationName == "" {
			pg.ApplicationName = "gateway-controller"
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
		if strings.ToLower(c.Controller.Logging.Level) == level {
			isValidLevel = true
			break
		}
	}
	if !isValidLevel {
		return fmt.Errorf("logging.level must be one of: debug, info, warn, error, got: %s", c.Controller.Logging.Level)
	}

	// Validate log format
	if c.Controller.Logging.Format != "json" && c.Controller.Logging.Format != "text" {
		return fmt.Errorf("logging.format must be either 'json' or 'text', got: %s", c.Controller.Logging.Format)
	}

	// Validate ports
	if c.Controller.Server.APIPort < 1 || c.Controller.Server.APIPort > 65535 {
		return fmt.Errorf("server.api_port must be between 1 and 65535, got: %d", c.Controller.Server.APIPort)
	}

	if c.Controller.Server.XDSPort < 1 || c.Controller.Server.XDSPort > 65535 {
		return fmt.Errorf("server.xds_port must be between 1 and 65535, got: %d", c.Controller.Server.XDSPort)
	}

	if strings.TrimSpace(c.Controller.Server.GatewayID) == "" {
		return fmt.Errorf("server.gateway_id is required and cannot be empty")
	}

	if c.Controller.AdminServer.Enabled {
		if c.Controller.AdminServer.Port < 1 || c.Controller.AdminServer.Port > 65535 {
			return fmt.Errorf("admin_server.port must be between 1 and 65535, got: %d", c.Controller.AdminServer.Port)
		}
		if c.Controller.AdminServer.Port == c.Controller.Server.APIPort {
			return fmt.Errorf("admin_server.port cannot be same as server.api_port")
		}
		if c.Controller.AdminServer.Port == c.Controller.Server.XDSPort {
			return fmt.Errorf("admin_server.port cannot be same as server.xds_port")
		}
	}

	// Validate metrics config
	if c.Controller.Metrics.Enabled {
		if c.Controller.Metrics.Port < 1 || c.Controller.Metrics.Port > 65535 {
			return fmt.Errorf("metrics.port must be between 1 and 65535, got: %d", c.Controller.Metrics.Port)
		}
		if c.Controller.Metrics.Port == c.Controller.Server.APIPort {
			return fmt.Errorf("metrics.port cannot be same as server.api_port")
		}
		if c.Controller.Metrics.Port == c.Controller.Server.XDSPort {
			return fmt.Errorf("metrics.port cannot be same as server.xds_port")
		}
		if c.Controller.AdminServer.Enabled && c.Controller.Metrics.Port == c.Controller.AdminServer.Port {
			return fmt.Errorf("metrics.port cannot be same as admin_server.port")
		}
	}

	if c.Router.ListenerPort < 1 || c.Router.ListenerPort > 65535 {
		return fmt.Errorf("router.listener_port must be between 1 and 65535, got: %d", c.Router.ListenerPort)
	}

	// Validate HTTPS port if HTTPS is enabled
	if c.Router.HTTPSEnabled {
		if c.Router.HTTPSPort < 1 || c.Router.HTTPSPort > 65535 {
			return fmt.Errorf("router.https_port must be between 1 and 65535, got: %d", c.Router.HTTPSPort)
		}
	}

	// Validate EventHub configuration
	eh := &c.Controller.EventHub
	if eh.PollInterval <= 0 {
		return fmt.Errorf("event_hub.poll_interval must be positive, got: %s", eh.PollInterval)
	}
	if eh.CleanupInterval <= 0 {
		return fmt.Errorf("event_hub.cleanup_interval must be positive, got: %s", eh.CleanupInterval)
	}
	if eh.RetentionPeriod <= 0 {
		return fmt.Errorf("event_hub.retention_period must be positive, got: %s", eh.RetentionPeriod)
	}

	// Validate event gateway configuration if enabled
	if c.Router.EventGateway.Enabled {
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

	// Validate subscriptions configuration (subscription token encryption key when set)
	if err := c.validateSubscriptionsConfig(); err != nil {
		return err
	}

	return nil
}

func (c *Config) validateEventGatewayConfig() error {
	if c.Router.EventGateway.WebSubHubPort < 1 || c.Router.EventGateway.WebSubHubPort > 65535 {
		return fmt.Errorf("router.event_gateway.websub_hub_port must be between 1 and 65535, got: %d", c.Router.EventGateway.WebSubHubPort)
	}
	if c.Router.EventGateway.WebSubHubListenerPort < 1 || c.Router.EventGateway.WebSubHubListenerPort > 65535 {
		return fmt.Errorf("router.event_gateway.websub_hub_listener_port must be between 1 and 65535, got: %d", c.Router.EventGateway.WebSubHubListenerPort)
	}

	// Validate WebSubHubURL if provided - must be a valid http(s) URL
	if strings.TrimSpace(c.Router.EventGateway.WebSubHubURL) != "" {
		u, err := url.Parse(c.Router.EventGateway.WebSubHubURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("router.event_gateway.websub_hub_url must be a valid URL with http or https scheme, got: %s", c.Router.EventGateway.WebSubHubURL)
		}
		if u.Host == "" {
			return fmt.Errorf("router.event_gateway.websub_hub_url must include a valid host, got: %s", c.Router.EventGateway.WebSubHubURL)
		}
	}
	if c.Router.EventGateway.TimeoutSeconds <= 0 {
		return fmt.Errorf("router.event_gateway.timeout_seconds must be positive, got: %d", c.Router.EventGateway.TimeoutSeconds)
	}
	return nil
}

// validateControlPlaneConfig validates the control plane configuration
func (c *Config) validateControlPlaneConfig() error {
	cp := &c.Controller.ControlPlane

	// If no host is set, the gateway runs in standalone mode — skip all CP validation.
	if cp.Host == "" {
		// A token without a host is a misconfiguration.
		if cp.Token != "" {
			return fmt.Errorf("controlplane.host is required when controlplane.token is set")
		}
		return nil
	}

	// Validate reconnection intervals
	if cp.ReconnectInitial <= 0 {
		return fmt.Errorf("controlplane.reconnect_initial must be positive, got: %s", cp.ReconnectInitial)
	}

	if cp.ReconnectMax <= 0 {
		return fmt.Errorf("controlplane.reconnect_max must be positive, got: %s", cp.ReconnectMax)
	}

	if cp.ReconnectInitial > cp.ReconnectMax {
		return fmt.Errorf("controlplane.reconnect_initial (%s) must be <= controlplane.reconnect_max (%s)",
			cp.ReconnectInitial, cp.ReconnectMax)
	}

	// Validate polling interval
	if cp.PollingInterval <= 0 {
		return fmt.Errorf("controlplane.polling_interval must be positive, got: %s", cp.PollingInterval)
	}

	// Validate sync batch size
	if cp.SyncBatchSize <= 0 {
		return fmt.Errorf("controlplane.sync_batch_size must be positive, got: %d", cp.SyncBatchSize)
	}

	return nil
}

// validateTLSConfig validates the upstream TLS configuration
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
		return fmt.Errorf("router.upstream.tls.minimum_protocol_version is required")
	}

	isValidMinVersion := false
	for _, version := range validTLSVersions {
		if minVersion == version {
			isValidMinVersion = true
			break
		}
	}
	if !isValidMinVersion {
		return fmt.Errorf("router.upstream.tls.minimum_protocol_version must be one of: %s, got: %s",
			strings.Join(validTLSVersions, ", "), minVersion)
	}

	// Validate maximum TLS version
	maxVersion := c.Router.Upstream.TLS.MaximumProtocolVersion
	if maxVersion == "" {
		return fmt.Errorf("router.upstream.tls.maximum_protocol_version is required")
	}

	isValidMaxVersion := false
	for _, version := range validTLSVersions {
		if maxVersion == version {
			isValidMaxVersion = true
			break
		}
	}
	if !isValidMaxVersion {
		return fmt.Errorf("router.upstream.tls.maximum_protocol_version must be one of: %s, got: %s",
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
		return fmt.Errorf("router.upstream.tls.minimum_protocol_version (%s) cannot be greater than maximum_protocol_version (%s)",
			minVersion, maxVersion)
	}

	// Validate cipher suites format (basic validation - ensure it's not empty if provided)
	ciphers := c.Router.Upstream.TLS.Ciphers
	if ciphers != "" {
		// Basic validation: ensure ciphers don't contain invalid characters
		if strings.Contains(ciphers, constants.CipherInvalidChars1) || strings.Contains(ciphers, constants.CipherInvalidChars2) {
			return fmt.Errorf("router.upstream.tls.ciphers contains invalid characters (use comma-separated values)")
		}

		// Ensure cipher list is not just whitespace
		if strings.TrimSpace(ciphers) == "" {
			return fmt.Errorf("router.upstream.tls.ciphers cannot be empty or whitespace only")
		}
	}

	// Validate trusted cert path if SSL verification is enabled
	if !c.Router.Upstream.TLS.DisableSslVerification && c.Router.Upstream.TLS.TrustedCertPath == "" {
		return fmt.Errorf("router.upstream.tls.trusted_cert_path is required when SSL verification is enabled")
	}

	// Validate downstream TLS configuration if HTTPS is enabled
	if c.Router.HTTPSEnabled {
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
	if c.Router.DownstreamTLS.CertPath == "" {
		return fmt.Errorf("router.downstream_tls.cert_path is required when HTTPS is enabled")
	}

	if c.Router.DownstreamTLS.KeyPath == "" {
		return fmt.Errorf("router.downstream_tls.key_path is required when HTTPS is enabled")
	}

	// Validate minimum TLS version
	minVersion := c.Router.DownstreamTLS.MinimumProtocolVersion
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
	maxVersion := c.Router.DownstreamTLS.MaximumProtocolVersion
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
	ciphers := c.Router.DownstreamTLS.Ciphers
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

// validateTimeoutConfig validates the upstream timeout configuration
func (c *Config) validateTimeoutConfig() error {
	timeouts := c.Router.Upstream.Timeouts

	// Validate route timeout
	if timeouts.RouteTimeoutMs <= 0 {
		return fmt.Errorf("router.upstream.timeouts.route_timeout_ms must be positive, got: %d",
			timeouts.RouteTimeoutMs)
	}

	// Validate idle timeout
	if timeouts.RouteIdleTimeoutMs <= 0 {
		return fmt.Errorf("router.upstream.timeouts.route_idle_timeout_ms must be positive, got: %d",
			timeouts.RouteIdleTimeoutMs)
	}

	// Validate connect timeout
	if timeouts.ConnectTimeoutMs <= 0 {
		return fmt.Errorf("router.upstream.timeouts.connect_timeout_ms must be positive, got: %d",
			timeouts.ConnectTimeoutMs)
	}

	// Validate reasonable timeout ranges (prevent extremely long timeouts)
	if timeouts.RouteTimeoutMs > constants.MaxReasonableTimeoutMs {
		return fmt.Errorf("router.upstream.timeouts.route_timeout_ms (%d) exceeds maximum reasonable timeout of %d ms",
			timeouts.RouteTimeoutMs, constants.MaxReasonableTimeoutMs)
	}

	if timeouts.RouteIdleTimeoutMs > constants.MaxReasonableTimeoutMs {
		return fmt.Errorf("router.upstream.timeouts.route_idle_timeout_ms (%d) exceeds maximum reasonable timeout of %d ms",
			timeouts.RouteIdleTimeoutMs, constants.MaxReasonableTimeoutMs)
	}

	if timeouts.ConnectTimeoutMs > constants.MaxReasonableTimeoutMs {
		return fmt.Errorf("router.upstream.timeouts.connect_timeout_ms (%d) exceeds maximum reasonable timeout of %d ms",
			timeouts.ConnectTimeoutMs, constants.MaxReasonableTimeoutMs)
	}

	return nil
}

// validatePolicyEngineConfig validates the policy engine configuration
func (c *Config) validatePolicyEngineConfig() error {
	policyEngine := c.Router.PolicyEngine

	// Validate connection mode
	switch policyEngine.Mode {
	case "uds", "":
		// UDS mode (default) - TLS is not supported with UDS (local communication)
		if policyEngine.TLS.Enabled {
			return fmt.Errorf("router.policy_engine.tls cannot be enabled when using Unix domain socket mode")
		}
	case "tcp":
		// TCP mode - validate host and port
		if policyEngine.Host == "" {
			return fmt.Errorf("router.policy_engine.host is required when mode is tcp")
		}
		if policyEngine.Port == 0 {
			return fmt.Errorf("router.policy_engine.port is required when mode is tcp")
		}
		if policyEngine.Port > 65535 {
			return fmt.Errorf("router.policy_engine.port must be between 1 and 65535, got: %d", policyEngine.Port)
		}
	default:
		return fmt.Errorf("router.policy_engine.mode must be 'uds' or 'tcp', got: %s", policyEngine.Mode)
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

	return nil
}

// validateVHostsConfig validates the vhosts configuration
func (c *Config) validateVHostsConfig() error {
	if strings.TrimSpace(c.Router.VHosts.Main.Default) == "" {
		return fmt.Errorf("router.vhosts.main.default must be a non-empty string")
	}
	if strings.TrimSpace(c.Router.VHosts.Sandbox.Default) == "" {
		return fmt.Errorf("router.vhosts.sandbox.default must be a non-empty string")
	}

	// Validate main.domains (only if not nil)
	if err := validateDomains("router.vhosts.main.domains", c.Router.VHosts.Main.Domains); err != nil {
		return err
	}

	// Validate sandbox.domains (only if not nil)
	if err := validateDomains("router.vhosts.sandbox.domains", c.Router.VHosts.Sandbox.Domains); err != nil {
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
		// Validate gRPC event server configuration
		grpcEventServerCfg := c.Analytics.GRPCEventServerCfg

		// Validate connection mode
		switch grpcEventServerCfg.Mode {
		case "uds", "":
			// UDS mode (default) - port is unused for Envoy connection
		case "tcp":
			// TCP mode - validate port (host is derived from policy_engine.host)
			if grpcEventServerCfg.Port <= 0 || grpcEventServerCfg.Port > 65535 {
				return fmt.Errorf("analytics.grpc_event_server.port must be between 1 and 65535 when mode is tcp, got %d", grpcEventServerCfg.Port)
			}
		default:
			return fmt.Errorf("analytics.grpc_event_server.mode must be 'uds' or 'tcp', got: %s", grpcEventServerCfg.Mode)
		}

		// Validate buffer and timeout settings
		if grpcEventServerCfg.BufferFlushInterval <= 0 || grpcEventServerCfg.BufferSizeBytes <= 0 || grpcEventServerCfg.GRPCRequestTimeout <= 0 {
			return fmt.Errorf(
				"invalid gRPC event server configuration: bufferFlushInterval=%d, bufferSizeBytes=%d, grpcRequestTimeout=%d (all must be > 0)",
				grpcEventServerCfg.BufferFlushInterval,
				grpcEventServerCfg.BufferSizeBytes,
				grpcEventServerCfg.GRPCRequestTimeout,
			)
		}

		// Validate server port
		if grpcEventServerCfg.ServerPort <= 0 || grpcEventServerCfg.ServerPort > 65535 {
			return fmt.Errorf("analytics.grpc_event_server.server_port must be between 1 and 65535, got %d", grpcEventServerCfg.ServerPort)
		}
	}
	return nil
}

// validateAuthConfig validates the authentication configuration
func (c *Config) validateAuthConfig() error {
	// Validate IDP role mapping for multiple wildcards
	if c.Controller.Auth.IDP.Enabled && len(c.Controller.Auth.IDP.RoleMapping) > 0 {
		wildcardRoles := []string{}
		for localRole, idpRoles := range c.Controller.Auth.IDP.RoleMapping {
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
	if c.APIKey.APIKeysPerUserPerAPI <= 0 {
		return fmt.Errorf("api_key.api_keys_per_user_per_api must be a positive integer, got: %d",
			c.APIKey.APIKeysPerUserPerAPI)
	}

	// Default min/max key lengths if not configured
	if c.APIKey.MinKeyLength <= 0 {
		c.APIKey.MinKeyLength = constants.DefaultMinAPIKeyLength
	}
	if c.APIKey.MaxKeyLength <= 0 {
		c.APIKey.MaxKeyLength = constants.DefaultMaxAPIKeyLength
	}
	if c.APIKey.MinKeyLength > c.APIKey.MaxKeyLength {
		return fmt.Errorf("api_key.min_key_length (%d) must not exceed api_key.max_key_length (%d)",
			c.APIKey.MinKeyLength, c.APIKey.MaxKeyLength)
	}

	// If hashing is enabled but no algorithm is provided, default to SHA256
	if c.APIKey.Algorithm == "" {
		c.APIKey.Algorithm = constants.HashingAlgorithmSHA256
		return nil
	}

	// Only SHA256 is supported
	if strings.ToLower(c.APIKey.Algorithm) != constants.HashingAlgorithmSHA256 {
		return fmt.Errorf("api_key.algorithm must be %s, got: %s",
			constants.HashingAlgorithmSHA256, c.APIKey.Algorithm)
	}
	return nil
}

// validateSubscriptionsConfig validates subscriptions configuration.
func (c *Config) validateSubscriptionsConfig() error {
	return nil
}

// IsAccessLogsEnabled returns true if access logs are enabled
func (c *Config) IsAccessLogsEnabled() bool {
	return c.Router.AccessLogs.Enabled
}

// validateHTTPListenerConfig validates the HTTP listener configuration
func (c *Config) validateHTTPListenerConfig() error {
	httpListener := &c.Router.HTTPListener

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
