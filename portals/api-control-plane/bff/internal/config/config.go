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
// {{ env }} / {{ file }} interpolation tokens through the local configinterpolate
// package and unmarshalling the result into a nested struct via koanf — the same
// loading stack the Gateway, Platform API, and AI Workspace BFF use. The file is
// the only source: a key takes its value from the environment or a mounted secret
// file exactly when its token says so. The BFF never validates tokens itself, so
// there are no signing keys here — only the IDP client credentials needed to
// perform the OAuth2 code exchange.
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
// [api_control_plane.*] tables in config.toml, so koanf unmarshals straight into
// it. Keys the BFF does not consume (browser-only values the SPA reads) are
// deliberately not modeled here; they flow to RuntimeConfig straight from the
// parsed config (see runtime_config.go).
type Config struct {
	// Domain is the externally-reachable host:port for this deployment, used only
	// for the startup log banner. The browser never needs it — it already knows
	// its own address via window.location.
	Domain string `koanf:"domain"`

	Server       ServerConfig       `koanf:"server"`
	Logging      LoggingConfig      `koanf:"logging"`
	ControlPlane ControlPlaneConfig `koanf:"control_plane"`
	Session      SessionConfig      `koanf:"session"`
	Auth         AuthConfig         `koanf:"auth"`
	Features     FeatureConfig      `koanf:"features"`

	RuntimeConfig map[string]string `koanf:"-"`
}

// FeatureConfig is [api_control_plane.features]. Feature switches are emitted
// to the SPA runtime config but remain server-owned deployment settings.
type FeatureConfig struct {
	ObservabilityLogs bool `koanf:"observability_logs"`
}

// ServerConfig is [api_control_plane.server]: two independent listeners,
// following the platform-wide [server.http] / [server.https] shape — either or
// both may run, each on its own port. The listeners always bind all interfaces.
type ServerConfig struct {
	StaticDir string        `koanf:"static_dir"` // directory containing the built SPA (index.html + assets)
	HTTP      HTTPListener  `koanf:"http"`
	HTTPS     HTTPSListener `koanf:"https"`
}

// HTTPListener configures the plain-HTTP listener. Enable it only when a trusted
// upstream (ingress, service-mesh sidecar, cloud gateway) terminates TLS, or for
// local development; never expose it directly to untrusted networks.
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

// LoggingConfig is [api_control_plane.logging]. Level and Format are matched
// case-insensitively (lowercased in normalize).
type LoggingConfig struct {
	Level  string `koanf:"level"`  // debug | info | warn | error (default "info")
	Format string `koanf:"format"` // text | json (default "text")
}

// ControlPlaneConfig is [api_control_plane.control_plane]: the primary upstream
// (Platform API, or a cloud org-context proxy in front of it) plus any additional
// named upstreams (e.g. a billing service) mounted alongside it. Named upstreams
// exist so a deployment that must call more than one backend (cloud's
// first-login billing activation is the motivating case) can proxy all of them
// same-origin through this BFF instead of handing the browser a token — the
// primary upstream alone is sufficient for every standalone deployment today.
type ControlPlaneConfig struct {
	// URL is the primary upstream's base URL, e.g. https://platform-api:9243. Its
	// http/https scheme is the single source of truth for whether that hop uses
	// TLS — there is deliberately no separate boolean, since that could
	// contradict the URL.
	URL string `koanf:"url"`
	// CAFile is a PEM bundle to trust for the primary upstream's TLS certificate,
	// appended to the system roots rather than replacing them. Ignored when
	// TLSSkipVerify is true.
	CAFile string `koanf:"ca_file"`
	// TLSSkipVerify disables the primary upstream's certificate verification
	// entirely. Last-resort escape hatch for dev/demo only; prefer CAFile.
	TLSSkipVerify bool `koanf:"tls_skip_verify"`
	// PortalBasePath is the primary upstream's portal route prefix (e.g.
	// /api/portal/v0.9), used to build paths for BFF-initiated calls (file-based
	// login today).
	PortalBasePath string `koanf:"portal_base_path"`
	// ProxyPrefix is the same-origin reverse-proxy prefix the SPA calls for the
	// primary upstream; it is stripped before forwarding, so the browser only
	// ever talks to this BFF's own origin.
	ProxyPrefix string `koanf:"proxy_prefix"`
	// Upstreams are additional named backends, each proxied same-origin at
	// {name-derived prefix}/*. Optional; empty for every standalone deployment.
	Upstreams []UpstreamConfig `koanf:"upstreams"`
}

