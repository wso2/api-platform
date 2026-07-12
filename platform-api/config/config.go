/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package config

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/go-viper/mapstructure/v2"
	toml "github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/confmap"
	kenv "github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"

	"github.com/wso2/api-platform/common/configinterpolate"
	"github.com/wso2/api-platform/platform-api/internal/logger"
)

// FileBasedUser represents a built-in user for file-based auth mode.
type FileBasedUser struct {
	Username     string `json:"username"     koanf:"username"`
	PasswordHash string `json:"password_hash" koanf:"password_hash"`
	Scopes       string `json:"scopes"       koanf:"scopes"`
}

// FileBasedUsers is a slice of FileBasedUser that can be decoded from a JSON string (env var)
// or from a TOML array of tables ([[auth.file_based.users]]).
type FileBasedUsers []FileBasedUser

// FileBasedOrg holds the single organization used in file-based auth mode.
type FileBasedOrg struct {
	// ID is the organization handle (URL-safe slug), e.g. "default".
	ID string `koanf:"id"`

	// DisplayName is the human-readable name of the organization.
	DisplayName string `koanf:"display_name"`

	// Region is the deployment region for the organization.
	Region string `koanf:"region"`

	// UUID is the platform organization UUID. File-based auth has no external
	// IDP, so this value is stored as idp_organization_ref_uuid and emitted as
	// the `organization` claim in issued tokens.
	UUID string `koanf:"uuid"`
}

// FileBased holds configuration for local username/password authentication.
type FileBased struct {
	Enabled      bool           `koanf:"enabled"`
	Organization FileBasedOrg   `koanf:"organization"`
	Users        FileBasedUsers `koanf:"users"`
}

// Server holds the configuration parameters for the application.
type Server struct {
	LogLevel  string `koanf:"log_level"`
	LogFormat string `koanf:"log_format"`

	DBSchemaPath               string `koanf:"db_schema_path"`
	OpenAPISpecPath            string `koanf:"openapi_spec_path"`
	LLMTemplateDefinitionsPath string `koanf:"llm_template_definitions_path"`

	EncryptionKey string `koanf:"encryption_key"`

	Database         Database         `koanf:"database"`
	Auth             Auth             `koanf:"auth"`
	WebSocket        WebSocket        `koanf:"websocket"`
	DefaultDevPortal DefaultDevPortal `koanf:"default_devportal"`
	Deployments      Deployments      `koanf:"deployments"`
	ArtifactLimits   ArtifactLimits   `koanf:"artifact_limits"`
	HTTP             HTTPListener     `koanf:"http"`
	HTTPS            HTTPSListener    `koanf:"https"`
	Timeouts         Timeouts         `koanf:"timeouts"`
	CORS             CORS             `koanf:"cors"`
	APIKey           APIKey           `koanf:"api_key"`
	Gateway          Gateway          `koanf:"gateway"`
	EventHub         EventHub         `koanf:"event_hub"`
	Webhook          Webhook          `koanf:"webhook"`

	EnableScopeValidation bool `koanf:"enable_scope_validation"`
}

// Auth groups all authentication-related configuration.
type Auth struct {
	SkipPaths []string  `koanf:"skip_paths"`
	IDP       IDP       `koanf:"idp"`
	JWT       JWT       `koanf:"jwt"`
	FileBased FileBased `koanf:"file_based"`
}

// IDPClaimMappings holds JWT claim name mappings for an IDP.
type IDPClaimMappings struct {
	OrganizationClaimName string `koanf:"organization_claim_name"`
	OrgNameClaimName      string `koanf:"org_name_claim_name"`
	OrgHandleClaimName    string `koanf:"org_handle_claim_name"`
	UserIDClaimName       string `koanf:"user_id_claim_name"`
	UsernameClaimName     string `koanf:"username_claim_name"`
	EmailClaimName        string `koanf:"email_claim_name"`
	ScopeClaimName        string `koanf:"scope_claim_name"`
	RolesClaimPath        string `koanf:"roles_claim_path"`
}

// IDP holds configuration for JWKS-based identity providers.
type IDP struct {
	Enabled          bool             `koanf:"enabled"`
	Name             string           `koanf:"name"`
	JWKSUrl          string           `koanf:"jwks_url"`
	Issuer           []string         `koanf:"issuer"`
	Audience         []string         `koanf:"audience"`
	ValidationMode   string           `koanf:"validation_mode"`
	RoleMappingsFile string           `koanf:"role_mappings_file"`
	ClaimMappings    IDPClaimMappings `koanf:"claim_mappings"`
}

