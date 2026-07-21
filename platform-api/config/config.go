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
// or from a TOML array of tables ([[auth.file.users]]).
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
// Active when Auth.Mode is AuthModeFile.
type FileBased struct {
	Organization FileBasedOrg   `koanf:"organization"`
	Users        FileBasedUsers `koanf:"users"`
}

// Logging holds logging configuration.
type Logging struct {
	Level  string `koanf:"level"`
	Format string `koanf:"format"`
}

// Server holds the configuration parameters for the application.
type Server struct {
	Logging Logging `koanf:"logging"`

	DBSchemaPath               string `koanf:"db_schema_path"`
	OpenAPISpecPath            string `koanf:"openapi_spec_path"`
	LLMTemplateDefinitionsPath string `koanf:"llm_template_definitions_path"`
	OpenAPISpecMaxFetchBytes   int64  `koanf:"openapi_spec_max_fetch_bytes"`

	Database    Database        `koanf:"database"`
	Auth        Auth            `koanf:"auth"`
	Deployments Deployments     `koanf:"deployments"`
	Listeners   ServerListeners `koanf:"server"`
	Security    Security        `koanf:"security"`
	Gateway     Gateway         `koanf:"gateway"`
	EventHub    EventHub        `koanf:"event_hub"`
	Webhook     Webhook         `koanf:"webhook"`
}

// Authentication modes selectable via auth.mode. Exactly one mode is active;
// modeling the choice as a single discriminator (rather than per-mode enabled
// flags) makes conflicting configurations inexpressible.
const (
	// AuthModeExternalToken verifies locally-signed HMAC JWTs (auth.jwt.secret_key)
	// that are minted externally, e.g. by the Developer Portal using the shared secret.
	AuthModeExternalToken = "external_token"
	// AuthModeFile is AuthModeExternalToken plus local username/password login: the
	// login endpoint authenticates users from auth.file and issues HMAC JWTs signed
	// with auth.jwt.secret_key.
	AuthModeFile = "file"
	// AuthModeIDP validates tokens against an external IDP's JWKS (auth.idp).
	AuthModeIDP = "idp"
)

// Auth groups all authentication-related configuration.
type Auth struct {
	// Mode selects the active authentication mode: "external_token", "file", or "idp".
	Mode string `koanf:"mode"`
	// ScopeValidation enforces per-endpoint OAuth2 scopes on validated tokens.
	// Disable only to temporarily bypass authorization during development.
	ScopeValidation bool     `koanf:"scope_validation"`
	SkipPaths       []string `koanf:"skip_paths"`
	IDP             IDP      `koanf:"idp"`
	// JWT is shared by two modes — "external_token" mode only verifies
	// externally-minted tokens with it, "file" mode both signs and verifies
	// with it.
	JWT  JWT       `koanf:"jwt"`
	File FileBased `koanf:"file"`
	// ClaimMappings names the JWT claims that carry each identity field. It is
	// shared by all three auth modes: "idp" reads incoming claims by these
	// names, "file" mode's login endpoint signs tokens using these names, and
	// "external_token" mode reads externally-minted tokens by these names too
	// — one mapping, so issuance and validation can never drift apart. Every
	// field accepts either a flat top-level claim name ("org_id") or a
	// dot-separated path into a nested claim ("realm_access.org_id") — see
	// resolveClaimPath in internal/middleware/auth.go.
	ClaimMappings ClaimMappings `koanf:"claim_mappings"`
}

// ClaimMappings holds JWT claim name mappings, shared across all auth modes.
type ClaimMappings struct {
	Organization string `koanf:"organization"`
	OrgName      string `koanf:"org_name"`
	OrgHandle    string `koanf:"org_handle"`
	UserID       string `koanf:"user_id"`
	Username     string `koanf:"username"`
	Email        string `koanf:"email"`
	Scope        string `koanf:"scope"`
	Roles        string `koanf:"roles"`
}

