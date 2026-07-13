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

// Package config loads BFF configuration from a flat config.toml and APIP_AIW_*
// environment variables (env wins), resolving {{ env }} / {{ file }} interpolation
// tokens through the shared configinterpolate library. Browser-safe keys are
// surfaced to the SPA as APIP_AIW_* runtime config. The BFF never validates tokens, so
// there are no signing keys here — only the IDP client credentials needed to
// perform the OAuth2 code exchange.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// Config is the fully-resolved BFF configuration.
type Config struct {
	// Listener
	Addr      string // host:port to listen on, e.g. ":5380"
	StaticDir string // directory containing the built SPA (index.html + assets)

	// Logging
	LogLevel  string // "debug" | "info" | "warn" | "error" (default "info")
	LogFormat string // "text" | "json" (default "text")

	// TLS for the BFF listener
	TLS TLSConfig

	// Upstream Platform API
	PlatformAPI PlatformAPIConfig

	// Same-origin reverse-proxy prefix the SPA calls (stripped before forwarding)
	ProxyPrefix string

	// Session / cookie
	Session SessionConfig
	Cookie  CookieConfig

	// CSRF
	CSRFHeader string // custom header required on state-mutating requests

	// Auth
	AuthMode string // "basic" | "oidc" — informs the SPA which login UX to show
	OIDC     OIDCConfig

	// DemoMode mirrors Platform API's APIP_DEMO_MODE: defaults to true, and an
	// explicit "false"/"0" opts into production-grade startup checks (no
	// file-based/basic auth, no auto-generated self-signed TLS certificate).
	DemoMode bool

	// Runtime config surfaced to the SPA (window.__RUNTIME_CONFIG__)
	RuntimeConfig map[string]string
}

// PlatformAPIConfig groups everything about the upstream Platform API hop: where
// it is, and how its TLS certificate is trusted.
type PlatformAPIConfig struct {
	// URL is the base URL, e.g. https://platform-api:9243. Its http/https scheme is
	// the single source of truth for whether the outbound hop uses TLS — there is
	// deliberately no separate boolean, since that could contradict the URL.
	URL string
	// CAFile is a PEM bundle to trust for the upstream's TLS certificate. It is
	// appended to the system roots rather than replacing them, so public CAs keep
	// working; leaving it empty simply uses the OS trust store on its own. Set it to
	// accept a private/self-signed Platform API cert with verification still on.
	// Ignored when TLSSkipVerify is true.
	CAFile string
	// TLSSkipVerify disables upstream certificate verification entirely.
	// Last-resort escape hatch for dev/demo only; prefer CAFile.
	TLSSkipVerify bool
	// LoginPath is the file-based login path on the Platform API.
	LoginPath string
}

// TLSConfig controls whether the BFF listener serves HTTPS directly or sits
// behind a component that terminates TLS on its behalf.
type TLSConfig struct {
	// TerminateTLS makes the BFF serve HTTPS on its own listener: it presents the
	// certificate and decrypts inbound TLS itself. Defaults to true (config key
	// tls_enabled). Set to false only when a trusted upstream (ingress,
	// service-mesh sidecar) terminates TLS and forwards plain HTTP to the BFF; no
	// certificate is then read, generated, or required.
	TerminateTLS bool
	SelfSigned   bool
	CertFile     string
	KeyFile      string
}

// SessionConfig controls server-side session lifetime.
type SessionConfig struct {
	Store       string        // "memory" (default) | "redis" (future)
	IdleTimeout time.Duration // sliding idle window
	AbsoluteTTL time.Duration // hard cap regardless of activity / token exp
}

// CookieConfig controls the session cookie attributes.
type CookieConfig struct {
	Name     string
	Secure   bool
	SameSite string // "lax" | "strict" | "none"
}

// OIDCConfig holds the confidential-client settings. The client secret lives
// only here on the BFF and is never emitted to the browser.
type OIDCConfig struct {
	Enabled               bool
	Issuer                string // discovery base; {issuer}/.well-known/openid-configuration
	ClientID              string
	ClientSecret          string
	RedirectURL           string // must equal the IDP-registered redirect, points at /api/auth/callback
	PostLogoutRedirectURL string
	Scopes                string // space-separated

	// Claims maps which token claim names carry each user/org field. Override per
	// IDP when the defaults don't match (e.g. the display name lands on "sub").
	Claims ClaimMappingConfig
}