// EventHub holds EventHub-specific configuration for multi-replica HA event delivery.
type EventHub struct {
	PollInterval    time.Duration `koanf:"poll_interval"`
	CleanupInterval time.Duration `koanf:"cleanup_interval"`
	RetentionPeriod time.Duration `koanf:"retention_period"`
}

// Webhook holds configuration for the control-plane webhook receiver. The Developer Portal
// delivers signed events (API key / subscription changes) to this endpoint. See
// docs-local/platform-api-webhook.md.
type Webhook struct {
	// Enabled controls whether the webhook endpoint is registered.
	Enabled bool `koanf:"enabled"`
	// Secret is the HMAC-SHA256 shared secret used to verify request signatures.
	Secret string `koanf:"secret"`
	// PrivateKeyPath points to the PEM RSA private key used to decrypt encrypted_key fields.
	// Optional: required only for events that carry encrypted secrets (API key generate/regenerate).
	PrivateKeyPath string `koanf:"private_key_path"`
	// GatewayType filters events meant for this platform type. Events with a different
	// gateway_type are accepted as a no-op.
	GatewayType string `koanf:"gateway_type"`
	// SignatureTolerance bounds how old a signed request may be (replay protection).
	SignatureTolerance time.Duration `koanf:"signature_tolerance"`
	// MaxBodySize caps the request body size in bytes.
	MaxBodySize int64 `koanf:"max_body_size"`
	// SignatureHeader is the header carrying the "t=...,v1=..." signature.
	SignatureHeader string `koanf:"signature_header"`
}

// Gateway holds gateway-related configuration.
type Gateway struct {
	EnableVersionVerification           bool `koanf:"enable_version_verification"`
	EnableFunctionalityTypeVerification bool `koanf:"enable_functionality_type_verification"`
}

// HTTPListener and HTTPSListener model the two independent listeners, following
// the gateway router's http/https split (see RouterConfig.HTTPSEnabled /
// HTTPSPort in gateway-controller). Each is enabled independently and bound to
// its own port, so a deployment can serve plain HTTP internally, HTTPS
// externally, or both at once (e.g. to migrate clients between the two without
// downtime).

// HTTPListener configures the plain-HTTP listener. Enable it only when a trusted
// upstream (ingress, service-mesh sidecar) terminates TLS, or for internal
// cluster traffic; never expose it directly to untrusted networks.
type HTTPListener struct {
	Enabled bool   `koanf:"enabled"`
	Port    string `koanf:"port"`
}

// HTTPSListener configures the TLS listener. CertDir must contain cert.pem and
// key.pem when Enabled is true; in demo mode a self-signed pair is generated
// there when none is present.
type HTTPSListener struct {
	Enabled bool   `koanf:"enabled"`
	Port    string `koanf:"port"`
	CertDir string `koanf:"cert_dir"`
}

// Timeouts bounds the lifetime of a connection on both listeners, so a slow or
// idle peer cannot hold one open indefinitely (Slowloris). The values apply to
// the plain-HTTP and HTTPS listeners alike, since both serve the same handler.
//
// A zero value disables the corresponding timeout, matching net/http semantics.
// Disabling Read or ReadHeader removes the Slowloris protection — only do so
// behind a proxy that already enforces its own bounds.
//
// WebSocket routes are unaffected: gorilla/websocket clears the hijacked
// connection's deadlines during the upgrade, so long-lived sockets outlive these.
type Timeouts struct {
	// ReadHeader bounds how long a client may take to send request headers.
	ReadHeader time.Duration `koanf:"read_header"`
	// Read bounds the whole request read, including bodies such as uploaded API
	// definitions. Must be >= ReadHeader when both are set.
	Read time.Duration `koanf:"read"`
	// Write bounds handler execution plus the response write. Keep it generous:
	// some handlers proxy slow upstreams (LLM completions, deployments).
	Write time.Duration `koanf:"write"`
	// Idle bounds how long a keep-alive connection may sit unused between
	// requests.
	Idle time.Duration `koanf:"idle"`
}

// CORS holds cross-origin resource sharing configuration.
type CORS struct {
	// AllowedOrigins lists the exact origins permitted to make credentialed
	// cross-origin requests. Must not be ["*"] outside demo mode — wildcard
	// origins cannot be combined with credentialed requests.
	AllowedOrigins []string `koanf:"allowed_origins"`
}

