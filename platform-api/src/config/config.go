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
	"fmt"
	"sync"

	"github.com/kelseyhightower/envconfig"
)

// Server holds the configuration parameters for the application.
type Server struct {
	LogLevel string `envconfig:"LOG_LEVEL" default:"DEBUG"`

	// Server configurations
	Port string `envconfig:"PORT" default:"9243"`

	// Database configurations
	Database     Database `envconfig:"DATABASE"`
	DBSchemaPath string   `envconfig:"DB_SCHEMA_PATH" default:"./internal/database/schema.sql"`

	// OpenAPI spec path — used at startup to build the scope registry.
	OpenAPISpecPath string `envconfig:"OPENAPI_SPEC_PATH" default:"./resources/openapi.yaml"`

	// LLM provider template bootstrap (used to seed defaults into the DB)
	LLMTemplateDefinitionsPath string `envconfig:"LLM_TEMPLATE_DEFINITIONS_PATH" default:"./resources/default-llm-provider-templates"`

	// JWT configurations — used in non-IDP mode (IDP_ENABLED=false)
	JWT JWT `envconfig:"JWT"`

	// WebSocket configurations
	WebSocket WebSocket `envconfig:"WEBSOCKET"`

	// Default DevPortal configurations
	DefaultDevPortal DefaultDevPortal `envconfig:"DEFAULT_DEVPORTAL"`

	// Deployment configurations
	Deployments Deployments `envconfig:"DEPLOYMENTS"`

	// TLS configurations
	TLS TLS `envconfig:"TLS"`

	// API key configurations
	APIKey APIKey `envconfig:"API_KEY"`

	// IDP configurations — controls authentication mode and claim extraction.
	// All identity providers (Thunder, Keycloak, Asgardeo, Azure AD, Okta, …)
	// share the same config surface; set IDP_ENABLED=true to activate.
	IDP IDP `envconfig:"IDP"`

	// Gateway configurations
	Gateway Gateway `envconfig:"GATEWAY"`

	// RBAC configurations
	RBAC RBAC `envconfig:"RBAC"`
}

