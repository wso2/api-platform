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
// library. The file is the only source: a key takes its value from the environment or
// a mounted secret file exactly when its token says so. Browser-safe keys are surfaced
// to the SPA as APIP_AIW_* runtime config. The BFF never validates tokens, so there are
// no signing keys here — only the IDP client credentials needed to perform the OAuth2
// code exchange.
package config

import (
	"fmt"
	"log/slog"
	"net/url"
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

	// Upstream Platform API (control plane)
	ControlPlane ControlPlaneConfig

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

	// Runtime config surfaced to the SPA (window.__RUNTIME_CONFIG__)
	RuntimeConfig map[string]string
}

// ControlPlaneConfig groups everything about the upstream Platform API (control
// plane) hop: where it is, and how its TLS certificate is trusted.
type ControlPlaneConfig struct {
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
	// [tls] enabled). Set to false only when a trusted upstream (ingress,
	// service-mesh sidecar) terminates TLS and forwards plain HTTP to the BFF; no
	// certificate is then read, generated, or required.
	TerminateTLS bool
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
// to the user, otherwise it drops the ungranted ones. Override with the [oidc] scope
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
// DefaultConfigFile when path is empty. The file's {{ env }} / {{ file }} tokens are
// expanded first, so any key — the OIDC client secret in particular — can be pulled
// from an environment variable or a mounted secret file instead of being written in
// the clear. A key not present in the file falls back to the default below.
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigFile
	}
	s, err := loadSettings(path)
	if err != nil {
		return nil, err
	}

	authMode := strings.ToLower(s.get("auth_mode", "basic"))
	// A typo'd mode must not silently degrade to basic auth: any value other than
	// "oidc" would leave OIDC.Enabled false and hand the SPA an unknown login UX.
	if authMode != "basic" && authMode != "oidc" {
		return nil, fmt.Errorf("invalid auth_mode %q: must be \"basic\" or \"oidc\"", authMode)
	}

	// Parse typed values up front so a malformed one fails startup instead of
	// being silently replaced with the default.
	tlsEnabled, err := s.getbool("tls.enabled", true)
	if err != nil {
		return nil, err
	}
	controlPlaneTLSSkipVerify, err := s.getbool("control_plane.tls_skip_verify", false)
	if err != nil {
		return nil, err
	}
	idleTimeout, err := s.getdur("session.idle_timeout", 30*time.Minute)
	if err != nil {
		return nil, err
	}
	absoluteTTL, err := s.getdur("session.absolute_ttl", 8*time.Hour)
	if err != nil {
		return nil, err
	}
	cookieSecure, err := s.getbool("cookie.secure", true)
	if err != nil {
		return nil, err
	}
	oidcEnabled, err := s.getbool("oidc.enabled", false)
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
			// Convention matches the container's mount path. A certificate pair is
			// required there whenever TerminateTLS is on.
			CertFile: s.get("tls.cert_file", "/etc/ai-workspace/tls/cert.pem"),
			KeyFile:  s.get("tls.key_file", "/etc/ai-workspace/tls/key.pem"),
		},
		ControlPlane: ControlPlaneConfig{
			URL:           strings.TrimRight(s.get("control_plane.url", ""), "/"),
			CAFile:        s.get("control_plane.ca_file", ""),
			TLSSkipVerify: controlPlaneTLSSkipVerify,
			LoginPath:     s.get("control_plane.login_path", "/api/portal/v0.9/auth/login"),
		},
		ProxyPrefix: strings.TrimRight(s.get("proxy_prefix", "/api/proxy"), "/"),
		Session: SessionConfig{
			Store:       s.get("session.store", "memory"),
			IdleTimeout: idleTimeout,
			AbsoluteTTL: absoluteTTL,
		},
		Cookie: CookieConfig{
			Name:     s.get("cookie.name", "_ai_workspace_session"),
			Secure:   cookieSecure,
			SameSite: strings.ToLower(s.get("cookie.samesite", "lax")),
		},
		CSRFHeader: s.get("csrf_header", "X-Requested-By"),
		AuthMode:   authMode,
		OIDC: OIDCConfig{
			Enabled:  authMode == "oidc" || oidcEnabled,
			Issuer:   strings.TrimRight(s.get("oidc.authority", ""), "/"),
			ClientID: s.get("oidc.client_id", ""),
			// Never write the secret as a literal. Point the key at an environment
			// variable with '{{ env "APIP_AIW_OIDC_CLIENT_SECRET" }}', or — preferably —
			// at a mounted file with '{{ file "/secrets/ai-workspace/oidc_client_secret" }}'.
			ClientSecret: s.get("oidc.client_secret", ""),
			RedirectURL:  s.get("oidc.redirect_url", ""),
			// Empty by default: LogoutURL() forwards this as post_logout_redirect_uri,
			// which IDPs require to be an absolute, pre-registered URL. A relative
			// default would produce an invalid logout request, so leave it unset
			// unless an absolute URL is explicitly configured.
			PostLogoutRedirectURL: s.get("oidc.post_logout_redirect_url", ""),
			Scopes:                s.get("oidc.scope", defaultOIDCScopes),
			// [oidc.claim_mappings] deliberately mirrors the Platform API's
			// [auth.idp.claim_mappings], key for key: both describe the same IDP token,
			// and they must agree, so they are named the same on both sides.
			//
			// The same keys drive the BFF's session mapping and the SPA's runtime
			// config, so one config entry keeps both layers in sync.
			Claims: ClaimMappingConfig{
				Username:  s.get("oidc.claim_mappings.username", "username"),
				Email:     s.get("oidc.claim_mappings.email", "email"),
				Role:      s.get("oidc.claim_mappings.role", "platform_role"),
				Scope:     s.get("oidc.claim_mappings.scope", "scope"),
				OrgID:     s.get("oidc.claim_mappings.organization", "org_id"),
				OrgName:   s.get("oidc.claim_mappings.org_name", "org_name"),
				OrgHandle: s.get("oidc.claim_mappings.org_handle", "org_handle"),
			},
		},
	}

	if cfg.ControlPlane.URL == "" {
		return nil, fmt.Errorf("[control_plane] url is required: set it in config.toml, " +
			"either as a literal or via an {{ env }} / {{ file }} token")
	}
	// The scheme is the single source of truth for the outbound TLS decision, so a
	// missing/typo'd scheme must fail at startup rather than surface as an opaque
	// dial error on the first proxied request.
	u, err := url.Parse(cfg.ControlPlane.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("[control_plane] url must be an absolute http:// or https:// URL, got %q", cfg.ControlPlane.URL)
	}
	// Trust knobs only apply to an https upstream; flag them on a plain-http URL so a
	// mistaken belief that TLS is in effect is caught early.
	if u.Scheme == "http" {
		if cfg.ControlPlane.CAFile != "" || cfg.ControlPlane.TLSSkipVerify {
			return nil, fmt.Errorf("[control_plane] ca_file / tls_skip_verify are set but [control_plane] url is http:// (no TLS on the upstream hop)")
		}
	}
	// Skipping verification is a security downgrade; say so loudly and point at
	// the supported alternative.
	if u.Scheme == "https" && cfg.ControlPlane.TLSSkipVerify {
		slog.Warn("[control_plane] tls_skip_verify = true — upstream certificate verification is DISABLED. " +
			"Trust the upstream certificate with [control_plane] ca_file instead.")
	}
	if cfg.OIDC.Enabled {
		if cfg.OIDC.Issuer == "" || cfg.OIDC.ClientID == "" || cfg.OIDC.ClientSecret == "" || cfg.OIDC.RedirectURL == "" {
			return nil, fmt.Errorf("OIDC mode requires [oidc] authority, client_id, client_secret and redirect_url")
		}
	}
	// Empty is fine (the key is optional), but a relative value would be forwarded as an
	// invalid post_logout_redirect_uri and only fail at logout time — catch it here.
	if cfg.OIDC.PostLogoutRedirectURL != "" {
		u, err := url.Parse(cfg.OIDC.PostLogoutRedirectURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, fmt.Errorf("[oidc] post_logout_redirect_url must be an absolute http:// or https:// URL, got %q",
				cfg.OIDC.PostLogoutRedirectURL)
		}
	}

	// Basic (file-based) auth is supported for quickstart deployments but is not
	// recommended for production; point operators at OIDC.
	if !cfg.OIDC.Enabled {
		slog.Warn("basic (file-based) auth is enabled — this is not recommended for production; " +
			"configure OIDC (set auth_mode = \"oidc\" and [oidc] authority, client_id, client_secret, redirect_url)")
	}

	cfg.RuntimeConfig = buildRuntimeConfig(cfg, s)
	return cfg, nil
}