// UpstreamConfig is one [[api_control_plane.control_plane.upstreams]] entry: a
// second (or third, ...) backend proxied same-origin alongside the primary one.
type UpstreamConfig struct {
	// Name identifies the upstream and, when Prefix is unset, derives its mount
	// point as {control_plane.proxy_prefix}/{name}/*.
	Name          string `koanf:"name"`
	Prefix        string `koanf:"prefix"` // explicit mount point; overrides the name-derived default
	URL           string `koanf:"url"`
	CAFile        string `koanf:"ca_file"`
	TLSSkipVerify bool   `koanf:"tls_skip_verify"`
}

// SessionConfig is [api_control_plane.session]: server-side session lifetime and
// the cookie attributes the browser receives it under.
type SessionConfig struct {
	Store       string        `koanf:"store"`        // "memory" (default) | "redis" (future)
	IdleTimeout time.Duration `koanf:"idle_timeout"` // sliding idle window
	AbsoluteTTL time.Duration `koanf:"absolute_ttl"` // hard cap regardless of activity / token exp
	Cookie      CookieConfig  `koanf:"cookie"`
}

// CookieConfig controls the session cookie's attributes. Unlike a fixed
// implementation detail, these ARE configurable: Secure must be false for local
// HTTP development (a cloud deployment behind a TLS-terminating gateway sets it
// true), and SameSite/Name are deployment choices, not universal constants.
type CookieConfig struct {
	Name     string `koanf:"name"`
	Secure   bool   `koanf:"secure"`
	SameSite string `koanf:"same_site"` // "lax" | "strict" | "none"
}

// AuthConfig is [api_control_plane.auth]: the login mode and the claim/OIDC
// settings.
type AuthConfig struct {
	Mode          string             `koanf:"mode"` // "basic" | "oidc" — informs the SPA which login UX to show
	OIDC          OIDCConfig         `koanf:"oidc"`
	ClaimMappings ClaimMappingConfig `koanf:"claim_mappings"`
}

// OIDCConfig is [api_control_plane.auth.oidc]: the confidential (or public+PKCE)
// client settings. The client secret, when set, lives only here on the BFF and is
// never emitted to the browser.
//
// Every field below exists specifically so this BFF stays IdP-agnostic: Platform
// API's file-based mode needs none of it, a standards-conformant IdP (Asgardeo,
// Keycloak) needs only Authority/ClientID/ClientSecret/RedirectURL/Scopes, and a
// non-conformant IdP (WSO2 Thunder: bare-name issuer, no discovery document) needs
// the Discovery/endpoint-override/ClientAuthMethod knobs. None of this is
// Thunder-specific code — it is the generic seam a future cloud deployment
// configures, exactly like any other IdP.
type OIDCConfig struct {
	Enabled bool `koanf:"enabled"`

	// Authority is the discovery base URL: {authority}/.well-known/openid-configuration.
	// Required when Discovery is true, otherwise unused.
	Authority string `koanf:"authority"`
	// Issuer is the expected `iss` claim on tokens from this IdP. Defaults to
	// Authority when empty. Deliberately NEVER URL-validated or required to be a
	// URL — some IdPs (Thunder) issue a bare name like "platform-idp".
	Issuer string `koanf:"issuer"`
	// Discovery controls whether the BFF fetches Authority's discovery document
	// at startup. When false, every *Endpoint field below must be set explicitly
	// and no HTTP call is made to discover them.
	Discovery bool `koanf:"discovery"`

	AuthorizationEndpoint string `koanf:"authorization_endpoint"`
	TokenEndpoint         string `koanf:"token_endpoint"`
	EndSessionEndpoint    string `koanf:"end_session_endpoint"` // optional even when set elsewhere; logout degrades gracefully
	UserinfoEndpoint      string `koanf:"userinfo_endpoint"`
	JWKSURI               string `koanf:"jwks_uri"`

	ClientID     string `koanf:"client_id"`
	ClientSecret string `koanf:"client_secret"`
	// ClientAuthMethod is how the client authenticates at the token endpoint:
	// "client_secret_post" (default), "client_secret_basic", or "none" for a
	// public client driven server-side by this BFF (PKCE still protects the
	// flow; the browser still never holds a token). Some IdPs cannot register a
	// confidential client — "none" is what makes those usable without weakening
	// the BFF's core guarantee.
	ClientAuthMethod string `koanf:"client_auth_method"`

	RedirectURL           string `koanf:"redirect_url"` // must equal the IDP-registered redirect, points at /api/auth/callback
	PostLogoutRedirectURL string `koanf:"post_logout_redirect_url"`
	Scopes                string `koanf:"scope"` // space-separated
	// Resource is an optional RFC 8707 resource indicator. When set, it is sent
	// during authorization, code exchange, and refresh so the IdP issues an
	// access token for the intended upstream API.
	Resource string `koanf:"resource"`
}