// IDP holds configuration for JWT-based identity providers.
// The same fields apply regardless of which IDP is in use (Thunder, Keycloak,
// Asgardeo, Azure AD, Okta, etc.).
//
// When IDP_ENABLED=false (default), the server validates tokens with a local
// HMAC secret (JWT_SECRET_KEY) or skips validation entirely in dev mode.
// When IDP_ENABLED=true, JWKS-based validation is performed against IDP_JWKS_URL.
type IDP struct {
	// Enabled controls whether JWKS-based JWT validation is active.
	// When false (default), the server uses HMAC validation (JWT_SECRET_KEY) or
	// skips validation when JWT_SKIP_VALIDATION=true (local development only).
	// Env: IDP_ENABLED (default: false)
	Enabled bool `envconfig:"ENABLED" default:"false"`

	// Type is an optional label describing which IDP is configured (e.g. "thunder",
	// "keycloak", "asgardeo"). It does not change runtime behavior — all IDPs use the
	// same config fields — but it appears in startup log messages.
	// Env: IDP_TYPE (default: "")
	Type string `envconfig:"TYPE" default:""`

	// JWKSUrl is the IDP's JWKS endpoint for fetching public signing keys.
	// Required when IDP_ENABLED=true.
	// Env: IDP_JWKS_URL
	JWKSUrl string `envconfig:"JWKS_URL" default:""`

	// Issuer is the list of accepted JWT issuers (comma-separated).
	// Required when IDP_ENABLED=true.
	// Example: "https://accounts.example.com,https://sso.example.com"
	// Env: IDP_ISSUER
	Issuer []string `envconfig:"ISSUER"`

	// Audience is the list of accepted JWT audiences (comma-separated).
	// Optional. Entries ending with "*" are treated as prefix matches.
	// Env: IDP_AUDIENCE
	Audience []string `envconfig:"AUDIENCE"`

	// --- Claim name mappings ---
	// Set these when your IDP uses non-standard claim names.

	// OrganizationClaimName is the JWT claim that holds the organization/tenant UUID.
	// Every protected request must carry this claim; requests without it are rejected.
	// Env: IDP_ORGANIZATION_CLAIM_NAME (default: "organization")
	OrganizationClaimName string `envconfig:"ORGANIZATION_CLAIM_NAME" default:"organization"`

	// UserIDClaimName is the JWT claim used as the canonical user identifier.
	// Env: IDP_USER_ID_CLAIM_NAME (default: "sub")
	UserIDClaimName string `envconfig:"USER_ID_CLAIM_NAME" default:"sub"`

	// UsernameClaimName is the JWT claim for the human-readable username.
	// Env: IDP_USERNAME_CLAIM_NAME (default: "username")
	UsernameClaimName string `envconfig:"USERNAME_CLAIM_NAME" default:"username"`

	// EmailClaimName is the JWT claim for the user's email address.
	// Env: IDP_EMAIL_CLAIM_NAME (default: "email")
	EmailClaimName string `envconfig:"EMAIL_CLAIM_NAME" default:"email"`

	// ScopeClaimName is the JWT claim that carries the granted OAuth2 scopes.
	// When this claim is present in the token, scope-based validation is used directly.
	// When absent, role-based expansion applies (see RolesClaimPath).
	// Env: IDP_SCOPE_CLAIM_NAME (default: "scope")
	ScopeClaimName string `envconfig:"SCOPE_CLAIM_NAME" default:"scope"`

	// --- Role-based access (for IDPs that issue roles instead of scopes) ---

	// RolesClaimPath is the dot-notation path to the claim containing the user's roles.
	// Supports both flat claims ("roles") and nested claims ("realm_access.roles").
	// The claim value can be a string array or a space-separated string.
	// When empty, role-based expansion is disabled and only scope-based validation applies.
	// Env: IDP_ROLES_CLAIM_PATH (default: "")
	RolesClaimPath string `envconfig:"ROLES_CLAIM_PATH" default:""`

	// RoleMappings maps IDP role values to platform roles (admin, developer, viewer).
	// Format: comma-separated "idp-role=platform-role" pairs.
	// Example: "PLATFORM_ADMIN=admin,PLATFORM_DEV=developer,PLATFORM_VIEWER=viewer"
	// When empty, IDP role values are used as platform role names directly
	// (only works if the IDP already issues "admin", "developer", or "viewer").
	// Only relevant when IDP_VALIDATION_MODE=role.
	// Env: IDP_ROLE_MAPPINGS
	RoleMappings []string `envconfig:"ROLE_MAPPINGS"`

	// ValidationMode selects how authorization is enforced. Pick one:
	//   "scope" (default) — validate using the JWT scope claim directly.
	//                       The IDP must issue fine-grained platform scopes.
	//   "role"            — validate by expanding IDP roles to platform roles
	//                       and treating the full role permission set as the
	//                       caller's effective scopes. Requires RolesClaimPath
	//                       and optionally RoleMappings to be configured.
	// These modes are mutually exclusive; there is no fallback between them.
	// Env: IDP_VALIDATION_MODE (default: "scope")
	ValidationMode string `envconfig:"VALIDATION_MODE" default:"scope"`
}

// RBAC holds role-based access control configuration.
type RBAC struct {
	// Enabled controls whether scope checks are enforced on protected routes.
	// When false, all authenticated requests are allowed regardless of scope — useful
	// for local development or initial deployment before scopes are configured.
	// Env: RBAC_ENABLED (default: true)
	Enabled bool `envconfig:"ENABLED" default:"true"`
}

