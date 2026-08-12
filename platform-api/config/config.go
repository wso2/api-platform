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
	"crypto/rsa"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/golang-jwt/jwt/v5"
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
	// Roles names one or more of the roles in the
	// auth.authorization.role_to_scope_mapping file and is the user's entire
	// grant: the login endpoint expands them into the scope claim of the token it
	// issues, unioning what each role grants — most-permissive wins, the same way
	// a token carrying several roles is expanded in role authorization mode. It is
	// the only way to grant a file-mode user, so a user's privileges are expressed
	// exactly the way an IDP expresses them — as roles — and changing what a role
	// grants is a single edit to the mapping file rather than a per-user scope
	// string to keep in sync.
	Roles []string `json:"roles" koanf:"roles"`
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
	// AuthModeInternalToken verifies asymmetrically-signed JWTs (RS256) minted by
	// another trusted platform component holding the matching RSA private key.
	// Verification uses the RSA public key in auth.jwt.public_key_file; symmetric
	// (HMAC) and unsigned ("none") tokens are rejected.
	AuthModeInternalToken = "internal_token"
	// AuthModeFile is AuthModeInternalToken plus local username/password login: the
	// login endpoint authenticates users from auth.file and issues RS256 JWTs signed
	// with the RSA private key in auth.jwt.private_key_file, verified with the matching
	// auth.jwt.public_key_file.
	AuthModeFile = "file"
	// AuthModeIDP validates tokens against an external IDP's JWKS (auth.idp).
	AuthModeIDP = "idp"
)

// Auth groups all authentication-related configuration.
type Auth struct {
	// Mode selects the active authentication mode: "internal_token", "file", or "idp".
	Mode string `koanf:"mode"`
	// Authorization holds every authorization setting — whether it is enforced,
	// how (scope or role), and the role-to-scope mapping file. It is deliberately
	// its own section rather than living under [auth.idp]: authorization applies
	// in every auth mode, because a token minted by an enterprise IDP carries the
	// same roles claim whether the platform verifies it via JWKS or with a local
	// public key.
	Authorization Authorization `koanf:"authorization"`
	// SkipPaths are the path prefixes that bypass authentication and scope
	// enforcement — health/metrics probes, the login endpoint, and the internal
	// routes authenticated by a gateway token instead of a user JWT. It is not
	// operator-configurable (koanf:"-"): the list is a property of the product's
	// own routing, and a wrong entry here is an auth bypass, so it comes from
	// DefaultConfig plus the prefixes plugins declare. Operators turn
	// authorization on and off with auth.authorization.enabled instead.
	SkipPaths []string `koanf:"-"`
	IDP       IDP      `koanf:"idp"`
	// JWT is shared by two modes — "internal_token" mode only verifies
	// tokens minted elsewhere with the public key, "file" mode both signs (with
	// the private key) and verifies (with the public key) using the RSA key pair.
	JWT  JWT       `koanf:"jwt"`
	File FileBased `koanf:"file"`
	// ClaimMappings names the JWT claims that carry each identity field. It is
	// shared by all three auth modes: "idp" reads incoming claims by these
	// names, "file" mode's login endpoint signs tokens using these names, and
	// "internal_token" mode reads tokens minted elsewhere by these names too
	// — one mapping, so issuance and validation can never drift apart. Every
	// field accepts either a flat top-level claim name ("org_id") or a
	// dot-separated path into a nested claim ("realm_access.org_id") — see
	// resolveClaimPath in internal/middleware/auth.go.
	ClaimMappings ClaimMappings `koanf:"claim_mappings"`
}

// Authorization modes selectable via auth.authorization.mode.
const (
	// AuthzModeScope authorizes using the JWT scope claim directly.
	AuthzModeScope = "scope"
	// AuthzModeRole authorizes by expanding the token's roles claim into
	// platform scopes via the auth.authorization.role_to_scope_mapping file.
	AuthzModeRole = "role"
)