// ClaimMappingConfig is [api_control_plane.auth.claim_mappings]: which claim
// names the BFF reads for each user/org field. Values may be dotted paths (e.g.
// "realm_access.roles") to reach a nested claim — plain names are just a
// one-segment path. It mirrors the Platform API's own [auth.claim_mappings] key
// for key, and the two must agree, because it applies to BOTH auth modes: OIDC
// tokens from the configured IdP, and the JWTs Platform API's file-based login
// endpoint signs with these same mapped claim names.
type ClaimMappingConfig struct {
	Username  string `koanf:"username"`
	Email     string `koanf:"email"`
	Roles     string `koanf:"roles"`
	Scope     string `koanf:"scope"`
	OrgID     string `koanf:"organization"`
	OrgName   string `koanf:"org_name"`
	OrgHandle string `koanf:"org_handle"`
}

// CSRFHeaderName is the header the SPA must set on every state-mutating request,
// and the BFF checks for on the way in. It is a fixed contract between the BFF
// and the SPA it ships, not a deployment concern — an operator changing it on one
// side without the other would silently break CSRF protection, so it is a
// constant rather than a config key. The SPA's copy must be kept in sync with
// this value.
const CSRFHeaderName = "X-Requested-By"

// defaultOIDCScopes requests every ap:* permission Platform API authorizes
// against, so a logged-in user's access token carries them all. The IDP must
// have these scopes registered and granted to the user, otherwise it drops the
// ungranted ones. Override with the [auth.oidc] scope config key to request a
// narrower set — a deployment against an IdP that cannot grant this many scopes
// (or any ap:* scope at all) MUST override this rather than silently losing
// access to scope-gated features.
//
// offline_access is required: without it most IDPs issue no refresh token, so
// the BFF cannot silently renew the access token and the user is logged out the
// moment it expires. Keep it in any override.
const defaultOIDCScopes = "openid profile email offline_access" +
	" ap:organization:read ap:organization:manage ap:organization:subscription:read" +
	" ap:project:read ap:project:create ap:project:update ap:project:delete ap:project:manage" +
	" ap:application:read ap:application:create ap:application:update ap:application:delete ap:application:manage" +
	" ap:application:api_key:read ap:application:api_key:create ap:application:api_key:delete ap:application:api_key:manage" +
	" ap:gateway:read ap:gateway:create ap:gateway:update ap:gateway:delete ap:gateway:manage" +
	" ap:gateway:token:read ap:gateway:token:create ap:gateway:token:delete ap:gateway:token:manage" +
	" ap:rest_api:read ap:rest_api:create ap:rest_api:update ap:rest_api:delete ap:rest_api:manage ap:rest_api:import" +
	" ap:rest_api:deployment:read ap:rest_api:deployment:create ap:rest_api:deployment:delete ap:rest_api:deployment:manage" +
	" ap:rest_api:api_key:read ap:rest_api:api_key:create ap:rest_api:api_key:update ap:rest_api:api_key:delete ap:rest_api:api_key:manage" +
	" ap:subscription:read ap:subscription:create ap:subscription:update ap:subscription:delete ap:subscription:manage" +
	" ap:subscription_plan:read ap:subscription_plan:create ap:subscription_plan:update ap:subscription_plan:delete ap:subscription_plan:manage" +
	" ap:secret:read ap:secret:create ap:secret:update ap:secret:delete ap:secret:manage"

