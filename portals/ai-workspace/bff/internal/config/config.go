/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package config loads BFF configuration from config.toml, resolving its
// {{ env }} / {{ file }} interpolation tokens through the shared configinterpolate
// library and unmarshalling the result into a nested struct via koanf — the same
// loading stack the Gateway and Platform API use. The file is the only source: a key
// takes its value from the environment or a mounted secret file exactly when its token
// says so. Browser-safe keys are surfaced to the SPA as APIP_AIW_* runtime config. The
// BFF never validates tokens, so there are no signing keys here — only the IDP client
// credentials needed to perform the OAuth2 code exchange.
package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/knadh/koanf/v2"
)

// Config is the fully-resolved BFF configuration. Its shape mirrors the
// [ai_workspace.*] tables in config.toml, so koanf unmarshals straight into it — the
// same pattern the Platform API uses. Keys the BFF does not consume (browser-only
// values the SPA reads) are deliberately not modeled here; they flow to RuntimeConfig
// straight from the parsed config, gated by browserSafeKeys (see runtime_config.go).
type Config struct {
	// Domain is the externally-reachable host:port for this deployment (e.g. the
	// address an nginx reverse proxy exposes it on), used only for the startup log
	// banner. The internal listener bind address (see ServerConfig) is not what an
	// operator should visit when a proxy in front of the BFF exposes a different
	// host/port, so this is a separate, explicit value rather than derived from the
	// listener. Not browser-safe: the SPA never needs it, since the browser already
	// knows its own address via window.location.
	Domain string `koanf:"domain"`

	Server       ServerConfig       `koanf:"server"`
	Logging      LoggingConfig      `koanf:"logging"`
	ControlPlane ControlPlaneConfig `koanf:"control_plane"`
	Session      SessionConfig      `koanf:"session"`
	Auth         AuthConfig         `koanf:"auth"`

	// Cookie attributes are fixed implementation details of the session mechanism, not
	// deployment config; Load sets them. RuntimeConfig is assembled after load.
	Cookie        CookieConfig      `koanf:"-"`
	RuntimeConfig map[string]string `koanf:"-"`
}

// ServerConfig is [ai_workspace.server]: two independent listeners, following the
// platform-wide [server.http] / [server.https] shape — either or both may run, each
// on its own port, so a deployment can serve plain HTTP internally, HTTPS externally,
// or both at once to migrate clients between them without downtime. The listeners
// always bind all interfaces, so there is no host to configure. The SPA never needs
// to be told its own origin — the browser already knows it via window.location; this
// is not sent to the browser (see browserSafeKeys in runtime_config.go).
type ServerConfig struct {
	StaticDir string        `koanf:"static_dir"` // directory containing the built SPA (index.html + assets)
	HTTP      HTTPListener  `koanf:"http"`
	HTTPS     HTTPSListener `koanf:"https"`
}

// HTTPListener configures the plain-HTTP listener. Enable it only when a trusted
// upstream (ingress, service-mesh sidecar) terminates TLS, or for internal traffic;
// never expose it directly to untrusted networks.
type HTTPListener struct {
	Enabled bool `koanf:"enabled"`
	Port    int  `koanf:"port"`
}

// HTTPSListener configures the TLS listener. CertFile/KeyFile are required when
// Enabled — there is no self-signed fallback.
type HTTPSListener struct {
	Enabled  bool   `koanf:"enabled"`
	Port     int    `koanf:"port"`
	CertFile string `koanf:"cert_file"`
	KeyFile  string `koanf:"key_file"`
}

// LoggingConfig is [ai_workspace.logging]. Level/Format are this process's own logs;
// browser_debug is browser-only and not modeled here — it reaches the SPA through
// RuntimeConfig. Level and Format are matched case-insensitively (lowercased in Load).
type LoggingConfig struct {
	Level  string `koanf:"level"`  // debug | info | warn | error (default "info")
	Format string `koanf:"format"` // text | json (default "text")
}