// Authorization groups all authorization configuration. It applies in every
// auth mode — authentication (how a token is verified) and authorization (what
// a verified token may do) are configured independently, mirroring the
// separation Kubernetes draws between its authentication and authorization
// configs and Envoy draws between JWT providers and rules.
type Authorization struct {
	// Enabled enforces per-endpoint OAuth2 scopes on validated tokens.
	// Disable only to temporarily bypass authorization during development.
	Enabled bool `koanf:"enabled"`
	// Mode selects how authorization is enforced: "scope" (default) or "role".
	Mode string `koanf:"mode"`
	// RoleToScopeMapping is the path to a YAML file mapping IDP roles to platform
	// scopes. Required in "role" mode (validateAuthorizationConfig rejects an
	// empty path there); unused in "scope" mode.
	RoleToScopeMapping string `koanf:"role_to_scope_mapping"`
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
	Name     string   `koanf:"name"`
	JWKSUrl  string   `koanf:"jwks_url"`
	Issuer   []string `koanf:"issuer"`
	Audience []string `koanf:"audience"`
}

// EventHub holds EventHub-specific configuration for multi-replica HA event delivery.
type EventHub struct {
	PollInterval    time.Duration `koanf:"poll_interval"`
	CleanupInterval time.Duration `koanf:"cleanup_interval"`
	RetentionPeriod time.Duration `koanf:"retention_period"`
}