// Load resolves configuration from one or more config.toml files. At least one
// path is required and each must exist and parse — there is no default path and
// no silent fallback to built-in defaults. Files are merged in the order given
// with last-wins precedence. It loads defaults, overlays the merged file(s) (with
// their {{ env }} / {{ file }} tokens expanded), normalizes derived fields, then
// validates.
func Load(paths ...string) (*Config, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("at least one config file path is required")
	}
	k, err := loadConfigKoanf(paths...)
	if err != nil {
		return nil, err
	}

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

	cfg.RuntimeConfig = buildRuntimeConfig(cfg)
	return cfg, nil
}

// normalize resolves the derived fields that are not a straight copy of a config
// key: case-folding (level/format/mode), trimming trailing slashes off
// URLs/prefixes, the oidc-mode-implies-enabled rule, and Issuer defaulting to
// Authority.
func (c *Config) normalize() {
	c.Logging.Level = strings.ToLower(c.Logging.Level)
	c.Logging.Format = strings.ToLower(c.Logging.Format)
	c.Auth.Mode = strings.ToLower(c.Auth.Mode)

	c.ControlPlane.URL = strings.TrimRight(c.ControlPlane.URL, "/")
	c.ControlPlane.PortalBasePath = strings.TrimRight(c.ControlPlane.PortalBasePath, "/")
	c.ControlPlane.ProxyPrefix = strings.TrimRight(c.ControlPlane.ProxyPrefix, "/")
	for i := range c.ControlPlane.Upstreams {
		c.ControlPlane.Upstreams[i].URL = strings.TrimRight(c.ControlPlane.Upstreams[i].URL, "/")
		c.ControlPlane.Upstreams[i].Prefix = strings.TrimRight(c.ControlPlane.Upstreams[i].Prefix, "/")
	}

	c.Auth.OIDC.Authority = strings.TrimRight(c.Auth.OIDC.Authority, "/")
	// oidc mode implies the client is enabled even if the explicit flag is unset,
	// so a typo'd mode cannot silently degrade to basic auth.
	c.Auth.OIDC.Enabled = c.Auth.OIDC.Enabled || c.Auth.Mode == "oidc"
	if c.Auth.OIDC.Issuer == "" {
		c.Auth.OIDC.Issuer = c.Auth.OIDC.Authority
	}
	if c.Auth.OIDC.ClientAuthMethod == "" {
		c.Auth.OIDC.ClientAuthMethod = "client_secret_post"
	}
}