// JWT holds configuration for local HMAC JWT authentication.
type JWT struct {
	Enabled        bool   `koanf:"enabled"`
	SecretKey      string `koanf:"secret_key"`
	Issuer         string `koanf:"issuer"`
	SkipValidation bool   `koanf:"skip_validation"`
}

// WebSocket holds WebSocket-specific configuration.
type WebSocket struct {
	MaxConnections       int  `koanf:"max_connections"`
	ConnectionTimeout    int  `koanf:"connection_timeout"`
	RateLimitPerMin      int  `koanf:"rate_limit_per_min"`
	MaxConnectionsPerOrg int  `koanf:"max_connections_per_org"`
	MetricsLogEnabled    bool `koanf:"metrics_log_enabled"`
	MetricsLogInterval   int  `koanf:"metrics_log_interval"`
}

// Database holds database-specific configuration.
type Database struct {
	// Driver supports: sqlite3, postgres/postgresql/pgx, sqlserver/mssql.
	Driver string `koanf:"driver"`
	// Path is the file path for SQLite databases.
	Path            string `koanf:"path"`
	Host            string `koanf:"host"`
	Port            int    `koanf:"port"`
	Name            string `koanf:"name"`
	User            string `koanf:"user"`
	Password        string `koanf:"password"`
	SSLMode         string `koanf:"ssl_mode"`
	MaxOpenConns    int    `koanf:"max_open_conns"`
	MaxIdleConns    int    `koanf:"max_idle_conns"`
	ConnMaxLifetime int    `koanf:"conn_max_lifetime"`
}

// DefaultDevPortal holds default DevPortal configuration for new organizations.
type DefaultDevPortal struct {
	Enabled       bool   `koanf:"enabled"`
	Name          string `koanf:"name"`
	Identifier    string `koanf:"identifier"`
	APIUrl        string `koanf:"api_url"`
	Hostname      string `koanf:"hostname"`
	APIKey        string `koanf:"api_key"`
	HeaderKeyName string `koanf:"header_key_name"`
	Timeout       int    `koanf:"timeout"`

	RoleClaimName         string `koanf:"role_claim_name"`
	GroupsClaimName       string `koanf:"groups_claim_name"`
	OrganizationClaimName string `koanf:"organization_claim_name"`
	AdminRole             string `koanf:"admin_role"`
	SubscriberRole        string `koanf:"subscriber_role"`
	SuperAdminRole        string `koanf:"super_admin_role"`
}

// Deployments holds deployment-specific configuration.
type Deployments struct {
	MaxPerAPIGateway          int  `koanf:"max_per_api_gateway"`
	TransitionalStatusEnabled bool `koanf:"transitional_status_enabled"`
	TimeoutEnabled            bool `koanf:"timeout_enabled"`
	TimeoutInterval           int  `koanf:"timeout_interval"`
	TimeoutDuration           int  `koanf:"timeout_duration"`
}

// ArtifactLimits holds the maximum number of each artifact kind an organization
// may create. Each limit is optional: a value <= 0 (the default) means unlimited,
// so organizations may create as many artifacts of that kind as they want.
type ArtifactLimits struct {
	MaxLLMProvidersPerOrg  int `koanf:"max_llm_providers_per_org"`
	MaxLLMProxiesPerOrg    int `koanf:"max_llm_proxies_per_org"`
	MaxMCPProxiesPerOrg    int `koanf:"max_mcp_proxies_per_org"`
	MaxWebSubAPIsPerOrg    int `koanf:"max_websub_apis_per_org"`
	MaxWebBrokerAPIsPerOrg int `koanf:"max_webbroker_apis_per_org"`
}

// LimitReached reports whether an organization currently holding currentCount
// artifacts of some kind has reached its configured limit. A limit <= 0 means
// unlimited, in which case this always returns false.
func LimitReached(currentCount, limit int) bool {
	return limit > 0 && currentCount >= limit
}

// APIKey holds API key-specific configuration.
type APIKey struct {
	HashingAlgorithms []string `koanf:"hashing_algorithms"`
}

// package-level singleton.
var (
	configFilePath  string
	processOnce     sync.Once
	settingInstance *Server
)

// SetConfigPath configures the path to a config.toml file.
// Must be called before the first GetConfig() if a config file is used.
func SetConfigPath(path string) {
	configFilePath = path
}

// GetConfig returns the singleton config instance, loading it on first call.
func GetConfig() *Server {
	var err error
	processOnce.Do(func() {
		settingInstance, err = LoadConfig(configFilePath)
	})
	if err != nil {
		panic(err)
	}
	return settingInstance
}

