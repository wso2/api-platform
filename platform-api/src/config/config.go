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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/go-viper/mapstructure/v2"
	toml "github.com/knadh/koanf/parsers/toml/v2"
	kenv "github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// BasicAuthUser represents a built-in user for basic-auth mode.
type BasicAuthUser struct {
	Username     string `json:"username"     koanf:"username"`
	PasswordHash string `json:"password_hash" koanf:"password_hash"`
	Scopes       string `json:"scopes"       koanf:"scopes"`
}

// BasicAuthUsers is a slice of BasicAuthUser that can be decoded from a JSON string (env var)
// or from a TOML array of tables ([[auth.basic_auth.users]]).
type BasicAuthUsers []BasicAuthUser

// BasicAuthOrg holds the single organization used in basic-auth mode.
type BasicAuthOrg struct {
	// ID is the org UUID. Auto-generated at startup if empty.
	ID string `koanf:"id"`

	// Name is the display name of the organization.
	Name string `koanf:"name"`

	// Handle is the URL-safe slug for the organization.
	Handle string `koanf:"handle"`

	// Region is the deployment region for the organization.
	Region string `koanf:"region"`
}

// BasicAuth holds configuration for local username/password authentication.
type BasicAuth struct {
	Enabled      bool          `koanf:"enabled"`
	Organization BasicAuthOrg  `koanf:"organization"`
	Users        BasicAuthUsers `koanf:"users"`
}

// Server holds the configuration parameters for the application.
type Server struct {
	LogLevel string `koanf:"log_level"`
	DevMode  bool   `koanf:"dev_mode"`
	Port     string `koanf:"port"`

	DBSchemaPath               string `koanf:"db_schema_path"`
	OpenAPISpecPath            string `koanf:"openapi_spec_path"`
	LLMTemplateDefinitionsPath string `koanf:"llm_template_definitions_path"`

	Database         Database         `koanf:"database"`
	Auth             Auth             `koanf:"auth"`
	WebSocket        WebSocket        `koanf:"websocket"`
	DefaultDevPortal DefaultDevPortal `koanf:"default_devportal"`
	Deployments      Deployments      `koanf:"deployments"`
	TLS              TLS              `koanf:"tls"`
	APIKey           APIKey           `koanf:"api_key"`
	Gateway          Gateway          `koanf:"gateway"`

	EnableScopeValidation bool `koanf:"enable_scope_validation"`
}

// Auth groups all authentication-related configuration.
type Auth struct {
	SkipPaths []string  `koanf:"skip_paths"`
	IDP       IDP       `koanf:"idp"`
	JWT       JWT       `koanf:"jwt"`
	BasicAuth BasicAuth `koanf:"basic_auth"`
}

// IDP holds configuration for JWKS-based identity providers.
type IDP struct {
	Enabled bool   `koanf:"enabled"`
	Name    string `koanf:"name"`
	JWKSUrl string `koanf:"jwks_url"`
	Issuer  []string `koanf:"issuer"`
	Audience []string `koanf:"audience"`

	OrganizationClaimName string `koanf:"organization_claim_name"`
	OrgNameClaimName      string `koanf:"org_name_claim_name"`
	OrgHandleClaimName    string `koanf:"org_handle_claim_name"`
	UserIDClaimName       string `koanf:"user_id_claim_name"`
	UsernameClaimName     string `koanf:"username_claim_name"`
	EmailClaimName        string `koanf:"email_claim_name"`
	ScopeClaimName        string `koanf:"scope_claim_name"`
	RolesClaimPath   string `koanf:"roles_claim_path"`
	RoleMappingsFile string `koanf:"role_mappings_file"`
	ValidationMode   string `koanf:"validation_mode"`
}

// Gateway holds gateway-related configuration.
type Gateway struct {
	EnableVersionVerification           bool `koanf:"enable_version_verification"`
	EnableFunctionalityTypeVerification bool `koanf:"enable_functionality_type_verification"`
}