// ControlPlaneConfig is [ai_workspace.control_plane]: everything about the upstream
// Platform API hop — where it is, how its TLS certificate is trusted, and the
// same-origin prefix the SPA calls.
type ControlPlaneConfig struct {
	// URL is the base URL, e.g. https://platform-api:9243. Its http/https scheme is
	// the single source of truth for whether the outbound hop uses TLS — there is
	// deliberately no separate boolean, since that could contradict the URL.
	URL string `koanf:"url"`
	// CAFile is a PEM bundle to trust for the upstream's TLS certificate, appended to
	// the system roots rather than replacing them. Ignored when TLSSkipVerify is true.
	CAFile string `koanf:"ca_file"`
	// TLSSkipVerify disables upstream certificate verification entirely. Last-resort
	// escape hatch for dev/demo only; prefer CAFile.
	TLSSkipVerify bool `koanf:"tls_skip_verify"`
	// PortalBasePath is the Platform API's portal route prefix (e.g. /api/portal/v0.9),
	// used to build paths for BFF-initiated calls (file-based login today).
	PortalBasePath string `koanf:"portal_base_path"`
	// ProxyPrefix is the same-origin reverse-proxy prefix the SPA calls; it is stripped
	// before forwarding upstream, so the browser only ever talks to the app origin.
	ProxyPrefix string `koanf:"proxy_prefix"`
}

// SessionConfig is [ai_workspace.session]: server-side session lifetime.
type SessionConfig struct {
	Store       string        `koanf:"store"`        // "memory" (default) | "redis" (future)
	IdleTimeout time.Duration `koanf:"idle_timeout"` // sliding idle window
	AbsoluteTTL time.Duration `koanf:"absolute_ttl"` // hard cap regardless of activity / token exp
}

// AuthConfig is [ai_workspace.auth]: the login mode and the claim/OIDC settings.
type AuthConfig struct {
	Mode          string             `koanf:"mode"` // "basic" | "oidc" — informs the SPA which login UX to show
	OIDC          OIDCConfig         `koanf:"oidc"`
	ClaimMappings ClaimMappingConfig `koanf:"claim_mappings"`
}

// OIDCConfig is [ai_workspace.auth.oidc]: the confidential-client settings. The client
// secret lives only here on the BFF and is never emitted to the browser. Enabled is
// both a config key and derived — Load ORs it with (auth.mode == "oidc").
type OIDCConfig struct {
	Enabled               bool   `koanf:"enabled"`
	Issuer                string `koanf:"authority"` // discovery base; {issuer}/.well-known/openid-configuration
	ClientID              string `koanf:"client_id"`
	ClientSecret          string `koanf:"client_secret"`
	RedirectURL           string `koanf:"redirect_url"` // must equal the IDP-registered redirect, points at /api/auth/callback
	PostLogoutRedirectURL string `koanf:"post_logout_redirect_url"`
	Scopes                string `koanf:"scope"` // space-separated
}

// ClaimMappingConfig is [ai_workspace.auth.claim_mappings]: which claim names the BFF
// reads for each user/org field. It mirrors the Platform API's [auth.claim_mappings]
// key for key, and the two must agree. It is a sibling of [auth.oidc], not nested
// inside it, because it applies to BOTH auth modes — OIDC tokens from the configured
// IDP, and the HMAC JWTs the Platform API's file-based login endpoint signs with these
// same mapped claim names. The same keys drive the BFF's session mapping and the SPA's
// runtime config, so one config entry keeps both layers in sync.
type ClaimMappingConfig struct {
	Username  string `koanf:"username"`
	Email     string `koanf:"email"`
	Roles     string `koanf:"roles"`
	Scope     string `koanf:"scope"`
	OrgID     string `koanf:"organization"`
	OrgName   string `koanf:"org_name"`
	OrgHandle string `koanf:"org_handle"`
}

// CookieConfig controls the session cookie attributes. Not user-configurable: these
// are implementation details of the BFF's session mechanism, not a deployment concern.
// The BFF always terminates TLS (or sits behind a proxy that does), so Secure is
// unconditionally true; there is no supported plain-HTTP deployment that would need it
// false.
type CookieConfig struct {
	Name     string
	Secure   bool
	SameSite string // "lax" | "strict" | "none"
}

// cookieName is the session cookie's name.
const cookieName = "_ai_workspace_session"

// CSRFHeaderName is the header the SPA must set on every state-mutating request, and
// the BFF checks for on the way in (see server/middleware.go requireCSRF). It is a
// fixed contract between the BFF and the SPA it ships, not a deployment concern — an
// operator changing it on one side without the other would silently break CSRF
// protection, so it is a constant rather than a config key. The SPA's copy lives in
// src/config.env.ts CSRF_HEADER and must be kept in sync with this value.
const CSRFHeaderName = "X-Requested-By"