// defaultFileSourceAllowlist is the platform-api's default set of directories that a
// {{ file "..." }} config-interpolation token may read from. It can be overridden via
// the shared APIP_CONFIG_FILE_SOURCE_ALLOWLIST env var (see
// configinterpolate.ResolveAllowlist).
var defaultFileSourceAllowlist = []string{
	"/etc/platform-api",
	"/secrets/platform-api",
}

// LoadConfig loads configuration with priority: env vars > config file > defaults.
// configPath may be empty — when omitted only env vars and defaults are used.
func LoadConfig(configPath string) (*Server, error) {
	cfg := defaultConfig()
	k := koanf.New(".")

	if configPath != "" {
		if err := k.Load(file.Provider(configPath), toml.Parser()); err != nil {
			return nil, fmt.Errorf("failed to load config file %q: %w", configPath, err)
		}
	}

	// Load environment variables. The callback maps known env var names to koanf
	// dot-notation keys; unknown vars or empty values return "" and are skipped.
	// Empty values are skipped so that ${VAR:-} placeholders in docker-compose
	// do not override non-empty values already loaded from the config file.
	if err := k.Load(kenv.ProviderWithValue("", ".", func(s, v string) (string, interface{}) {
		if v == "" {
			return "", nil
		}
		return envToKoanfKey(strings.ToLower(s)), v
	}), nil); err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	// Resolve {{ env }} / {{ file }} interpolation tokens after the env+file merge
	// and before unmarshal, so any config field may pull its value from an
	// environment variable or an allowlisted file. String leaves without a "{{"
	// token pass through unchanged, so a token-free config is unaffected.
	k, err := interpolate(k)
	if err != nil {
		return nil, err
	}

	if err := k.UnmarshalWithConf("", cfg, koanf.UnmarshalConf{
		DecoderConfig: &mapstructure.DecoderConfig{
			TagName:          "koanf",
			WeaklyTypedInput: true,
			Result:           cfg,
			DecodeHook: mapstructure.ComposeDecodeHookFunc(
				mapstructure.StringToSliceHookFunc(","),
				mapstructure.StringToTimeDurationHookFunc(),
				fileBasedUsersDecodeHook(),
			),
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Install the configured logger as the slog default so the warnings/info logs
	// emitted below (and any package-level slog.* call in this file) use the same
	// format as the rest of the application, instead of slog's default handler.
	slog.SetDefault(logger.NewLogger(logger.Config{Level: cfg.LogLevel, Format: cfg.LogFormat}))

	if err := validateTimeoutsConfig(&cfg.Timeouts); err != nil {
		return nil, err
	}
	if err := validateDefaultDevPortalConfig(&cfg.DefaultDevPortal); err != nil {
		return nil, err
	}
	if err := validateDeploymentsConfig(&cfg.Deployments); err != nil {
		return nil, err
	}
	if err := validateEventHubConfig(&cfg.EventHub); err != nil {
		return nil, err
	}
	if err := validateIDPConfig(&cfg.Auth.IDP); err != nil {
		return nil, err
	}
	if err := validateWebhookConfig(&cfg.Webhook); err != nil {
		return nil, err
	}
	if err := validateFileBasedConfig(&cfg.Auth.FileBased); err != nil {
		return nil, err
	}
	if err := validateAuthModeExclusivity(&cfg.Auth); err != nil {
		return nil, err
	}

	if cfg.Auth.JWT.Enabled {
		if cfg.Auth.JWT.SecretKey == "" {
			return nil, fmt.Errorf("AUTH_JWT_SECRET_KEY is required when JWT authentication is enabled; " +
				"generate one with: openssl rand -hex 32")
		}
		if !valid32ByteKey(cfg.Auth.JWT.SecretKey) {
			return nil, fmt.Errorf("invalid AUTH_JWT_SECRET_KEY: must be 64 hex characters or " +
				"base64 decoding to 32 bytes (generate one with: openssl rand -hex 32)")
		}
	}

	if cfg.EncryptionKey == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY is required; generate one with: openssl rand -hex 32")
	}
	if !valid32ByteKey(cfg.EncryptionKey) {
		return nil, fmt.Errorf("invalid ENCRYPTION_KEY: must be 64 hex characters or " +
			"base64 decoding to 32 bytes (generate one with: openssl rand -hex 32)")
	}

	return cfg, nil
}

// interpolate resolves Go template tokens ({{ env }} / {{ file }}) in the merged
// config and returns a fresh koanf instance holding the expanded values. It loads the
// expanded map into a new instance (rather than reloading into k) so no un-expanded
// leaves survive. The file-source allowlist is the platform-api default, overridable
// via the shared APIP_CONFIG_FILE_SOURCE_ALLOWLIST env var. Resolved values are never
// logged; only reference counts are emitted at info level.
func interpolate(k *koanf.Koanf) (*koanf.Koanf, error) {
	opts := configinterpolate.Options{
		FileAllowlist: configinterpolate.ResolveAllowlist(defaultFileSourceAllowlist),
	}
	expanded, stats, err := configinterpolate.Expand(k.Raw(), opts)
	if err != nil {
		return nil, fmt.Errorf("config interpolation failed: %w", err)
	}

	out := koanf.New(".")
	if err := out.Load(confmap.Provider(expanded, "."), nil); err != nil {
		return nil, fmt.Errorf("failed to reload interpolated config: %w", err)
	}
	if stats.Fields > 0 {
		slog.Info("config interpolation complete",
			slog.Int("env_refs", stats.EnvRefs),
			slog.Int("file_refs", stats.FileRefs),
			slog.Int("fields", stats.Fields))
	}
	return out, nil
}

// valid32ByteKey reports whether keyStr is a 32-byte key encoded as 64 hex characters
// or base64 decoding to 32 bytes — matching utils.DeriveEncryptionKey's acceptance.
func valid32ByteKey(keyStr string) bool {
	if len(keyStr) == 64 {
		if k, err := hex.DecodeString(keyStr); err == nil && len(k) == 32 {
			return true
		}
	}
	if k, err := base64.StdEncoding.DecodeString(keyStr); err == nil && len(k) == 32 {
		return true
	}
	return false
}

// envToKoanfKey maps a lowercased environment variable name to its koanf dot-notation key.
// Returns "" for unknown variables, which causes koanf to skip them.
// Supports both the current env var names (e.g. DATABASE_DB_PATH) and the legacy
// WEBSOCKET_WS_* naming from the old envconfig setup.
func envToKoanfKey(s string) string {
	switch s {
	// Server-level
	case "log_level":
		return "log_level"
	case "log_format":
		return "log_format"
	case "db_schema_path":
		return "db_schema_path"
	case "openapi_spec_path":
		return "openapi_spec_path"
	case "llm_template_definitions_path":
		return "llm_template_definitions_path"
	case "enable_scope_validation":
		return "enable_scope_validation"
	case "encryption_key":
		return "encryption_key"

	// Database
	case "database_driver":
		return "database.driver"
	case "database_db_path":
		return "database.path"
	case "database_host":
		return "database.host"
	case "database_port":
		return "database.port"
	case "database_name":
		return "database.name"
	case "database_user":
		return "database.user"
	case "database_password":
		return "database.password"
	case "database_ssl_mode":
		return "database.ssl_mode"
	case "database_max_open_conns":
		return "database.max_open_conns"
	case "database_max_idle_conns":
		return "database.max_idle_conns"
	case "database_conn_max_lifetime":
		return "database.conn_max_lifetime"

	// Auth
	case "auth_skip_paths":
		return "auth.skip_paths"

	// Auth JWT
	case "auth_jwt_enabled":
		return "auth.jwt.enabled"
	case "auth_jwt_secret_key":
		return "auth.jwt.secret_key"
	case "auth_jwt_issuer":
		return "auth.jwt.issuer"
	case "auth_jwt_skip_validation":
		return "auth.jwt.skip_validation"

	// Auth IDP
	case "auth_idp_enabled":
		return "auth.idp.enabled"
	case "auth_idp_name":
		return "auth.idp.name"
	case "auth_idp_jwks_url":
		return "auth.idp.jwks_url"
	case "auth_idp_issuer":
		return "auth.idp.issuer"
	case "auth_idp_audience":
		return "auth.idp.audience"
	case "auth_idp_validation_mode":
		return "auth.idp.validation_mode"
	case "auth_idp_role_mappings_file":
		return "auth.idp.role_mappings_file"
	case "auth_idp_claim_mappings_organization_claim_name":
		return "auth.idp.claim_mappings.organization_claim_name"
	case "auth_idp_claim_mappings_org_name_claim_name":
		return "auth.idp.claim_mappings.org_name_claim_name"
	case "auth_idp_claim_mappings_org_handle_claim_name":
		return "auth.idp.claim_mappings.org_handle_claim_name"
	case "auth_idp_claim_mappings_user_id_claim_name":
		return "auth.idp.claim_mappings.user_id_claim_name"
	case "auth_idp_claim_mappings_username_claim_name":
		return "auth.idp.claim_mappings.username_claim_name"
	case "auth_idp_claim_mappings_email_claim_name":
		return "auth.idp.claim_mappings.email_claim_name"
	case "auth_idp_claim_mappings_scope_claim_name":
		return "auth.idp.claim_mappings.scope_claim_name"
	case "auth_idp_claim_mappings_roles_claim_path":
		return "auth.idp.claim_mappings.roles_claim_path"

	// Auth FileBased
	case "auth_file_based_enabled":
		return "auth.file_based.enabled"
	case "auth_file_based_organization_id":
		return "auth.file_based.organization.id"
	case "auth_file_based_organization_uuid":
		return "auth.file_based.organization.uuid"
	case "auth_file_based_organization_display_name":
		return "auth.file_based.organization.display_name"
	case "auth_file_based_organization_region":
		return "auth.file_based.organization.region"
	case "auth_file_based_users":
		return "auth.file_based.users"

	// WebSocket — accept both legacy WEBSOCKET_WS_* and clean WEBSOCKET_*
	case "websocket_ws_max_connections", "websocket_max_connections":
		return "websocket.max_connections"
	case "websocket_ws_connection_timeout", "websocket_connection_timeout":
		return "websocket.connection_timeout"
	case "websocket_ws_rate_limit_per_minute", "websocket_rate_limit_per_min":
		return "websocket.rate_limit_per_min"
	case "websocket_ws_max_connections_per_org", "websocket_max_connections_per_org":
		return "websocket.max_connections_per_org"
	case "websocket_ws_metrics_log_enabled", "websocket_metrics_log_enabled":
		return "websocket.metrics_log_enabled"
	case "websocket_ws_metrics_log_interval", "websocket_metrics_log_interval":
		return "websocket.metrics_log_interval"

	// Default DevPortal
	case "default_devportal_enabled":
		return "default_devportal.enabled"
	case "default_devportal_name":
		return "default_devportal.name"
	case "default_devportal_identifier":
		return "default_devportal.identifier"
	case "default_devportal_api_url":
		return "default_devportal.api_url"
	case "default_devportal_hostname":
		return "default_devportal.hostname"
	case "default_devportal_api_key":
		return "default_devportal.api_key"
	case "default_devportal_header_key_name":
		return "default_devportal.header_key_name"
	case "default_devportal_timeout":
		return "default_devportal.timeout"
	case "default_devportal_role_claim_name":
		return "default_devportal.role_claim_name"
	case "default_devportal_groups_claim_name":
		return "default_devportal.groups_claim_name"
	case "default_devportal_organization_claim_name":
		return "default_devportal.organization_claim_name"
	case "default_devportal_admin_role":
		return "default_devportal.admin_role"
	case "default_devportal_subscriber_role":
		return "default_devportal.subscriber_role"
	case "default_devportal_super_admin_role":
		return "default_devportal.super_admin_role"

	// Deployments
	case "deployments_max_per_api_gateway":
		return "deployments.max_per_api_gateway"
	case "deployments_transitional_status_enabled":
		return "deployments.transitional_status_enabled"
	case "deployments_timeout_enabled":
		return "deployments.timeout_enabled"
	case "deployments_timeout_interval":
		return "deployments.timeout_interval"
	case "deployments_timeout_duration":
		return "deployments.timeout_duration"

	// Artifact limits (per organization; <= 0 means unlimited)
	case "artifact_limits_max_llm_providers_per_org":
		return "artifact_limits.max_llm_providers_per_org"
	case "artifact_limits_max_llm_proxies_per_org":
		return "artifact_limits.max_llm_proxies_per_org"
	case "artifact_limits_max_mcp_proxies_per_org":
		return "artifact_limits.max_mcp_proxies_per_org"
	case "artifact_limits_max_websub_apis_per_org":
		return "artifact_limits.max_websub_apis_per_org"
	case "artifact_limits_max_webbroker_apis_per_org":
		return "artifact_limits.max_webbroker_apis_per_org"

	// Plain-HTTP listener
	case "http_enabled":
		return "http.enabled"
	case "http_port":
		return "http.port"

	// HTTPS / TLS listener.
	// Legacy TLS_ENABLED / TLS_CERT_DIR and the single-listener PORT are still
	// honored and map onto the HTTPS listener, which historically served on 9243.
	case "https_enabled", "tls_enabled":
		return "https.enabled"
	case "https_port", "port":
		return "https.port"
	case "https_cert_dir", "tls_cert_dir":
		return "https.cert_dir"

	// Listener timeouts (apply to both listeners). Values are durations, e.g.
	// "10s", "2m". 0 disables the timeout.
	case "timeouts_read_header":
		return "timeouts.read_header"
	case "timeouts_read":
		return "timeouts.read"
	case "timeouts_write":
		return "timeouts.write"
	case "timeouts_idle":
		return "timeouts.idle"

	// CORS
	case "cors_allowed_origins":
		return "cors.allowed_origins"

	// API Key
	case "api_key_hashing_algorithms":
		return "api_key.hashing_algorithms"

	// Gateway
	case "gateway_enable_version_verification":
		return "gateway.enable_version_verification"
	case "gateway_enable_functionality_type_verification":
		return "gateway.enable_functionality_type_verification"

	// EventHub
	case "event_hub_poll_interval":
		return "event_hub.poll_interval"
	case "event_hub_cleanup_interval":
		return "event_hub.cleanup_interval"
	case "event_hub_retention_period":
		return "event_hub.retention_period"

	// Webhook
	case "webhook_enabled":
		return "webhook.enabled"
	case "webhook_secret":
		return "webhook.secret"
	case "webhook_private_key_path":
		return "webhook.private_key_path"
	case "webhook_gateway_type":
		return "webhook.gateway_type"
	case "webhook_signature_tolerance":
		return "webhook.signature_tolerance"
	case "webhook_max_body_size":
		return "webhook.max_body_size"
	case "webhook_signature_header":
		return "webhook.signature_header"

	default:
		return ""
	}
}

// fileBasedUsersDecodeHook handles decoding AUTH_FILE_BASED_USERS from a JSON string
// (env var format) in addition to the native TOML array-of-tables format.
func fileBasedUsersDecodeHook() mapstructure.DecodeHookFuncType {
	return func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
		if t != reflect.TypeOf(FileBasedUsers{}) {
			return data, nil
		}
		s, ok := data.(string)
		if !ok {
			return data, nil
		}
		if s == "" {
			return FileBasedUsers{}, nil
		}
		var users FileBasedUsers
		if err := json.Unmarshal([]byte(s), &users); err != nil {
			return nil, fmt.Errorf("failed to parse AUTH_FILE_BASED_USERS as JSON: %w", err)
		}
		return users, nil
	}
}

// validateTimeoutsConfig rejects negative durations (net/http treats only zero as
// "no timeout"; a negative deadline would expire immediately and break every
// request) and a Read bound that would cut off header reading before ReadHeader.
func validateTimeoutsConfig(cfg *Timeouts) error {
	for _, f := range []struct {
		name  string
		value time.Duration
	}{
		{"timeouts.read_header (TIMEOUTS_READ_HEADER)", cfg.ReadHeader},
		{"timeouts.read (TIMEOUTS_READ)", cfg.Read},
		{"timeouts.write (TIMEOUTS_WRITE)", cfg.Write},
		{"timeouts.idle (TIMEOUTS_IDLE)", cfg.Idle},
	} {
		if f.value < 0 {
			return fmt.Errorf("%s must not be negative (got %s); use 0 to disable the timeout", f.name, f.value)
		}
	}
	if cfg.Read > 0 && cfg.ReadHeader > cfg.Read {
		return fmt.Errorf(
			"timeouts.read_header (%s) must not exceed timeouts.read (%s): the header deadline would never be reached",
			cfg.ReadHeader, cfg.Read,
		)
	}
	return nil
}

func validateDefaultDevPortalConfig(cfg *DefaultDevPortal) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.Name == "" {
		return fmt.Errorf("default DevPortal is enabled but name is not configured")
	}
	if cfg.Identifier == "" {
		return fmt.Errorf("default DevPortal is enabled but identifier is not configured")
	}
	if cfg.APIUrl == "" {
		return fmt.Errorf("default DevPortal is enabled but api_url is not configured")
	}
	if cfg.Hostname == "" {
		return fmt.Errorf("default DevPortal is enabled but hostname is not configured")
	}
	if cfg.APIKey == "" {
		return fmt.Errorf("default DevPortal is enabled but api_key is not configured")
	}
	if cfg.HeaderKeyName == "" {
		return fmt.Errorf("default DevPortal header_key_name is not configured")
	}
	return nil
}