// Gateway holds gateway-related configuration.
type Gateway struct {
	// EnableVersionVerification controls whether the platform API rejects gateway
	// connections whose reported version does not match the registered version.
	// When false (default), a mismatch is logged and the connection is allowed to proceed.
	// Env: GATEWAY_ENABLE_VERSION_VERIFICATION (default: false)
	EnableVersionVerification bool `envconfig:"ENABLE_VERSION_VERIFICATION" default:"false"`

	// EnableFunctionalityTypeVerification controls whether the platform API rejects
	// gateway connections whose reported functionality type is incompatible with the
	// registered type. When false (default), a mismatch is logged and the connection
	// is allowed to proceed.
	// Env: GATEWAY_ENABLE_FUNCTIONALITY_TYPE_VERIFICATION (default: false)
	EnableFunctionalityTypeVerification bool `envconfig:"ENABLE_FUNCTIONALITY_TYPE_VERIFICATION" default:"false"`
}

// TLS holds TLS certificate configuration
type TLS struct {
	CertDir string `envconfig:"CERT_DIR" default:"./data/certs"`
}

// JWT holds configuration for the non-IDP authentication mode (IDP_ENABLED=false).
// When IDP_ENABLED=true, JWT signature validation is handled by JWKS (see IDP config).
type JWT struct {
	// SecretKey is the HMAC signing key used to verify token signatures when
	// IDP_ENABLED=false and JWT_SKIP_VALIDATION=false.
	// Env: JWT_SECRET_KEY (default: "your-secret-key-change-in-production")
	SecretKey string `envconfig:"SECRET_KEY" default:"your-secret-key-change-in-production"`

	// Issuer is the expected JWT issuer value for HMAC-signed tokens.
	// When empty, issuer validation is skipped.
	// Env: JWT_ISSUER (default: "")
	Issuer string `envconfig:"ISSUER" default:""`

	// SkipPaths is the list of path prefixes that bypass JWT authentication entirely.
	// Env: JWT_SKIP_PATHS
	SkipPaths []string `envconfig:"SKIP_PATHS" default:"/health,/metrics,/api/internal/v1/ws/gateways/connect,/api/internal/v1/apis,/api/internal/v1/llm-providers,/api/internal/v1/llm-proxies,/api/internal/v1/subscription-plans,/api/internal/v1/mcp-proxies,/api/internal/v1/gateways,/api/internal/v1/deployments,/api/internal/v1/artifacts,/api/internal/v1/websub-apis,/api/internal/v1/webbroker-apis"`

	// SkipValidation disables JWT signature verification.
	// Only applies when IDP_ENABLED=false. Use only for local development.
	// Env: JWT_SKIP_VALIDATION (default: true)
	SkipValidation bool `envconfig:"SKIP_VALIDATION" default:"true"`
}

// WebSocket holds WebSocket-specific configuration
type WebSocket struct {
	MaxConnections       int  `envconfig:"WS_MAX_CONNECTIONS" default:"1000"`
	ConnectionTimeout    int  `envconfig:"WS_CONNECTION_TIMEOUT" default:"30"` // seconds
	RateLimitPerMin      int  `envconfig:"WS_RATE_LIMIT_PER_MINUTE" default:"1000"`
	MaxConnectionsPerOrg int  `envconfig:"WS_MAX_CONNECTIONS_PER_ORG" default:"3"`
	MetricsLogEnabled    bool `envconfig:"WS_METRICS_LOG_ENABLED" default:"true"`
	MetricsLogInterval   int  `envconfig:"WS_METRICS_LOG_INTERVAL" default:"10"` // seconds
}