// Webhook holds configuration for the control-plane webhook receiver. The API Portal
// delivers signed events (API key / subscription changes) to this endpoint. See
// docs-local/platform-api-webhook.md.
type Webhook struct {
	// Enabled controls whether the webhook endpoint is registered.
	Enabled bool `koanf:"enabled"`
	// Secret is the shared secret with the API Portal. It serves two purposes: verifying
	// the HMAC-SHA256 request signature, and deriving (via HKDF-SHA3-256) the AES key that decrypts
	// encrypted payload fields such as an API key secret.
	Secret string `koanf:"secret"`
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

// JWT holds configuration for local asymmetric (RS256) JWT authentication.
// Active when Auth.Mode is AuthModeInternalToken (verify-only; tokens minted by
// another platform component) or AuthModeFile (file mode also issues these
// tokens). Signature
// validation is always on and strictly asymmetric — symmetric (HMAC) and
// unsigned ("none") algorithms are rejected.
//
// TODO(pqc): migrate — RS256 is quantum-vulnerable. Move to an ML-DSA (FIPS 204)
// signature once a Go JWT library exposes it. See post-quantum-cryptography.md.
type JWT struct {
	// PublicKeyFile is the path to a mounted PEM-encoded RSA public key file,
	// used to verify token signatures. Required in both "internal_token" and
	// "file" modes. The key is read from disk at the point of use rather than
	// being interpolated into config at load time, so the PEM content is never
	// held in the config struct.
	PublicKeyFile string `koanf:"public_key_file"`
	// PrivateKeyFile is the path to a mounted PEM-encoded RSA private key file,
	// used to sign tokens. Required only in "file" mode, whose login endpoint
	// mints tokens; unused (and not required) in verify-only "internal_token"
	// mode. Read from disk at the point of use, never cached as content.
	PrivateKeyFile string        `koanf:"private_key_file"`
	Issuer         string        `koanf:"issuer"`
	TokenTTL       time.Duration `koanf:"token_ttl"`
}

// skipJWTValidation is stamped in at BUILD time via ldflags, exactly like the
// binary's version (-X ...config.skipJWTValidation=true) — it is deliberately NOT
// a config-file field. Disabling JWT signature validation is a property of a
// specific build (the cloud build fronted by a trusted mediation layer on a
// private network that has already authenticated the caller and forwards an
// unsigned internal token carrying the org context), never a runtime toggle an
// operator could flip on an internet-facing deployment. The empty default that
// every normal build carries means strict validation.
var skipJWTValidation string

// skipJWTValidationEnabled is the parsed skipJWTValidation, evaluated once at
// package initialization (the ldflags value is fixed at link time) so
// SkipJWTValidation() is a plain field read on the hot authentication path.
var skipJWTValidationEnabled = parseSkipJWTValidation(skipJWTValidation)

// parseSkipJWTValidation reports whether the build-time flag value enables the
// bypass: true only for the literal "true" (case-insensitive, surrounding space
// trimmed); any other value keeps strict validation.
func parseSkipJWTValidation(v string) bool {
	return strings.EqualFold(strings.TrimSpace(v), "true")
}

// SkipJWTValidation reports whether this build disables JWT signature and issuer
// verification in "internal_token" mode (see skipJWTValidation). DANGEROUS: it
// makes unsigned ("none") tokens acceptable, so it must only be true in a build
// deployed behind a trusted mediation layer on a private network. Ignored in
// "file" and "idp" modes.
func SkipJWTValidation() bool {
	return skipJWTValidationEnabled
}

// LoadPublicKey reads and parses the PEM-encoded RSA public key from
// PublicKeyFile. The file is read fresh on every call rather than cached,
// so PublicKeyFile is a mounted-file path, never inlined PEM content.
func (j *JWT) LoadPublicKey() (*rsa.PublicKey, error) {
	raw, err := os.ReadFile(j.PublicKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read auth.jwt.public_key_file %q: %w", j.PublicKeyFile, err)
	}
	pub, err := jwt.ParseRSAPublicKeyFromPEM(raw)
	if err != nil {
		return nil, fmt.Errorf("auth.jwt.public_key_file %q must be a PEM-encoded RSA public key: %w", j.PublicKeyFile, err)
	}
	return pub, nil
}

// LoadPrivateKey reads and parses the PEM-encoded RSA private key from
// PrivateKeyFile. The file is read fresh on every call rather than cached,
// so PrivateKeyFile is a mounted-file path, never inlined PEM content.
func (j *JWT) LoadPrivateKey() (*rsa.PrivateKey, error) {
	raw, err := os.ReadFile(j.PrivateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read auth.jwt.private_key_file %q: %w", j.PrivateKeyFile, err)
	}
	priv, err := jwt.ParseRSAPrivateKeyFromPEM(raw)
	if err != nil {
		return nil, fmt.Errorf("auth.jwt.private_key_file %q must be a PEM-encoded RSA private key: %w", j.PrivateKeyFile, err)
	}
	return priv, nil
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
	MaxPerAPIGateway int  `koanf:"max_per_api_gateway"`
	TimeoutEnabled   bool `koanf:"timeout_enabled"`
	TimeoutInterval  int  `koanf:"timeout_interval"`
	TimeoutDuration  int  `koanf:"timeout_duration"`
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
	configFilePaths []string
	processOnce     sync.Once
	settingInstance *Server
)

// SetConfigPath configures a single config.toml path. Retained for backward
// compatibility; SetConfigPaths is the repeatable form. Must be called before the
// first GetConfig() if a config file is used.
func SetConfigPath(path string) {
	configFilePaths = []string{path}
}

// SetConfigPaths configures one or more config.toml paths, merged in order with
// last-wins precedence. Must be called before the first GetConfig() if config
// files are used.
func SetConfigPaths(paths ...string) {
	configFilePaths = paths
}

// GetConfig returns the singleton config instance, loading it on first call.
func GetConfig() *Server {
	var err error
	processOnce.Do(func() {
		settingInstance, err = LoadConfig(configFilePaths...)
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

// removedAuthSkipPathsKey is a config key that was removed; it is still
// recognized so a config file carrying it fails startup instead of being
// silently ignored. Path is relative to the platform_api subtree.
const removedAuthSkipPathsKey = "auth.skip_paths"

// LoadConfig loads configuration with priority: config files > defaults.
//
// configPaths is repeatable: files are merged in the order given with last-wins
// precedence (a key set in a later file overrides the same key from an earlier
// file). Merge semantics follow koanf — nested tables (maps) deep-merge, while
// list/array values are replaced wholesale, not appended. A field may be overridden
// across files with a different representation — e.g. a numeric value in the base and
// an {{ env }} token (a string) in an overlay — and still resolve, because types are
// only checked after interpolation by the weakly-typed unmarshal.
//
// Zero paths is permitted (the embeddable library API and callers supplying an
// already-built config via the platform façade rely on this) — with no files,
// only the {{ env }}-resolved defaults apply. The `platform-api` binary itself
// requires at least one -config file (enforced in cmd/main.go), so it never
// silently boots on defaults. Any path that is given must exist and parse.
func LoadConfig(configPaths ...string) (*Server, error) {
	cfg := defaultConfig()
	// Deliberately NOT koanf StrictMerge: strict merging compares the raw parsed
	// types across files, but an {{ env }} / {{ file }} interpolation token is a
	// string until it is resolved after the merge — so strict merging would reject a
	// numeric/bool field that one file sets natively and another overrides with a
	// token. Cross-file type errors are instead caught downstream by the weakly-typed
	// unmarshal and Validate.
	k := koanf.New(".")

	// Load each config file in order. Successive loads deep-merge maps and replace
	// arrays, giving last-wins precedence for keys set in more than one file.
	for _, configPath := range configPaths {
		if configPath == "" {
			// A zero-length variadic call (no files) is the legitimate "defaults
			// only" path for embedders; an explicit empty-string path (e.g. the
			// binary invoked with `-config=`) is a malformed input that must fail
			// fast rather than silently degrade to defaults — matching the other
			// loaders, which reject "" via file.Provider.
			return nil, fmt.Errorf("config path must not be empty")
		}
		if err := k.Load(file.Provider(configPath), toml.Parser()); err != nil {
			return nil, fmt.Errorf("failed to load config file %q: %w", configPath, err)
		}
	}

	// Narrow to this component's own subtree BEFORE interpolating, so a shared
	// multi-component config file (one that also carries [api_portal] or
	// [ai_workspace] sections) does not force platform-api to resolve another
	// component's {{ env }}/{{ file }} tokens — those reference env vars and
	// allowlisted paths that only exist in that other component's container, and
	// resolving them here would fail closed. Cut promotes the platform_api.*
	// children to the top level; an absent section yields an empty tree that
	// leaves cfg at its defaults, matching the pre-merge behavior.
	k = k.Cut(platformAPIConfigKey)

	// Resolve {{ env }} / {{ file }} interpolation tokens after the env+file merge
	// and before unmarshal, so any config field may pull its value from an
	// environment variable or an allowlisted file. String leaves without a "{{"
	// token pass through unchanged, so a token-free config is unaffected.
	k, err := interpolate(k)
	if err != nil {
		return nil, err
	}

	// auth.skip_paths used to be operator-configurable. It no longer is (the
	// list is a property of the product's routing, and a wrong entry is an auth
	// bypass), and unknown keys are otherwise ignored — so fail loudly rather
	// than let an operator believe a stale entry is still in effect.
	if k.Exists(removedAuthSkipPathsKey) {
		return nil, fmt.Errorf("config key %q.%s is no longer supported: the auth skip-path list is "+
			"built in and, for plugins, declared by the plugin — remove the key "+
			"(use auth.authorization.enabled to control authorization enforcement)",
			platformAPIConfigKey, removedAuthSkipPathsKey)
	}

	// Subtree is already promoted to the top level by Cut, so unmarshal from the
	// root ("") rather than re-descending through platformAPIConfigKey.
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
	slog.SetDefault(logger.NewLogger(logger.Config{Level: cfg.Logging.Level, Format: cfg.Logging.Format}))

	// Unknown keys are silently dropped by the unmarshal above (ErrorUnused is not
	// set), so a config still carrying a removed setting starts cleanly with that
	// setting having no effect at all. Warn explicitly instead of leaving the
	// operator to infer it from behavior.
	warnRemovedConfigKeys(k)

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
// activates, plus the authorization section that applies in every mode. Modes
// are mutually exclusive by construction: auth.mode is a single discriminator,
// so conflicting-mode configurations are inexpressible and only the active
// mode's section is validated.
func validateAuthConfig(auth *Auth) error {
	for _, p := range auth.SkipPaths {
		if err := ValidateAuthSkipPath(p); err != nil {
			return fmt.Errorf("invalid entry in auth.skip_paths: %w", err)
		}
	}

	if err := validateAuthModeConfig(auth); err != nil {
		return err
	}

	// Authorization is validated outside the mode switch, not inside any one
	// mode's branch: it applies in every authentication mode.
	return validateAuthorizationConfig(&auth.Authorization, &auth.ClaimMappings)
}

func validateAuthModeConfig(auth *Auth) error {
	switch auth.Mode {
	case AuthModeInternalToken:
		// Verify-only: a public key is sufficient (tokens are minted elsewhere).
		return validateJWTConfig(&auth.JWT, false)
	case AuthModeFile:
		// File mode also mints tokens, so it additionally needs a signing key.
		if err := validateJWTConfig(&auth.JWT, true); err != nil {
			return err
		}
		// TokenTTL only matters in file mode: the login endpoint mints tokens
		// itself here, whereas in plain "internal_token" mode tokens are minted
		// elsewhere and their expiry is whatever "exp" claim the issuer set.
		if auth.JWT.TokenTTL <= 0 {
			return fmt.Errorf("Auth.JWT.TokenTTL must be a positive duration when auth.mode is %q "+
				"(set auth.jwt.token_ttl, e.g. \"8h\")", AuthModeFile)
		}
		return validateFileBasedConfig(&auth.File, &auth.Authorization)
	case AuthModeIDP:
		return validateIDPConfig(&auth.IDP)
	default:
		return fmt.Errorf("auth.mode must be %q, %q, or %q (got %q)", AuthModeInternalToken, AuthModeFile, AuthModeIDP, auth.Mode)
	}
}

// ValidateAuthSkipPath rejects a skip-path entry that would widen the auth
// bypass beyond the narrow, specific prefix a skip path is meant to be.
// Skip-path matching is a segment-boundary prefix match, so "" or "/" would
// disable authentication and scope enforcement for every route on the server
// (GO-AUTH-004/GO-AUTH-011) — that must fail startup, not silently pass every
// request through. It is applied to both config-sourced entries and the
// prefixes plugins contribute, so neither source can drift from the other.
func ValidateAuthSkipPath(path string) error {
	switch {
	case path == "":
		return fmt.Errorf("path is empty, which matches every request")
	case path == "/":
		return fmt.Errorf("path is the root prefix, which matches every request")
	case !strings.HasPrefix(path, "/"):
		return fmt.Errorf("path %q must start with %q", path, "/")
	case strings.Contains(path, ".."):
		return fmt.Errorf("path %q must not contain %q", path, "..")
	}
	return nil
}

// validateJWTConfig verifies the local asymmetric JWT key material is present
// and readable. The RSA public key verifies token signatures and is required
// in both the "internal_token" and "file" auth modes. When requireSigningKey
// is true (file mode, which mints tokens at its login endpoint) the RSA
// private key is also required and must form a matching pair with the public
// key. Keys are mounted files, read fresh here rather than cached: a missing
// path, an unreadable file, or malformed material all fail startup. Only
// asymmetric RSA keys are accepted, so symmetric (HMAC) verification is
// structurally impossible.
func validateJWTConfig(jwtCfg *JWT, requireSigningKey bool) error {
	if jwtCfg.PublicKeyFile == "" {
		return fmt.Errorf("Auth.JWT.PublicKeyFile is required when auth.mode is %q or %q "+
			"(set auth.jwt.public_key_file to the path of a mounted PEM-encoded RSA public key)",
			AuthModeInternalToken, AuthModeFile)
	}
	pub, err := jwtCfg.LoadPublicKey()
	if err != nil {
		return fmt.Errorf("invalid Auth.JWT.PublicKeyFile: %w", err)
	}

	if !requireSigningKey {
		return nil
	}

	if jwtCfg.PrivateKeyFile == "" {
		return fmt.Errorf("Auth.JWT.PrivateKeyFile is required when auth.mode is %q "+
			"(set auth.jwt.private_key_file to the path of a mounted PEM-encoded RSA private key)",
			AuthModeFile)
	}
	priv, err := jwtCfg.LoadPrivateKey()
	if err != nil {
		return fmt.Errorf("invalid Auth.JWT.PrivateKeyFile: %w", err)
	}
	if !priv.PublicKey.Equal(pub) {
		return fmt.Errorf("Auth.JWT.PrivateKeyFile and Auth.JWT.PublicKeyFile must be a matching RSA key pair")
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

func validateIDPConfig(idp *IDP) error {
	if idp.JWKSUrl == "" {
		return fmt.Errorf("auth.mode=%q requires auth.idp.jwks_url to be configured", AuthModeIDP)
	}
	if len(idp.Issuer) == 0 {
		return fmt.Errorf("auth.mode=%q requires auth.idp.issuer to be configured", AuthModeIDP)
	}
	return nil
}

// validateAuthorizationConfig validates the [auth.authorization] section. It is
// checked in every auth mode: role-based authorization is equally meaningful
// against a locally-verified token as against a JWKS-verified one, so its
// validity must not depend on which authentication mode is active.
func validateAuthorizationConfig(authz *Authorization, claimMappings *ClaimMappings) error {
	switch authz.Mode {
	case AuthzModeScope, AuthzModeRole:
	default:
		return fmt.Errorf("auth.authorization.mode must be %q or %q (got %q)", AuthzModeScope, AuthzModeRole, authz.Mode)
	}
	if authz.Mode == AuthzModeRole {
		if claimMappings.Roles == "" {
			return fmt.Errorf("auth.authorization.mode=%s requires auth.claim_mappings.roles to be configured", AuthzModeRole)
		}
		// Without a mapping file, role names would be used verbatim as scope
		// values — an operator's IDP role would have to happen to be spelled
		// exactly like a platform scope, so silently accepting an empty path
		// means authorization that denies everything (or, for a role named after
		// a scope, grants unintentionally). Require the mapping explicitly.
		if authz.RoleToScopeMapping == "" {
			return fmt.Errorf("auth.authorization.mode=%s requires auth.authorization.role_to_scope_mapping to be configured", AuthzModeRole)
		}
	}
	return nil
}

func validateFileBasedConfig(cfg *FileBased, authz *Authorization) error {
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
		// The roles are the user's whole grant, so a user without any is
		// authenticated and then authorized for nothing — a login that succeeds and
		// then fails every request. Reject it at startup instead of issuing a token
		// with an empty scope claim. An entry that is present but blank is the same
		// mistake spelled differently, so reject it here rather than letting it
		// expand to nothing later.
		if len(u.Roles) == 0 {
			return fmt.Errorf("auth.file.users[%d] (%s): roles is required — name at least one role from auth.authorization.role_to_scope_mapping", i, u.Username)
		}
		for j, role := range u.Roles {
			if role == "" {
				return fmt.Errorf("auth.file.users[%d] (%s): roles[%d] is empty — name a role from auth.authorization.role_to_scope_mapping", i, u.Username, j)
			}
		}
		// The roles are expanded from the mapping file at login, so without the
		// file they grant nothing.
		if authz.RoleToScopeMapping == "" {
			return fmt.Errorf("auth.file.users[%d] (%s): roles %v require auth.authorization.role_to_scope_mapping to be configured",
				i, u.Username, u.Roles)
		}
	}
	return nil
}

// validateWebhookConfig validates and fills defaults for the webhook receiver config.
// It is a no-op when the webhook is disabled.
// removedConfigKeys maps a config key that no longer exists to the guidance shown
// when it is still present. Keys are relative to the platform_api subtree, matching
// the koanf tree after Cut. Add an entry whenever a setting is dropped: the
// unmarshal ignores unknown keys, so without this a stale setting is inert and
// silent, and an operator has no way to tell it stopped doing anything.
var removedConfigKeys = map[string]string{
	"webhook.private_key_path": "webhook payload fields are now encrypted with a key derived from webhook.secret " +
		"instead of an RSA key pair; this setting has no effect and the PEM file/mount it points to can be deleted",
	"deployments.transitional_status_enabled": "a deployment is now always treated as transitional until the gateway " +
		"acknowledges it, so there is nothing left to toggle; this setting has no effect and can be deleted " +
		"(deployments.timeout_duration bounds how long an unacknowledged deployment waits before being marked FAILED)",
}

// warnRemovedConfigKeys logs a warning for each removed key still present in the
// loaded config. It deliberately warns rather than failing: the removed setting is
// inert, the service is fully functional without it, and refusing to start would
// break an otherwise-working deployment on upgrade.
func warnRemovedConfigKeys(k *koanf.Koanf) {
	for key, guidance := range removedConfigKeys {
		if k.Exists(key) {
			slog.Warn("ignoring removed configuration key",
				"key", platformAPIConfigKey+"."+key,
				"detail", guidance)
		}
	}
}

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
		w.SignatureHeader = "X-Api-Portal-Signature"
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
