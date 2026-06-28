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

// Package config loads BFF configuration from environment variables (and an
// optional config.toml whose values are surfaced to the SPA as VITE_* runtime
// config). The BFF never validates tokens, so there are no signing keys here —
// only the IDP client credentials needed to perform the OAuth2 code exchange.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the fully-resolved BFF configuration.
type Config struct {
	// Listener
	Addr      string // host:port to listen on, e.g. ":5380"
	StaticDir string // directory containing the built SPA (index.html + assets)

	// TLS for the BFF listener
	TLS TLSConfig

	// Upstream Platform API
	PlatformAPIURL        string // base URL, e.g. https://platform-api:9243
	PlatformTLSSkipVerify bool   // accept the Platform API self-signed cert
	PlatformLoginPath     string // file-based login path on the Platform API

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

// TLSConfig controls how the BFF terminates TLS.
type TLSConfig struct {
	SelfSigned bool
	CertFile   string
	KeyFile    string
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
}

// Load resolves configuration from config.toml (if present) and environment
// variables. Environment variables always win over the config file.
func Load() (*Config, error) {
	// config.toml -> VITE_* env, only filling vars not already set (env wins).
	tomlPath := getenv("BFF_CONFIG_FILE", "/etc/ai-workspace/config.toml")
	applyTOMLToEnv(tomlPath)

	authMode := strings.ToLower(getenv("VITE_AUTH_MODE", getenv("AUTH_MODE", "basic")))

	cfg := &Config{
		Addr:      getenv("BFF_ADDR", ":5380"),
		StaticDir: getenv("STATIC_DIR", "/app"),
		TLS: TLSConfig{
			SelfSigned: getbool("BFF_TLS_SELF_SIGNED", true),
			// Convention matches the legacy entrypoint.sh mount path. buildTLS
			// falls back to a self-signed cert when these files are absent.
			CertFile: getenv("BFF_TLS_CERT_FILE", "/etc/ai-workspace/tls/tls.crt"),
			KeyFile:  getenv("BFF_TLS_KEY_FILE", "/etc/ai-workspace/tls/tls.key"),
		},
		PlatformAPIURL:        strings.TrimRight(getenv("PLATFORM_API_URL", ""), "/"),
		PlatformTLSSkipVerify: getbool("PLATFORM_API_TLS_SKIP_VERIFY", false),
		PlatformLoginPath:     getenv("PLATFORM_LOGIN_PATH", "/api/portal/v1/auth/login"),
		ProxyPrefix:           strings.TrimRight(getenv("PROXY_PREFIX", "/api/proxy"), "/"),
		Session: SessionConfig{
			Store:       getenv("SESSION_STORE", "memory"),
			IdleTimeout: getdur("SESSION_IDLE_TIMEOUT", 30*time.Minute),
			AbsoluteTTL: getdur("SESSION_ABSOLUTE_TTL", 8*time.Hour),
		},
		Cookie: CookieConfig{
			Name:     getenv("COOKIE_NAME", "_bff_session"),
			Secure:   getbool("COOKIE_SECURE", true),
			SameSite: strings.ToLower(getenv("COOKIE_SAMESITE", "lax")),
		},
		CSRFHeader: getenv("CSRF_HEADER", "X-Requested-By"),
		AuthMode:   authMode,
		OIDC: OIDCConfig{
			Enabled:               authMode == "oidc" || getbool("OIDC_ENABLED", false),
			Issuer:                strings.TrimRight(getenv("OIDC_ISSUER", getenv("VITE_OIDC_AUTHORITY", "")), "/"),
			ClientID:              getenv("OIDC_CLIENT_ID", getenv("VITE_OIDC_CLIENT_ID", "")),
			ClientSecret:          getenv("OIDC_CLIENT_SECRET", ""),
			RedirectURL:           getenv("OIDC_REDIRECT_URL", ""),
			PostLogoutRedirectURL: getenv("OIDC_POST_LOGOUT_REDIRECT_URL", "/login"),
			Scopes:                getenv("OIDC_SCOPES", getenv("VITE_OIDC_SCOPE", "openid profile email")),
		},
	}

	if cfg.PlatformAPIURL == "" {
		return nil, fmt.Errorf("PLATFORM_API_URL is required")
	}
	if cfg.OIDC.Enabled {
		if cfg.OIDC.Issuer == "" || cfg.OIDC.ClientID == "" || cfg.OIDC.ClientSecret == "" || cfg.OIDC.RedirectURL == "" {
			return nil, fmt.Errorf("OIDC mode requires OIDC_ISSUER, OIDC_CLIENT_ID, OIDC_CLIENT_SECRET and OIDC_REDIRECT_URL")
		}
	}

	cfg.RuntimeConfig = buildRuntimeConfig(cfg)
	return cfg, nil
}

func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getbool(key string, def bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return b
}

func getdur(key string, def time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	d, err := time.ParseDuration(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return d
}