// validate fails startup on any value that would otherwise surface as a
// confusing runtime error (a bad port, an empty upstream URL, an incomplete OIDC
// set) and warns on security-relevant downgrades.
func (c *Config) validate() error {
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
	if c.Session.IdleTimeout <= 0 {
		return fmt.Errorf("[session] idle_timeout must be positive, got %s", c.Session.IdleTimeout)
	}
	if c.Session.AbsoluteTTL <= 0 {
		return fmt.Errorf("[session] absolute_ttl must be positive, got %s", c.Session.AbsoluteTTL)
	}
	if c.Session.Cookie.SameSite != "lax" && c.Session.Cookie.SameSite != "strict" && c.Session.Cookie.SameSite != "none" {
		return fmt.Errorf("[session.cookie] same_site must be \"lax\", \"strict\", or \"none\", got %q", c.Session.Cookie.SameSite)
	}
	if c.Session.Cookie.SameSite == "none" && !c.Session.Cookie.Secure {
		return fmt.Errorf("[session.cookie] same_site = \"none\" requires secure = true (browsers reject SameSite=None cookies without Secure)")
	}
	if !c.Session.Cookie.Secure {
		slog.Warn("[session.cookie] secure = false — the session cookie will be sent over plain HTTP. " +
			"This is expected for local development only; set secure = true for any deployment reachable over a network.")
	}

	if err := validateUpstream("control_plane", c.ControlPlane.URL, c.ControlPlane.CAFile, c.ControlPlane.TLSSkipVerify); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, u := range c.ControlPlane.Upstreams {
		if u.Name == "" {
			return fmt.Errorf("[[control_plane.upstreams]] entry is missing a name")
		}
		if seen[u.Name] {
			return fmt.Errorf("[[control_plane.upstreams]] name %q is configured more than once", u.Name)
		}
		seen[u.Name] = true
		if err := validateUpstream(fmt.Sprintf("control_plane.upstreams[%s]", u.Name), u.URL, u.CAFile, u.TLSSkipVerify); err != nil {
			return err
		}
	}

	if c.Auth.OIDC.Enabled {
		if c.Auth.OIDC.ClientID == "" || c.Auth.OIDC.RedirectURL == "" {
			return fmt.Errorf("OIDC mode requires [auth.oidc] client_id and redirect_url")
		}
		switch c.Auth.OIDC.ClientAuthMethod {
		case "client_secret_post", "client_secret_basic":
			if c.Auth.OIDC.ClientSecret == "" {
				return fmt.Errorf("[auth.oidc] client_auth_method %q requires client_secret", c.Auth.OIDC.ClientAuthMethod)
			}
		case "none":
			// A public client driven server-side; PKCE alone protects the flow.
		default:
			return fmt.Errorf("[auth.oidc] client_auth_method must be \"client_secret_post\", \"client_secret_basic\", or \"none\", got %q",
				c.Auth.OIDC.ClientAuthMethod)
		}
		if c.Auth.OIDC.Discovery {
			if c.Auth.OIDC.Authority == "" {
				return fmt.Errorf("[auth.oidc] discovery = true requires authority to be set")
			}
		} else if c.Auth.OIDC.AuthorizationEndpoint == "" || c.Auth.OIDC.TokenEndpoint == "" {
			return fmt.Errorf("[auth.oidc] discovery = false requires authorization_endpoint and token_endpoint to be set explicitly")
		}
	}
	if c.Auth.OIDC.PostLogoutRedirectURL != "" {
		if err := validateAbsoluteURL("[auth.oidc] post_logout_redirect_url", c.Auth.OIDC.PostLogoutRedirectURL); err != nil {
			return err
		}
	}

	if !c.Auth.OIDC.Enabled {
		slog.Warn("basic (file-based) auth is enabled — this is the default, no-external-IdP mode. " +
			"Configure OIDC (set [auth] mode = \"oidc\" and the [auth.oidc] client settings) for an external IdP.")
	}

	return nil
}

func validateUpstream(label, rawURL, caFile string, skipVerify bool) error {
	if rawURL == "" {
		return fmt.Errorf("[%s] url is required: set it in config.toml, "+
			"either as a literal or via an {{ env }} / {{ file }} token", label)
	}
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("[%s] url must be an absolute http:// or https:// URL, got %q", label, rawURL)
	}
	if u.Scheme == "http" && (caFile != "" || skipVerify) {
		return fmt.Errorf("[%s] ca_file / tls_skip_verify are set but url is http:// (no TLS on this hop)", label)
	}
	if u.Scheme == "https" && skipVerify {
		slog.Warn(fmt.Sprintf("[%s] tls_skip_verify = true — upstream certificate verification is DISABLED. "+
			"Trust the upstream certificate with ca_file instead.", label))
	}
	return nil
}

func validateAbsoluteURL(label, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("%s must be an absolute http:// or https:// URL, got %q", label, rawURL)
	}
	return nil
}