// IDP holds configuration for JWKS-based identity providers. Active when
// Auth.Mode is AuthModeIDP.
type IDP struct {
	Name           string   `koanf:"name"`
	JWKSUrl        string   `koanf:"jwks_url"`
	Issuer         []string `koanf:"issuer"`
	Audience       []string `koanf:"audience"`
	ValidationMode string   `koanf:"validation_mode"`
	RoleMappings   string   `koanf:"role_mappings"`
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

// ServerListeners models the [server] section: the two independent HTTP
// listeners (each enabled independently and bound to its own port, so a
// deployment can serve plain HTTP internally, HTTPS externally, or both at
// once to migrate clients between them without downtime), plus the
// cross-cutting settings — timeouts, CORS, WebSocket — that apply to
// whichever listener(s) are serving requests.
type ServerListeners struct {
	HTTP      HTTPListener  `koanf:"http"`
	HTTPS     HTTPSListener `koanf:"https"`
	Timeouts  Timeouts      `koanf:"timeouts"`
	CORS      CORS          `koanf:"cors"`
	WebSocket WebSocket     `koanf:"websocket"`
}

// HTTPListener configures the plain-HTTP listener. Enable it only when a trusted
// upstream (ingress, service-mesh sidecar) terminates TLS, or for internal
// cluster traffic; never expose it directly to untrusted networks.
type HTTPListener struct {
	Enabled bool `koanf:"enabled"`
	Port    int  `koanf:"port"`
}

// HTTPSListener configures the TLS listener. CertFile and KeyFile must point at a
// certificate pair when Enabled is true; there is no self-signed fallback.
type HTTPSListener struct {
	Enabled  bool   `koanf:"enabled"`
	Port     int    `koanf:"port"`
	CertFile string `koanf:"cert_file"`
	KeyFile  string `koanf:"key_file"`
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
	// cross-origin requests. Must never be ["*"] — wildcard
	// origins cannot be combined with credentialed requests.
	AllowedOrigins []string `koanf:"allowed_origins"`
}

// JWT holds configuration for local HMAC JWT authentication. Active when
// Auth.Mode is AuthModeExternalToken (verify-only, externally-minted tokens)
// or AuthModeFile (file mode also issues these tokens). Signature validation
// is always on.
type JWT struct {
	SecretKey string        `koanf:"secret_key"`
	Issuer    string        `koanf:"issuer"`
	TokenTTL  time.Duration `koanf:"token_ttl"`
}

// WebSocket holds WebSocket-specific configuration.
type WebSocket struct {
	MaxConnections     int  `koanf:"max_connections"`
	ConnectionTimeout  int  `koanf:"connection_timeout"`
	RateLimitPerMin    int  `koanf:"rate_limit_per_min"`
	MetricsLogEnabled  bool `koanf:"metrics_log_enabled"`
	MetricsLogInterval int  `koanf:"metrics_log_interval"`
}

// Database holds database-specific configuration.
type Database struct {
	// Driver supports: sqlite3, postgres/postgresql/pgx, sqlserver/mssql.
	Driver string `koanf:"driver"`
	// Path is the file path for SQLite databases.
	Path     string `koanf:"path"`
	Host     string `koanf:"host"`
	Port     int    `koanf:"port"`
	Name     string `koanf:"name"`
	User     string `koanf:"user"`
	Password string `koanf:"password"`
	SSLMode  string `koanf:"ssl_mode"`
	// SSLRootCert is the CA certificate file path used to verify the server's
	// certificate. Required when SSLMode is "verify-ca" or "verify-full".
	SSLRootCert string `koanf:"ssl_root_cert"`
	// SSLCert and SSLKey are the client certificate/key pair used for mutual
	// TLS. Optional; both must be set together or not at all.
	SSLCert         string `koanf:"ssl_cert"`
	SSLKey          string `koanf:"ssl_key"`
	MaxOpenConns    int    `koanf:"max_open_conns"`
	MaxIdleConns    int    `koanf:"max_idle_conns"`
	ConnMaxLifetime int    `koanf:"conn_max_lifetime"`
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

// Security holds cryptographic/secret-handling configuration.
type Security struct {
	// EncryptionKey is the single 32-byte key used for ALL at-rest encryption
	// (secrets, subscription tokens, WebSub HMAC secrets).
	EncryptionKey string `koanf:"encryption_key"`
	APIKey        APIKey `koanf:"api_key"`
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
// the shared APIP_CONFIG_FILE_SOURCE_ALLOWLIST env var (see configinterpolate.ResolveAllowlist).
var defaultFileSourceAllowlist = []string{
	"/etc/platform-api",
	"/secrets/platform-api",
}

// platformAPIConfigKey is the top-level TOML table that all Platform API
// settings live under (e.g. [platform_api], [platform_api.database]). This
// namespacing lets a Platform API config file coexist with sibling services'
// sections in a shared deployment config.
const platformAPIConfigKey = "platform_api"

// LoadConfig loads configuration with priority: config file > defaults.
// configPath may be empty — when omitted only env vars and defaults are used.
func LoadConfig(configPath string) (*Server, error) {
	cfg := defaultConfig()
	k := koanf.New(".")

	if configPath != "" {
		if err := k.Load(file.Provider(configPath), toml.Parser()); err != nil {
			return nil, fmt.Errorf("failed to load config file %q: %w", configPath, err)
		}
	}

	// Resolve {{ env }} / {{ file }} interpolation tokens after the env+file merge
	// and before unmarshal, so any config field may pull its value from an
	// environment variable or an allowlisted file. String leaves without a "{{"
	// token pass through unchanged, so a token-free config is unaffected.
	k, err := interpolate(k)
	if err != nil {
		return nil, err
	}

	if err := k.UnmarshalWithConf(platformAPIConfigKey, cfg, koanf.UnmarshalConf{
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
	slog.SetDefault(logger.NewLogger(logger.Config{Level: cfg.Logging.Level, Format: cfg.Logging.Format}))

	if err := validateLoggingConfig(cfg.Logging.Level, cfg.Logging.Format); err != nil {
		return nil, err
	}
	if err := validateTimeoutsConfig(&cfg.Listeners.Timeouts); err != nil {
		return nil, err
	}
	if err := validateDeploymentsConfig(&cfg.Deployments); err != nil {
		return nil, err
	}
	if err := validateEventHubConfig(&cfg.EventHub); err != nil {
		return nil, err
	}
	if err := validateAuthConfig(&cfg.Auth); err != nil {
		return nil, err
	}
	if err := validateWebhookConfig(&cfg.Webhook); err != nil {
		return nil, err
	}
	if err := validateEncryptionKey(cfg.Security.EncryptionKey); err != nil {
		return nil, err
	}
	if err := validateDatabaseConfig(&cfg.Database); err != nil {
		return nil, err
	}
	if err := validateListenersConfig(&cfg.Listeners); err != nil {
		return nil, err
	}
	if err := validateCORSConfig(&cfg.Listeners.CORS); err != nil {
		return nil, err
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

// fileBasedUsersDecodeHook handles decoding auth.file.users from a JSON string
// (e.g. a {{ env }} token) in addition to the native TOML array-of-tables format.
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
			return nil, fmt.Errorf("failed to parse auth.file.users as JSON: %w", err)
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
		{"server.timeouts.read_header", cfg.ReadHeader},
		{"server.timeouts.read", cfg.Read},
		{"server.timeouts.write", cfg.Write},
		{"server.timeouts.idle", cfg.Idle},
	} {
		if f.value < 0 {
			return fmt.Errorf("%s must not be negative (got %s); use 0 to disable the timeout", f.name, f.value)
		}
	}
	if cfg.Read > 0 && cfg.ReadHeader > cfg.Read {
		return fmt.Errorf(
			"server.timeouts.read_header (%s) must not exceed server.timeouts.read (%s): the header deadline would never be reached",
			cfg.ReadHeader, cfg.Read,
		)
	}
	return nil
}

// validateAuthConfig validates the selected auth mode and the section that mode
// activates. Modes are mutually exclusive by construction: auth.mode is a single
// discriminator, so conflicting-mode configurations are inexpressible and only
// the active mode's section is validated.
func validateAuthConfig(auth *Auth) error {
	switch auth.Mode {
	case AuthModeExternalToken:
		return validateJWTConfig(&auth.JWT)
	case AuthModeFile:
		if err := validateJWTConfig(&auth.JWT); err != nil {
			return err
		}
		// TokenTTL only matters in file mode: the login endpoint mints tokens
		// itself here, whereas in plain "external_token" mode tokens are minted
		// externally and their expiry is whatever "exp" claim the issuer set.
		if auth.JWT.TokenTTL <= 0 {
			return fmt.Errorf("Auth.JWT.TokenTTL must be a positive duration when auth.mode is %q "+
				"(set auth.jwt.token_ttl, e.g. \"8h\")", AuthModeFile)
		}
		return validateFileBasedConfig(&auth.File)
	case AuthModeIDP:
		return validateIDPConfig(&auth.IDP, &auth.ClaimMappings)
	default:
		return fmt.Errorf("auth.mode must be %q, %q, or %q (got %q)", AuthModeExternalToken, AuthModeFile, AuthModeIDP, auth.Mode)
	}
}

// validateJWTConfig verifies the local HMAC JWT secret. The same secret signs and
// verifies the login tokens issued in file mode, so it is required in both the
// "external_token" and "file" auth modes. The secret is never generated: a
// missing or malformed key fails startup.
func validateJWTConfig(jwt *JWT) error {
	if jwt.SecretKey == "" {
		return fmt.Errorf("Auth.JWT.SecretKey is required when auth.mode is %q or %q "+
			"(set auth.jwt.secret_key in config via {{ env }}/{{ file }})", AuthModeExternalToken, AuthModeFile)
	}
	if !valid32ByteKey(jwt.SecretKey) {
		return fmt.Errorf("invalid Auth.JWT.SecretKey: must be 64 hex characters or base64 decoding to 32 bytes")
	}
	return nil
}

// validateEncryptionKey verifies the at-rest encryption key.
// A missing or malformed key fails startup.
func validateEncryptionKey(key string) error {
	if key == "" {
		return fmt.Errorf("EncryptionKey is required (set encryption_key in config via " +
			"{{ env }}/{{ file }}")
	}
	if !valid32ByteKey(key) {
		return fmt.Errorf("invalid EncryptionKey: must be 64 hex characters or " +
			"base64 decoding to 32 bytes")
	}
	return nil
}

// validateLoggingConfig rejects a logging.level/logging.format typo at startup
// instead of silently falling back to logger.NewLogger's default (info/json),
// which would leave an operator's requested verbosity or encoding silently
// ignored. The level is matched case-insensitively (canonical form is lowercase).
func validateLoggingConfig(level, format string) error {
	switch strings.ToLower(level) {
	case "debug", "info", "warn", "warning", "error":
	default:
		return fmt.Errorf("logging.level must be one of \"debug\", \"info\", \"warn\", or \"error\" (got %q)", level)
	}
	switch strings.ToLower(format) {
	case "text", "json":
	default:
		return fmt.Errorf("logging.format must be \"text\" or \"json\" (got %q)", format)
	}
	return nil
}

// validateDatabaseConfig fails closed when the selected driver's required
// connection fields are missing, rather than surfacing an opaque driver-level
// connection error only once the server tries to open the database.
func validateDatabaseConfig(cfg *Database) error {
	driver := strings.ToLower(cfg.Driver)
	switch driver {
	case "sqlite3", "postgres", "postgresql", "pgx", "sqlserver", "mssql":
	default:
		return fmt.Errorf("database.driver must be one of \"sqlite3\", \"postgres\", \"postgresql\", \"pgx\", "+
			"\"sqlserver\", or \"mssql\" (got %q)", cfg.Driver)
	}
	if driver == "sqlite3" {
		return nil
	}
	if cfg.Host == "" {
		return fmt.Errorf("database.host is required when database.driver is %q", cfg.Driver)
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("database.port must be between 1 and 65535 when database.driver is %q (got %d)", cfg.Driver, cfg.Port)
	}
	if cfg.Name == "" {
		return fmt.Errorf("database.name is required when database.driver is %q", cfg.Driver)
	}
	if cfg.User == "" {
		return fmt.Errorf("database.user is required when database.driver is %q", cfg.Driver)
	}
	switch cfg.SSLMode {
	case "", "disable", "require", "verify-ca", "verify-full":
	default:
		return fmt.Errorf("database.ssl_mode must be \"disable\", \"require\", \"verify-ca\", or \"verify-full\" (got %q)", cfg.SSLMode)
	}
	if (cfg.SSLMode == "verify-ca" || cfg.SSLMode == "verify-full") && cfg.SSLRootCert == "" {
		return fmt.Errorf("database.ssl_root_cert is required when database.ssl_mode is %q", cfg.SSLMode)
	}
	if (cfg.SSLCert == "") != (cfg.SSLKey == "") {
		return fmt.Errorf("database.ssl_cert and database.ssl_key must both be set together, or both left empty")
	}
	return nil
}

// validateListenersConfig rejects an out-of-range port on either listener and
// a port collision when both listeners are enabled, rather than failing at
// bind time with a generic "address already in use" error.
func validateListenersConfig(l *ServerListeners) error {
	if l.HTTP.Enabled && (l.HTTP.Port <= 0 || l.HTTP.Port > 65535) {
		return fmt.Errorf("server.http.port must be between 1 and 65535 (got %d)", l.HTTP.Port)
	}
	if l.HTTPS.Enabled && (l.HTTPS.Port <= 0 || l.HTTPS.Port > 65535) {
		return fmt.Errorf("server.https.port must be between 1 and 65535 (got %d)", l.HTTPS.Port)
	}
	if l.HTTP.Enabled && l.HTTPS.Enabled && l.HTTP.Port == l.HTTPS.Port {
		return fmt.Errorf("server.http.port and server.https.port must differ when both listeners are enabled (both are %d)", l.HTTP.Port)
	}
	return nil
}

// validateCORSConfig rejects a wildcard origin: CORS.AllowedOrigins is used
// for credentialed cross-origin requests, and wildcard origins cannot be
// combined with credentials without opening a cross-tenant exploit path.
func validateCORSConfig(c *CORS) error {
	for _, o := range c.AllowedOrigins {
		if o == "*" {
			return fmt.Errorf("cors.allowed_origins must not contain \"*\" (wildcard origins cannot be combined with credentialed requests)")
		}
	}
	return nil
}

func validateIDPConfig(idp *IDP, claimMappings *ClaimMappings) error {
	if idp.JWKSUrl == "" {
		return fmt.Errorf("auth.mode=%q requires auth.idp.jwks_url to be configured", AuthModeIDP)
	}
	if len(idp.Issuer) == 0 {
		return fmt.Errorf("auth.mode=%q requires auth.idp.issuer to be configured", AuthModeIDP)
	}
	switch idp.ValidationMode {
	case "scope", "role":
	default:
		return fmt.Errorf("auth.idp.validation_mode must be \"scope\" or \"role\" (got %q)", idp.ValidationMode)
	}
	if idp.ValidationMode == "role" && claimMappings.Roles == "" {
		return fmt.Errorf("auth.idp.validation_mode=role requires auth.claim_mappings.roles to be configured")
	}
	return nil
}

func validateFileBasedConfig(cfg *FileBased) error {
	if cfg.Organization.ID == "" {
		return fmt.Errorf("auth.mode=%q requires auth.file.organization.id to be configured", AuthModeFile)
	}
	if cfg.Organization.DisplayName == "" {
		return fmt.Errorf("auth.mode=%q requires auth.file.organization.display_name to be configured", AuthModeFile)
	}
	if len(cfg.Users) == 0 {
		return fmt.Errorf("auth.mode=%q requires at least one user in auth.file.users", AuthModeFile)
	}
	for i, u := range cfg.Users {
		if u.Username == "" {
			return fmt.Errorf("auth.file.users[%d]: username is required (set it in config via {{ env }}/{{ file }})", i)
		}
		if u.PasswordHash == "" {
			return fmt.Errorf("auth.file.users[%d] (%s): password_hash is required (set it in config via {{ env }}/{{ file }})", i, u.Username)
		}
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