// TLS holds TLS certificate configuration.
type TLS struct {
	CertDir string `koanf:"cert_dir"`
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

	ExecuteSchemaDDL               bool   `koanf:"execute_schema_ddl"`
	SubscriptionTokenEncryptionKey string `koanf:"subscription_token_encryption_key"`
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

// LoadConfig loads configuration with priority: env vars > config file > defaults.
// configPath may be empty — when omitted only env vars and defaults are used.
func LoadConfig(configPath string) (*Server, error) {
	applyLegacyEnvAliases()

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

	if err := k.UnmarshalWithConf("", cfg, koanf.UnmarshalConf{
		DecoderConfig: &mapstructure.DecoderConfig{
			TagName:          "koanf",
			WeaklyTypedInput: true,
			Result:           cfg,
			DecodeHook: mapstructure.ComposeDecodeHookFunc(
				mapstructure.StringToSliceHookFunc(","),
				mapstructure.StringToTimeDurationHookFunc(),
				basicAuthUsersDecodeHook(),
			),
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := validateDefaultDevPortalConfig(&cfg.DefaultDevPortal); err != nil {
		return nil, err
	}
	if err := validateDeploymentsConfig(&cfg.Deployments); err != nil {
		return nil, err
	}
	if err := validateIDPConfig(&cfg.Auth.IDP); err != nil {
		return nil, err
	}
	if err := validateBasicAuthConfig(&cfg.Auth.BasicAuth); err != nil {
		return nil, err
	}

	return cfg, nil
}

// defaultConfig returns a Server with all default values.
func defaultConfig() *Server {
	return &Server{
		LogLevel:                   "DEBUG",
		DevMode:                    false,
		Port:                       "9243",
		DBSchemaPath:               "./internal/database/schema.sql",
		OpenAPISpecPath:            "./resources/openapi.yaml",
		LLMTemplateDefinitionsPath: "./resources/default-llm-provider-templates",
		EnableScopeValidation:      true,
		Database: Database{
			Driver:           "sqlite3",
			Path:             "./data/api_platform.db",
			Host:             "localhost",
			Port:             5432,
			Name:             "platform_api",
			SSLMode:          "disable",
			MaxOpenConns:     25,
			MaxIdleConns:     10,
			ConnMaxLifetime:  300,
			ExecuteSchemaDDL: true,
		},
		Auth: Auth{
			SkipPaths: []string{
				"/health",
				"/metrics",
				"/api/internal/v1/ws/gateways/connect",
				"/api/internal/v1/apis",
				"/api/internal/v1/llm-providers",
				"/api/internal/v1/llm-proxies",
				"/api/internal/v1/subscription-plans",
				"/api/internal/v1/mcp-proxies",
				"/api/internal/v1/gateways",
				"/api/internal/v1/deployments",
				"/api/internal/v1/artifacts",
				"/api/internal/v1/websub-apis",
				"/api/internal/v1/webbroker-apis",
			},
			JWT: JWT{
				Enabled:   true,
				SecretKey: "your-secret-key-change-in-production",
				Issuer:    "platform-api",
			},
			IDP: IDP{
				OrganizationClaimName: "organization",
				OrgNameClaimName:      "org_name",
				OrgHandleClaimName:    "org_handle",
				UserIDClaimName:       "sub",
				UsernameClaimName:     "username",
				EmailClaimName:        "email",
				ScopeClaimName:        "scope",
				ValidationMode:        "scope",
			},
			BasicAuth: BasicAuth{
				Organization: BasicAuthOrg{Region: "us"},
			},
		},
		WebSocket: WebSocket{
			MaxConnections:       1000,
			ConnectionTimeout:    30,
			RateLimitPerMin:      1000,
			MaxConnectionsPerOrg: 3,
			MetricsLogEnabled:    true,
			MetricsLogInterval:   10,
		},
		DefaultDevPortal: DefaultDevPortal{
			Enabled:               true,
			Name:                  "Default DevPortal",
			Identifier:            "default",
			APIUrl:                "http://localhost:3001",
			Hostname:              "devportal.local",
			APIKey:                "default-api-key",
			HeaderKeyName:         "x-wso2-api-key",
			Timeout:               10,
			RoleClaimName:         "roles",
			GroupsClaimName:       "groups",
			OrganizationClaimName: "organizationID",
			AdminRole:             "admin",
			SubscriberRole:        "Internal/subscriber",
			SuperAdminRole:        "superAdmin",
		},
		Deployments: Deployments{
			MaxPerAPIGateway: 20,
			TimeoutEnabled:   true,
			TimeoutInterval:  20,
			TimeoutDuration:  60,
		},
		TLS: TLS{
			CertDir: "./data/certs",
		},
		APIKey: APIKey{
			HashingAlgorithms: []string{"sha256"},
		},
	}
}

// envToKoanfKey maps a lowercased environment variable name to its koanf dot-notation key.
// Returns "" for unknown variables, which causes koanf to skip them.
// Supports both the current env var names (e.g. DATABASE_DB_PATH) and the legacy
// WEBSOCKET_WS_* naming from the old envconfig setup.
func envToKoanfKey(s string) string {
	switch s {
	// Server-level
	case "log_level":                     return "log_level"
	case "dev_mode":                      return "dev_mode"
	case "port":                          return "port"
	case "db_schema_path":                return "db_schema_path"
	case "openapi_spec_path":             return "openapi_spec_path"
	case "llm_template_definitions_path": return "llm_template_definitions_path"
	case "enable_scope_validation":       return "enable_scope_validation"

	// Database
	case "database_driver":              return "database.driver"
	case "database_db_path":             return "database.path"
	case "database_host":                return "database.host"
	case "database_port":                return "database.port"
	case "database_name":                return "database.name"
	case "database_user":                return "database.user"
	case "database_password":            return "database.password"
	case "database_ssl_mode":            return "database.ssl_mode"
	case "database_max_open_conns":      return "database.max_open_conns"
	case "database_max_idle_conns":      return "database.max_idle_conns"
	case "database_conn_max_lifetime":   return "database.conn_max_lifetime"
	case "database_execute_schema_ddl":  return "database.execute_schema_ddl"
	case "database_subscription_token_encryption_key": return "database.subscription_token_encryption_key"

	// Auth
	case "auth_skip_paths": return "auth.skip_paths"

	// Auth JWT
	case "auth_jwt_enabled":         return "auth.jwt.enabled"
	case "auth_jwt_secret_key":      return "auth.jwt.secret_key"
	case "auth_jwt_issuer":          return "auth.jwt.issuer"
	case "auth_jwt_skip_validation": return "auth.jwt.skip_validation"

	// Auth IDP
	case "auth_idp_enabled":                  return "auth.idp.enabled"
	case "auth_idp_name":                     return "auth.idp.name"
	case "auth_idp_jwks_url":                 return "auth.idp.jwks_url"
	case "auth_idp_issuer":                   return "auth.idp.issuer"
	case "auth_idp_audience":                 return "auth.idp.audience"
	case "auth_idp_organization_claim_name":  return "auth.idp.organization_claim_name"
	case "auth_idp_org_name_claim_name":      return "auth.idp.org_name_claim_name"
	case "auth_idp_org_handle_claim_name":    return "auth.idp.org_handle_claim_name"
	case "auth_idp_user_id_claim_name":       return "auth.idp.user_id_claim_name"
	case "auth_idp_username_claim_name":      return "auth.idp.username_claim_name"
	case "auth_idp_email_claim_name":         return "auth.idp.email_claim_name"
	case "auth_idp_scope_claim_name":         return "auth.idp.scope_claim_name"
	case "auth_idp_roles_claim_path":         return "auth.idp.roles_claim_path"
	case "auth_idp_role_mappings_file":       return "auth.idp.role_mappings_file"
	case "auth_idp_validation_mode":          return "auth.idp.validation_mode"

	// Auth BasicAuth
	case "auth_basic_auth_enabled":              return "auth.basic_auth.enabled"
	case "auth_basic_auth_organization_id":      return "auth.basic_auth.organization.id"
	case "auth_basic_auth_organization_name":    return "auth.basic_auth.organization.name"
	case "auth_basic_auth_organization_handle":  return "auth.basic_auth.organization.handle"
	case "auth_basic_auth_organization_region":  return "auth.basic_auth.organization.region"
	case "auth_basic_auth_users":                return "auth.basic_auth.users"

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
	case "default_devportal_enabled":                  return "default_devportal.enabled"
	case "default_devportal_name":                     return "default_devportal.name"
	case "default_devportal_identifier":               return "default_devportal.identifier"
	case "default_devportal_api_url":                  return "default_devportal.api_url"
	case "default_devportal_hostname":                 return "default_devportal.hostname"
	case "default_devportal_api_key":                  return "default_devportal.api_key"
	case "default_devportal_header_key_name":          return "default_devportal.header_key_name"
	case "default_devportal_timeout":                  return "default_devportal.timeout"
	case "default_devportal_role_claim_name":          return "default_devportal.role_claim_name"
	case "default_devportal_groups_claim_name":        return "default_devportal.groups_claim_name"
	case "default_devportal_organization_claim_name":  return "default_devportal.organization_claim_name"
	case "default_devportal_admin_role":               return "default_devportal.admin_role"
	case "default_devportal_subscriber_role":          return "default_devportal.subscriber_role"
	case "default_devportal_super_admin_role":         return "default_devportal.super_admin_role"

	// Deployments
	case "deployments_max_per_api_gateway":          return "deployments.max_per_api_gateway"
	case "deployments_transitional_status_enabled":  return "deployments.transitional_status_enabled"
	case "deployments_timeout_enabled":              return "deployments.timeout_enabled"
	case "deployments_timeout_interval":             return "deployments.timeout_interval"
	case "deployments_timeout_duration":             return "deployments.timeout_duration"

	// TLS
	case "tls_cert_dir": return "tls.cert_dir"

	// API Key
	case "api_key_hashing_algorithms": return "api_key.hashing_algorithms"

	// Gateway
	case "gateway_enable_version_verification":            return "gateway.enable_version_verification"
	case "gateway_enable_functionality_type_verification": return "gateway.enable_functionality_type_verification"

	default:
		return ""
	}
}

// basicAuthUsersDecodeHook handles decoding AUTH_BASIC_AUTH_USERS from a JSON string
// (env var format) in addition to the native TOML array-of-tables format.
func basicAuthUsersDecodeHook() mapstructure.DecodeHookFuncType {
	return func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
		if t != reflect.TypeOf(BasicAuthUsers{}) {
			return data, nil
		}
		s, ok := data.(string)
		if !ok {
			return data, nil
		}
		if s == "" {
			return BasicAuthUsers{}, nil
		}
		var users BasicAuthUsers
		if err := json.Unmarshal([]byte(s), &users); err != nil {
			return nil, fmt.Errorf("failed to parse AUTH_BASIC_AUTH_USERS as JSON: %w", err)
		}
		return users, nil
	}
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
	if idp.ValidationMode == "role" && idp.RolesClaimPath == "" {
		return fmt.Errorf("auth.idp.validation_mode=role requires auth.idp.roles_claim_path to be configured")
	}
	return nil
}

func validateBasicAuthConfig(cfg *BasicAuth) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.Organization.Name == "" {
		return fmt.Errorf("auth.basic_auth.enabled=true requires auth.basic_auth.organization.name to be configured")
	}
	if cfg.Organization.Handle == "" {
		return fmt.Errorf("auth.basic_auth.enabled=true requires auth.basic_auth.organization.handle to be configured")
	}
	if len(cfg.Users) == 0 {
		return fmt.Errorf("auth.basic_auth.enabled=true requires at least one user in auth.basic_auth.users")
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