// validateAuthModeExclusivity enforces that IDP (JWKS) auth is not enabled
// alongside the local auth modes. When an IDP is configured every token must be
// validated against its JWKS; leaving local HMAC auth on would let the server
// silently validate (file-based) or accept (local JWT) tokens with the local
// secret instead, shadowing the IDP. So enabling the IDP requires consciously
// turning the local modes off.
func validateAuthModeExclusivity(auth *Auth) error {
	if !auth.IDP.Enabled {
		return nil
	}
	if auth.JWT.Enabled {
		return fmt.Errorf("auth.idp.enabled=true and auth.jwt.enabled=true are mutually exclusive: " +
			"set auth.jwt.enabled=false to delegate authentication to the IDP (tokens are validated against auth.idp.jwks_url)")
	}
	if auth.FileBased.Enabled {
		return fmt.Errorf("auth.idp.enabled=true and auth.file_based.enabled=true are mutually exclusive: " +
			"set auth.file_based.enabled=false to delegate authentication to the IDP (tokens are validated against auth.idp.jwks_url)")
	}
	return nil
}

func validateIDPConfig(idp *IDP) error {
	if !idp.Enabled {
		return nil
	}
	if idp.JWKSUrl == "" {
		return fmt.Errorf("auth.idp.enabled=true requires auth.idp.jwks_url to be configured")
	}
	if len(idp.Issuer) == 0 {
		return fmt.Errorf("auth.idp.enabled=true requires auth.idp.issuer to be configured")
	}
	switch idp.ValidationMode {
	case "scope", "role":
	default:
		return fmt.Errorf("auth.idp.validation_mode must be \"scope\" or \"role\" (got %q)", idp.ValidationMode)
	}
	if idp.ValidationMode == "role" && idp.ClaimMappings.RolesClaimPath == "" {
		return fmt.Errorf("auth.idp.validation_mode=role requires auth.idp.claim_mappings.roles_claim_path to be configured")
	}
	return nil
}