// defaultOIDCScopes is the full set of scopes the BFF requests in OIDC mode so a
// logged-in user's access token carries every ap:* permission the Platform API
// authorizes against. The IDP must still have these scopes registered and granted
// to the user, otherwise it drops the ungranted ones. Override with the [auth.oidc] scope
// config key to request a narrower set.
//
// offline_access is required: without it most IDPs (Asgardeo, WSO2 IS, Okta,
// Azure AD) issue no refresh token, so the BFF cannot silently renew the access
// token and the user is logged out the moment it expires. Keep it in any override.
const defaultOIDCScopes = "openid profile email offline_access" +
	" ap:organization:read ap:organization:manage ap:organization:subscription:read" +
	" ap:project:read ap:project:create ap:project:update ap:project:delete ap:project:manage" +
	" ap:application:read ap:application:create ap:application:update ap:application:delete ap:application:manage" +
	" ap:application:api_key:read ap:application:api_key:create ap:application:api_key:delete ap:application:api_key:manage" +
	" ap:application:association:read ap:application:association:create ap:application:association:delete ap:application:association:manage ap:application:association:api_key:read" +
	" ap:gateway:read ap:gateway:create ap:gateway:update ap:gateway:delete ap:gateway:manage" +
	" ap:gateway:token:read ap:gateway:token:create ap:gateway:token:delete ap:gateway:token:manage" +
	" ap:gateway_custom_policy:read ap:gateway_custom_policy:create ap:gateway_custom_policy:delete ap:gateway_custom_policy:manage" +
	" ap:gateway:artifact:read ap:gateway:manifest:read" +
	" ap:rest_api:read ap:rest_api:create ap:rest_api:update ap:rest_api:delete ap:rest_api:manage ap:rest_api:import" +
	" ap:rest_api:gateway:read ap:rest_api:gateway:create ap:rest_api:gateway:manage" +
	" ap:rest_api:deployment:read ap:rest_api:deployment:create ap:rest_api:deployment:delete ap:rest_api:deployment:manage ap:rest_api:deployment:undeploy ap:rest_api:deployment:restore" +
	" ap:rest_api:api_key:read ap:rest_api:api_key:create ap:rest_api:api_key:update ap:rest_api:api_key:delete ap:rest_api:api_key:manage" +
	" ap:rest_api:publication:read ap:rest_api:publication:create ap:rest_api:publication:delete" +
	" ap:devportal:read ap:devportal:create ap:devportal:update ap:devportal:delete ap:devportal:manage" +
	" ap:subscription:read ap:subscription:create ap:subscription:update ap:subscription:delete ap:subscription:manage" +
	" ap:subscription_plan:read ap:subscription_plan:create ap:subscription_plan:update ap:subscription_plan:delete ap:subscription_plan:manage" +
	" ap:llm_template:read ap:llm_template:create ap:llm_template:update ap:llm_template:delete ap:llm_template:manage" +
	" ap:llm_provider:read ap:llm_provider:create ap:llm_provider:update ap:llm_provider:delete ap:llm_provider:manage" +
	" ap:llm_provider:api_key:read ap:llm_provider:api_key:create ap:llm_provider:api_key:delete ap:llm_provider:api_key:manage" +
	" ap:llm_provider:deployment:read ap:llm_provider:deployment:create ap:llm_provider:deployment:delete ap:llm_provider:deployment:manage ap:llm_provider:deployment:undeploy ap:llm_provider:deployment:restore" +
	" ap:llm_proxy:read ap:llm_proxy:create ap:llm_proxy:update ap:llm_proxy:delete ap:llm_proxy:manage" +
	" ap:llm_proxy:api_key:read ap:llm_proxy:api_key:create ap:llm_proxy:api_key:delete ap:llm_proxy:api_key:manage" +
	" ap:llm_proxy:deployment:read ap:llm_proxy:deployment:create ap:llm_proxy:deployment:delete ap:llm_proxy:deployment:manage ap:llm_proxy:deployment:undeploy ap:llm_proxy:deployment:restore" +
	" ap:mcp_proxy:read ap:mcp_proxy:create ap:mcp_proxy:update ap:mcp_proxy:delete ap:mcp_proxy:manage" +
	" ap:mcp_proxy:deployment:read ap:mcp_proxy:deployment:create ap:mcp_proxy:deployment:delete ap:mcp_proxy:deployment:manage ap:mcp_proxy:deployment:undeploy ap:mcp_proxy:deployment:restore" +
	" ap:websub_api:read ap:websub_api:create ap:websub_api:update ap:websub_api:delete ap:websub_api:manage" +
	" ap:websub_api:api_key:read ap:websub_api:api_key:create ap:websub_api:api_key:delete ap:websub_api:api_key:manage ap:websub_api:api_key:update" +
	" ap:websub_api:deployment:read ap:websub_api:deployment:create ap:websub_api:deployment:delete ap:websub_api:deployment:manage ap:websub_api:deployment:undeploy ap:websub_api:deployment:restore" +
	" ap:websub_api:publication:read ap:websub_api:publication:create ap:websub_api:publication:delete" +
	" ap:webbroker_api:read ap:webbroker_api:create ap:webbroker_api:update ap:webbroker_api:delete ap:webbroker_api:manage" +
	" ap:webbroker_api:api_key:read ap:webbroker_api:api_key:create ap:webbroker_api:api_key:delete ap:webbroker_api:api_key:manage ap:webbroker_api:api_key:update" +
	" ap:webbroker_api:deployment:read ap:webbroker_api:deployment:create ap:webbroker_api:deployment:delete ap:webbroker_api:deployment:manage ap:webbroker_api:deployment:undeploy ap:webbroker_api:deployment:restore" +
	" ap:webbroker_api:publication:read ap:webbroker_api:publication:create ap:webbroker_api:publication:delete" +
	" ap:secret:read ap:secret:create ap:secret:update ap:secret:delete ap:secret:manage" +
	" ap:git:read"