// Database holds database-specific configuration
type Database struct {
	Driver string `envconfig:"DRIVER" default:"sqlite3"`
	// DBPath is the file path for SQLite databases.
	// Use DATABASE_DB_PATH to override; keeping it distinct from the OS PATH variable.
	Path            string `envconfig:"DB_PATH" default:"./data/api_platform.db"`
	Host            string `envconfig:"HOST" default:"localhost"`
	Port            int    `envconfig:"PORT" default:"5432"`
	Name            string `envconfig:"NAME" default:"platform_api"`
	User            string `envconfig:"USER" default:""`
	Password        string `envconfig:"PASSWORD" default:""`
	SSLMode         string `envconfig:"SSL_MODE" default:"disable"`
	MaxOpenConns    int    `envconfig:"MAX_OPEN_CONNS" default:"25"`
	MaxIdleConns    int    `envconfig:"MAX_IDLE_CONNS" default:"10"`
	ConnMaxLifetime int    `envconfig:"CONN_MAX_LIFETIME" default:"300"` // seconds

	// ExecuteSchemaDDL controls whether to run the schema DDL (CREATE TABLE, etc.) on startup.
	// Set to false when the DB user lacks DDL privileges (e.g. deployed Postgres with restricted role).
	// Env: DATABASE_EXECUTE_SCHEMA_DDL (default: true)
	ExecuteSchemaDDL bool `envconfig:"EXECUTE_SCHEMA_DDL" default:"true"`

	// SubscriptionTokenEncryptionKey is the 32-byte key for AES-256-GCM encryption of subscription tokens.
	// Provide as 64 hex chars or 44 base64 chars. Required for storing tokens in encrypted form (retrievable on GET).
	// Env: DATABASE_SUBSCRIPTION_TOKEN_ENCRYPTION_KEY. If empty, falls back to JWT_SECRET_KEY.
	SubscriptionTokenEncryptionKey string `envconfig:"SUBSCRIPTION_TOKEN_ENCRYPTION_KEY" default:""`
}

// DefaultDevPortal holds default DevPortal configuration for new organizations
type DefaultDevPortal struct {
	Enabled       bool   `envconfig:"ENABLED" default:"true"`
	Name          string `envconfig:"NAME" default:"Default DevPortal"`
	Identifier    string `envconfig:"IDENTIFIER" default:"default"`
	APIUrl        string `envconfig:"API_URL" default:"http://localhost:3001"`
	Hostname      string `envconfig:"HOSTNAME" default:"devportal.local"`
	APIKey        string `envconfig:"API_KEY" default:"default-api-key"`
	HeaderKeyName string `envconfig:"HEADER_KEY_NAME" default:"x-wso2-api-key"`
	Timeout       int    `envconfig:"TIMEOUT" default:"10"` // seconds

	// Role mapping configuration for DevPortal integrations
	RoleClaimName         string `envconfig:"ROLE_CLAIM_NAME" default:"roles"`
	GroupsClaimName       string `envconfig:"GROUPS_CLAIM_NAME" default:"groups"`
	OrganizationClaimName string `envconfig:"ORGANIZATION_CLAIM_NAME" default:"organizationID"`
	AdminRole             string `envconfig:"ADMIN_ROLE" default:"admin"`
	SubscriberRole        string `envconfig:"SUBSCRIBER_ROLE" default:"Internal/subscriber"`
	SuperAdminRole        string `envconfig:"SUPER_ADMIN_ROLE" default:"superAdmin"`
}

// Deployments holds deployment-specific configuration
type Deployments struct {
	MaxPerAPIGateway int `envconfig:"MAX_PER_API_GATEWAY" default:"20"`

	// TransitionalStatusEnabled controls whether deploy/undeploy sets DEPLOYING/UNDEPLOYING
	// before waiting for a gateway ack. When false (default), status is set immediately to
	// DEPLOYED/UNDEPLOYED without waiting for acknowledgement.
	TransitionalStatusEnabled bool `envconfig:"TRANSITIONAL_STATUS_ENABLED" default:"false"`

	// Timeout job settings for transitional deployment statuses (DEPLOYING/UNDEPLOYING).
	// Only relevant when TransitionalStatusEnabled is true.
	TimeoutEnabled  bool `envconfig:"TIMEOUT_ENABLED" default:"true"`
	TimeoutInterval int  `envconfig:"TIMEOUT_INTERVAL" default:"20"` // seconds between checks
	TimeoutDuration int  `envconfig:"TIMEOUT_DURATION" default:"60"` // seconds before a status is considered stale
}