// ClaimMappingConfig configures which claim names the BFF reads for each
// user/org field from the OIDC tokens. Empty fields fall back to built-in
// defaults in the session package.
type ClaimMappingConfig struct {
	Username  string
	Email     string
	Role      string
	Scope     string
	OrgID     string
	OrgName   string
	OrgHandle string
}

// defaultOIDCScopes is the full set of scopes the BFF requests in OIDC mode so a
// logged-in user's access token carries every ap:* permission the Platform API
// authorizes against. The IDP must still have these scopes registered and granted
// to the user, otherwise it drops the ungranted ones. Override with the oidc_scope
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

// configFileEnv names the environment variable that points at the config file.
// It is read before the config file exists, so it cannot itself be a config key.
const configFileEnv = "APIP_AIW_CONFIG_FILE"

// defaultConfigFile is where the container mounts config.toml.
const defaultConfigFile = "/etc/ai-workspace/config.toml"

// Load resolves configuration from config.toml (if present) and APIP_AIW_*
// environment variables, which always win over the config file. Interpolation
// tokens ({{ env }} / {{ file }}) in either source are resolved first, so any key
// — the OIDC client secret in particular — can be pulled from an environment
// variable or a mounted secret file instead of being written in the clear.
func Load() (*Config, error) {
	tomlPath := defaultConfigFile
	if v := os.Getenv(configFileEnv); v != "" {
		tomlPath = v
	}
	s, err := loadSettings(tomlPath)
	if err != nil {
		return nil, err
	}

	authMode := strings.ToLower(s.get("auth_mode", "basic"))

	// Parse typed values up front so a malformed one fails startup instead of
	// being silently replaced with the default.
	selfSigned, err := s.getbool("tls_self_signed", true)
	if err != nil {
		return nil, err
	}
	tlsEnabled, err := s.getbool("tls_enabled", true)
	if err != nil {
		return nil, err
	}
	platformTLSSkipVerify, err := s.getbool("platform_api_tls_skip_verify", false)
	if err != nil {
		return nil, err
	}
	idleTimeout, err := s.getdur("session_idle_timeout", 30*time.Minute)
	if err != nil {
		return nil, err
	}
	absoluteTTL, err := s.getdur("session_absolute_ttl", 8*time.Hour)
	if err != nil {
		return nil, err
	}
	cookieSecure, err := s.getbool("cookie_secure", true)
	if err != nil {
		return nil, err
	}
	oidcEnabled, err := s.getbool("oidc_enabled", false)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Addr:      s.get("listen_addr", ":5380"),
		StaticDir: s.get("static_dir", "/app"),
		LogLevel:  strings.ToLower(s.get("log_level", "info")),
		LogFormat: strings.ToLower(s.get("log_format", "text")),
		TLS: TLSConfig{
			TerminateTLS: tlsEnabled,
			SelfSigned:   selfSigned,
			// Convention matches the container's mount path. buildTLS falls back to
			// a self-signed cert when these files are absent.
			CertFile: s.get("tls_cert_file", "/etc/ai-workspace/tls/tls.crt"),
			KeyFile:  s.get("tls_key_file", "/etc/ai-workspace/tls/tls.key"),
		},
		PlatformAPI: PlatformAPIConfig{
			URL:           strings.TrimRight(s.get("platform_api_url", ""), "/"),
			CAFile:        s.get("platform_api_ca_file", ""),
			TLSSkipVerify: platformTLSSkipVerify,
			LoginPath:     s.get("platform_login_path", "/api/portal/v0.9/auth/login"),
		},
		ProxyPrefix: strings.TrimRight(s.get("proxy_prefix", "/api/proxy"), "/"),
		Session: SessionConfig{
			Store:       s.get("session_store", "memory"),
			IdleTimeout: idleTimeout,
			AbsoluteTTL: absoluteTTL,
		},
		Cookie: CookieConfig{
			Name:     s.get("cookie_name", "_ai_workspace_session"),
			Secure:   cookieSecure,
			SameSite: strings.ToLower(s.get("cookie_samesite", "lax")),
		},
		CSRFHeader: s.get("csrf_header", "X-Requested-By"),
		AuthMode:   authMode,
		DemoMode:   demoMode(),
		OIDC: OIDCConfig{
			Enabled:  authMode == "oidc" || oidcEnabled,
			Issuer:   strings.TrimRight(s.get("oidc_authority", ""), "/"),
			ClientID: s.get("oidc_client_id", ""),
			// Never write the secret itself into config.toml: set it via the
			// APIP_AIW_OIDC_CLIENT_SECRET env var, or — preferably — read it from a
			// mounted file with '{{ file "/secrets/ai-workspace/oidc_client_secret" }}'.
			ClientSecret: s.get("oidc_client_secret", ""),
			RedirectURL:  s.get("oidc_redirect_url", ""),
			// Empty by default: LogoutURL() forwards this as post_logout_redirect_uri,
			// which IDPs require to be an absolute, pre-registered URL. A relative
			// default would produce an invalid logout request, so leave it unset
			// unless an absolute URL is explicitly configured.
			PostLogoutRedirectURL: s.get("oidc_post_logout_redirect_url", ""),
			Scopes:                s.get("oidc_scope", defaultOIDCScopes),
			// The same claim-name keys drive both the BFF's session mapping and the
			// SPA's runtime config, so one config entry keeps both layers in sync.
			Claims: ClaimMappingConfig{
				Username:  s.get("oidc_username_claim", "username"),
				Email:     s.get("oidc_email_claim", "email"),
				Role:      s.get("oidc_role_claim", "platform_role"),
				Scope:     s.get("oidc_scope_claim", "scope"),
				OrgID:     s.get("oidc_org_id_claim", "org_id"),
				OrgName:   s.get("oidc_org_name_claim", "org_name"),
				OrgHandle: s.get("oidc_org_handle_claim", "org_handle"),
			},
		},
	}

	if cfg.PlatformAPI.URL == "" {
		return nil, fmt.Errorf("platform_api_url is required (set it in config.toml or via %sPLATFORM_API_URL)", EnvPrefix)
	}
	// The scheme is the single source of truth for the outbound TLS decision, so a
	// missing/typo'd scheme must fail at startup rather than surface as an opaque
	// dial error on the first proxied request.
	u, err := url.Parse(cfg.PlatformAPI.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("platform_api_url must be an absolute http:// or https:// URL, got %q", cfg.PlatformAPI.URL)
	}
	// Trust knobs only apply to an https upstream; flag them on a plain-http URL so a
	// mistaken belief that TLS is in effect is caught early.
	if u.Scheme == "http" {
		if cfg.PlatformAPI.CAFile != "" || cfg.PlatformAPI.TLSSkipVerify {
			return nil, fmt.Errorf("platform_api_ca_file / platform_api_tls_skip_verify are set but platform_api_url is http:// (no TLS on the upstream hop)")
		}
	}
	// Skipping verification outside demo mode is a security downgrade; require an
	// operator to reach it deliberately rather than inheriting it silently.
	if u.Scheme == "https" && cfg.PlatformAPI.TLSSkipVerify && !cfg.DemoMode {
		return nil, fmt.Errorf("platform_api_tls_skip_verify = true is not allowed while APIP_DEMO_MODE=false; " +
			"trust the upstream certificate with platform_api_ca_file instead")
	}
	if cfg.OIDC.Enabled {
		if cfg.OIDC.Issuer == "" || cfg.OIDC.ClientID == "" || cfg.OIDC.ClientSecret == "" || cfg.OIDC.RedirectURL == "" {
			return nil, fmt.Errorf("OIDC mode requires oidc_authority, oidc_client_id, oidc_client_secret and oidc_redirect_url")
		}
	}

	// Outside demo mode, basic (file-based) auth is not allowed — it relies on the
	// Platform API's built-in admin/admin credentials and is dev-only.
	if !cfg.DemoMode && !cfg.OIDC.Enabled {
		return nil, fmt.Errorf("APIP_DEMO_MODE=false does not allow basic (file-based) auth; " +
			"configure OIDC (set auth_mode = \"oidc\" and oidc_authority, oidc_client_id, oidc_client_secret, oidc_redirect_url)")
	}

	cfg.RuntimeConfig = buildRuntimeConfig(cfg, s)
	return cfg, nil
}

// demoMode reports whether APIP_DEMO_MODE is enabled. Defaults to true when the
// variable is unset; only an explicit "false"/"0" opts out. It is intentionally
// unprefixed: the same variable drives the Platform API, so one value governs the
// whole stack.
func demoMode() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("APIP_DEMO_MODE")))
	if v == "" {
		return true
	}
	return v == "true" || v == "1"
}