// DefaultConfigFile is where the container mounts config.toml. It is the path used
// unless -config names another one.
const DefaultConfigFile = "/etc/ai-workspace/config.toml"

// Load resolves configuration from the config.toml at path, or from the mounted
// DefaultConfigFile when path is empty. It loads defaults, overlays the file (with its
// {{ env }} / {{ file }} tokens expanded), normalizes derived fields, then validates —
// so any key, the OIDC client secret in particular, can be pulled from an environment
// variable or a mounted secret file instead of being written in the clear, and a key
// not present in the file falls back to its default.
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigFile
	}
	k, err := loadConfigKoanf(path)
	if err != nil {
		return nil, err
	}

	// Defaults first, then overlay the file. WeaklyTypedInput lets a {{ env }} token's
	// string value decode into the typed field (e.g. ":9680" -> int, "true" -> bool);
	// a value that cannot be coerced (e.g. enabled = "maybe") fails startup here rather
	// than being silently dropped.
	cfg := defaultConfig()
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

	cfg.normalize()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	cfg.RuntimeConfig = buildRuntimeConfig(cfg, k)
	return cfg, nil
}

// normalize resolves the derived fields that are not a straight copy of a config key:
// case-folding (level/format/mode), trimming trailing slashes off URLs/prefixes, the
// oidc-mode-implies-enabled rule, and the fixed cookie attributes.
func (c *Config) normalize() {
	c.Logging.Level = strings.ToLower(c.Logging.Level)
	c.Logging.Format = strings.ToLower(c.Logging.Format)
	c.Auth.Mode = strings.ToLower(c.Auth.Mode)

	c.ControlPlane.URL = strings.TrimRight(c.ControlPlane.URL, "/")
	c.ControlPlane.PortalBasePath = strings.TrimRight(c.ControlPlane.PortalBasePath, "/")
	c.ControlPlane.ProxyPrefix = strings.TrimRight(c.ControlPlane.ProxyPrefix, "/")
	c.Auth.OIDC.Issuer = strings.TrimRight(c.Auth.OIDC.Issuer, "/")

	// oidc mode implies the client is enabled even if the explicit flag is unset, so a
	// typo'd mode cannot silently degrade to basic auth.
	c.Auth.OIDC.Enabled = c.Auth.OIDC.Enabled || c.Auth.Mode == "oidc"

	c.Cookie = CookieConfig{Name: cookieName, Secure: true, SameSite: "lax"}
}