// APIKey holds API key-specific configuration
type APIKey struct {
	// HashingAlgorithms is the list of algorithms used to hash API keys before storage and broadcast.
	// Multiple algorithms can be specified as a comma-separated value (e.g. "sha256,sha512").
	// Currently only "sha256" is supported. Defaults to "sha256".
	HashingAlgorithms []string `envconfig:"HASHING_ALGORITHMS" default:"sha256"`
}

// package-level variable and mutex for thread safety
var (
	processOnce     sync.Once
	settingInstance *Server
)

// GetConfig initializes and returns a singleton instance of the Settings struct.
func GetConfig() *Server {
	var err error
	processOnce.Do(func() {
		settingInstance = &Server{}
		err = envconfig.Process("", settingInstance)
		if err == nil {
			err = validateDefaultDevPortalConfig(&settingInstance.DefaultDevPortal)
		}
		if err == nil {
			err = validateDeploymentsConfig(&settingInstance.Deployments)
		}
		if err == nil {
			err = validateIDPConfig(&settingInstance.IDP)
		}
	})
	if err != nil {
		panic(err)
	}
	return settingInstance
}

func validateDefaultDevPortalConfig(cfg *DefaultDevPortal) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.Name == "" {
		return fmt.Errorf("default DevPortal is enabled but DEFAULT_DEVPORTAL_NAME is not configured")
	}
	if cfg.Identifier == "" {
		return fmt.Errorf("default DevPortal is enabled but DEFAULT_DEVPORTAL_IDENTIFIER is not configured")
	}
	if cfg.APIUrl == "" {
		return fmt.Errorf("default DevPortal is enabled but DEFAULT_DEVPORTAL_API_URL is not configured")
	}
	if cfg.Hostname == "" {
		return fmt.Errorf("default DevPortal is enabled but DEFAULT_DEVPORTAL_HOSTNAME is not configured")
	}
	if cfg.APIKey == "" {
		return fmt.Errorf("default DevPortal is enabled but DEFAULT_DEVPORTAL_API_KEY is not configured")
	}
	if cfg.HeaderKeyName == "" {
		return fmt.Errorf("default DevPortal header key name is not configured")
	}
	return nil
}

// validateIDPConfig validates IDP configuration when enabled.
func validateIDPConfig(idp *IDP) error {
	if !idp.Enabled {
		return nil
	}
	if idp.JWKSUrl == "" {
		return fmt.Errorf("IDP_ENABLED=true requires IDP_JWKS_URL to be configured")
	}
	if len(idp.Issuer) == 0 {
		return fmt.Errorf("IDP_ENABLED=true requires IDP_ISSUER to be configured")
	}
	switch idp.ValidationMode {
	case "scope", "role":
		// valid
	default:
		return fmt.Errorf("IDP_VALIDATION_MODE must be \"scope\" or \"role\" (got %q)", idp.ValidationMode)
	}
	if idp.ValidationMode == "role" && idp.RolesClaimPath == "" {
		return fmt.Errorf("IDP_VALIDATION_MODE=role requires IDP_ROLES_CLAIM_PATH to be configured")
	}
	return nil
}

// validateDeploymentsConfig validates deployment timeout configuration.
func validateDeploymentsConfig(cfg *Deployments) error {
	if !cfg.TimeoutEnabled {
		return nil
	}
	if cfg.TimeoutInterval <= 0 {
		return fmt.Errorf("DEPLOYMENTS_TIMEOUT_INTERVAL must be a positive integer (got %d)", cfg.TimeoutInterval)
	}
	if cfg.TimeoutDuration <= 0 {
		return fmt.Errorf("DEPLOYMENTS_TIMEOUT_DURATION must be a positive integer (got %d)", cfg.TimeoutDuration)
	}
	return nil
}