func validateFileBasedConfig(cfg *FileBased) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.Organization.ID == "" {
		return fmt.Errorf("auth.file_based.enabled=true requires auth.file_based.organization.id to be configured")
	}
	if cfg.Organization.DisplayName == "" {
		return fmt.Errorf("auth.file_based.enabled=true requires auth.file_based.organization.display_name to be configured")
	}
	if len(cfg.Users) == 0 {
		return fmt.Errorf("auth.file_based.enabled=true requires at least one user in auth.file_based.users")
	}
	return nil
}

// validateWebhookConfig validates and fills defaults for the webhook receiver config.
// It is a no-op when the webhook is disabled.
func validateWebhookConfig(w *Webhook) error {
	if !w.Enabled {
		return nil
	}
	if w.Secret == "" {
		return fmt.Errorf("webhook.enabled=true requires webhook.secret to be configured")
	}
	if w.SignatureTolerance <= 0 {
		w.SignatureTolerance = 5 * time.Minute
	}
	if w.MaxBodySize <= 0 {
		w.MaxBodySize = 1 << 20 // 1 MiB
	}
	if w.SignatureHeader == "" {
		w.SignatureHeader = "X-Devportal-Signature"
	}
	if w.GatewayType == "" {
		w.GatewayType = "wso2/api-platform"
	}
	return nil
}

func validateEventHubConfig(e *EventHub) error {
	if e.PollInterval <= 0 {
		return fmt.Errorf("event_hub.poll_interval must be a positive duration (got %s)", e.PollInterval)
	}
	if e.CleanupInterval <= 0 {
		return fmt.Errorf("event_hub.cleanup_interval must be a positive duration (got %s)", e.CleanupInterval)
	}
	if e.RetentionPeriod <= 0 {
		return fmt.Errorf("event_hub.retention_period must be a positive duration (got %s)", e.RetentionPeriod)
	}
	return nil
}

func validateDeploymentsConfig(cfg *Deployments) error {
	if !cfg.TimeoutEnabled {
		return nil
	}
	if cfg.TimeoutInterval <= 0 {
		return fmt.Errorf("deployments.timeout_interval must be a positive integer (got %d)", cfg.TimeoutInterval)
	}
	if cfg.TimeoutDuration <= 0 {
		return fmt.Errorf("deployments.timeout_duration must be a positive integer (got %d)", cfg.TimeoutDuration)
	}
	return nil
}