// validate fails startup on any value that would otherwise surface as a confusing
// runtime error (a bad port, an empty upstream URL, an incomplete OIDC set) and warns
// on security-relevant downgrades.
func (c *Config) validate() error {
	// A typo'd mode must not silently degrade to basic auth: any value other than
	// "oidc" would leave OIDC.Enabled false and hand the SPA an unknown login UX.
	if c.Auth.Mode != "basic" && c.Auth.Mode != "oidc" {
		return fmt.Errorf("invalid [auth] mode %q: must be \"basic\" or \"oidc\"", c.Auth.Mode)
	}
	if !c.Server.HTTP.Enabled && !c.Server.HTTPS.Enabled {
		return fmt.Errorf("no listeners enabled: set [server.http] enabled = true and/or [server.https] enabled = true")
	}
	if c.Server.HTTP.Enabled && (c.Server.HTTP.Port < 1 || c.Server.HTTP.Port > 65535) {
		return fmt.Errorf("[server.http] port must be between 1 and 65535, got %d", c.Server.HTTP.Port)
	}
	if c.Server.HTTPS.Enabled && (c.Server.HTTPS.Port < 1 || c.Server.HTTPS.Port > 65535) {
		return fmt.Errorf("[server.https] port must be between 1 and 65535, got %d", c.Server.HTTPS.Port)
	}
	if c.Server.HTTP.Enabled && c.Server.HTTPS.Enabled && c.Server.HTTP.Port == c.Server.HTTPS.Port {
		return fmt.Errorf("[server.http] port and [server.https] port must differ, both are %d", c.Server.HTTP.Port)
	}
	// Every session duration is a lifetime, where <= 0 is never meaningful.
	if c.Session.IdleTimeout <= 0 {
		return fmt.Errorf("[session] idle_timeout must be positive, got %s", c.Session.IdleTimeout)
	}
	if c.Session.AbsoluteTTL <= 0 {
		return fmt.Errorf("[session] absolute_ttl must be positive, got %s", c.Session.AbsoluteTTL)
	}

	if c.ControlPlane.URL == "" {
		return fmt.Errorf("[control_plane] url is required: set it in config.toml, " +
			"either as a literal or via an {{ env }} / {{ file }} token")
	}
	// The scheme is the single source of truth for the outbound TLS decision, so a
	// missing/typo'd scheme must fail at startup rather than surface as an opaque dial
	// error on the first proxied request.
	u, err := url.Parse(c.ControlPlane.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("[control_plane] url must be an absolute http:// or https:// URL, got %q", c.ControlPlane.URL)
	}
	// Trust knobs only apply to an https upstream; flag them on a plain-http URL so a
	// mistaken belief that TLS is in effect is caught early.
	if u.Scheme == "http" {
		if c.ControlPlane.CAFile != "" || c.ControlPlane.TLSSkipVerify {
			return fmt.Errorf("[control_plane] ca_file / tls_skip_verify are set but [control_plane] url is http:// (no TLS on the upstream hop)")
		}
	}
	// Skipping verification is a security downgrade; say so loudly and point at the
	// supported alternative.
	if u.Scheme == "https" && c.ControlPlane.TLSSkipVerify {
		slog.Warn("[control_plane] tls_skip_verify = true — upstream certificate verification is DISABLED. " +
			"Trust the upstream certificate with [control_plane] ca_file instead.")
	}

	if c.Auth.OIDC.Enabled {
		if c.Auth.OIDC.Issuer == "" || c.Auth.OIDC.ClientID == "" || c.Auth.OIDC.ClientSecret == "" || c.Auth.OIDC.RedirectURL == "" {
			return fmt.Errorf("OIDC mode requires [auth.oidc] authority, client_id, client_secret and redirect_url")
		}
	}
	// Empty is fine (the key is optional), but a relative value would be forwarded as an
	// invalid post_logout_redirect_uri and only fail at logout time — catch it here.
	if c.Auth.OIDC.PostLogoutRedirectURL != "" {
		u, err := url.Parse(c.Auth.OIDC.PostLogoutRedirectURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("[auth.oidc] post_logout_redirect_url must be an absolute http:// or https:// URL, got %q",
				c.Auth.OIDC.PostLogoutRedirectURL)
		}
	}

	// Basic (file-based) auth is supported for quickstart deployments but is not
	// recommended for production; point operators at OIDC.
	if !c.Auth.OIDC.Enabled {
		slog.Warn("basic (file-based) auth is enabled — this is not recommended for production; " +
			"configure OIDC (set [auth] mode = \"oidc\" and [auth.oidc] authority, client_id, client_secret, redirect_url)")
	}

	return nil
}
