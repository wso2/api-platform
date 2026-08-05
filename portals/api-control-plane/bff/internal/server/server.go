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

// Package server wires the BFF HTTP surface: the file-based / OIDC auth
// endpoints and session lifecycle today; the same-origin reverse proxy to the
// Platform API and static SPA serving land in a follow-up commit.
package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"api-control-plane-bff/internal/auth"
	"api-control-plane-bff/internal/config"
	"api-control-plane-bff/internal/session"
)

// refreshLock is the single-flight coordinator for refreshing one access
// token. The first caller performs the refresh and records the outcome;
// concurrent waiters keyed by the same (old) token read the cached result
// instead of re-reading the store, whose old entry has since been
// re-keyed/deleted.
type refreshLock struct {
	sync.Mutex
	done   bool
	result *session.Session
	err    error
}

// Server holds the BFF dependencies and HTTP handler.
type Server struct {
	cfg       *config.Config
	claims    session.ClaimMapping
	store     session.Store
	fileBased *auth.FileBased
	oidc      *auth.OIDC
	handler   http.Handler

	refreshMu    sync.Mutex
	refreshLocks map[string]*refreshLock
}

// New builds a Server from config. It creates the upstream HTTP client, the
// session store, the file-based authenticator, and (when enabled) the OIDC
// authenticator — discovering the IDP endpoints up front when discovery is
// configured.
func New(ctx context.Context, cfg *config.Config) (*Server, error) {
	transport, err := newUpstreamTransport(cfg.ControlPlane.CAFile, cfg.ControlPlane.TLSSkipVerify)
	if err != nil {
		return nil, err
	}
	upstream := &http.Client{Transport: transport, Timeout: 60 * time.Second}

	// Shared by both auth modes: OIDC tokens from the configured IDP, and the
	// tokens the Platform API's file-based login endpoint signs with these same
	// mapped claim names. Building it once keeps the two readers from drifting
	// apart.
	claims := buildClaimMapping(cfg.Auth.ClaimMappings)

	s := &Server{
		cfg:          cfg,
		claims:       claims,
		fileBased:    auth.NewFileBased(upstream, cfg.ControlPlane.URL, cfg.ControlPlane.PortalBasePath, cfg.Session.AbsoluteTTL, claims),
		refreshLocks: make(map[string]*refreshLock),
	}

	if cfg.Auth.OIDC.Enabled {
		// The session store exists only to hold OIDC refresh/id tokens for
		// renewal. File-based sessions are fully self-contained in the cookie.
		s.store = session.NewMemoryStore()
		o, err := auth.NewOIDC(ctx, upstream, auth.OIDCOptions{
			Authority:             cfg.Auth.OIDC.Authority,
			Issuer:                cfg.Auth.OIDC.Issuer,
			Discovery:             cfg.Auth.OIDC.Discovery,
			AuthorizationEndpoint: cfg.Auth.OIDC.AuthorizationEndpoint,
			TokenEndpoint:         cfg.Auth.OIDC.TokenEndpoint,
			EndSessionEndpoint:    cfg.Auth.OIDC.EndSessionEndpoint,
			ClientID:              cfg.Auth.OIDC.ClientID,
			ClientSecret:          cfg.Auth.OIDC.ClientSecret,
			ClientAuthMethod:      cfg.Auth.OIDC.ClientAuthMethod,
			RedirectURL:           cfg.Auth.OIDC.RedirectURL,
			PostLogoutRedirectURL: cfg.Auth.OIDC.PostLogoutRedirectURL,
			Scopes:                cfg.Auth.OIDC.Scopes,
		}, claims, cfg.Session.AbsoluteTTL)
		if err != nil {
			return nil, err
		}
		s.oidc = o
	}

	s.handler = s.routes()
	return s, nil
}

// Handler returns the fully-wired HTTP handler (for the listener and for tests).
func (s *Server) Handler() http.Handler { return s.handler }

// Close releases background resources (the OIDC transaction sweeper and, when
// enabled, the session store's eviction sweeper).
func (s *Server) Close() error {
	if s.oidc != nil {
		s.oidc.Close()
	}
	if s.store != nil {
		return s.store.Close()
	}
	return nil
}

// routes builds the mux and wraps it with the global middleware chain. The
// reverse-proxy and static-SPA routes are added by a follow-up commit; until
// then, requests outside this set 404.
func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// Health (no auth, no CSRF).
	mux.HandleFunc("GET /healthz", handleHealth)

	// Runtime config consumed by the SPA before app init.
	mux.HandleFunc("GET /api-platform.env.config.js", s.handleRuntimeConfig)

	// Auth endpoints.
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
	mux.HandleFunc("GET /api/session", s.handleSession)
	mux.HandleFunc("GET /api/auth/login", s.handleOIDCLogin)
	mux.HandleFunc("GET /api/auth/callback", s.handleOIDCCallback)

	return chain(mux,
		recoverPanic,
		requestID,
		logRequests,
		s.securityHeaders,
		s.requireCSRF,
	)
}

// newUpstreamTransport builds the HTTP transport used for every outbound call
// to the control plane / IdP. caFile, when set, is appended to the system root
// pool (never replaces it); skipVerify is a last-resort dev/demo escape hatch.
// Both come from validated config (Config.validate already rejects
// tls_skip_verify/ca_file paired with a non-TLS scheme).
func newUpstreamTransport(caFile string, skipVerify bool) (*http.Transport, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if !skipVerify && caFile == "" {
		return transport, nil
	}

	tlsConfig := &tls.Config{InsecureSkipVerify: skipVerify} //nolint:gosec // explicit, config-gated dev/demo opt-out only
	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read ca_file %q: %w", caFile, err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("ca_file %q contains no usable certificates", caFile)
		}
		tlsConfig.RootCAs = pool
	}
	transport.TLSClientConfig = tlsConfig
	return transport, nil
}

// buildClaimMapping builds the claim mapping shared by both auth modes from
// config. Each field overrides the session-package default only when set, so
// an operator can point a single claim (e.g. the display name) at the right
// key without re-specifying the rest.
func buildClaimMapping(c config.ClaimMappingConfig) session.ClaimMapping {
	m := session.DefaultClaimMapping()
	if c.Username != "" {
		m.Username = c.Username
	}
	if c.Email != "" {
		m.Email = c.Email
	}
	if c.Roles != "" {
		m.Roles = c.Roles
	}
	if c.Scope != "" {
		m.Scope = c.Scope
	}
	if c.OrgID != "" {
		m.OrgID = c.OrgID
	}
	if c.OrgName != "" {
		m.OrgName = c.OrgName
	}
	if c.OrgHandle != "" {
		m.OrgHandle = c.OrgHandle
	}
	return m
}
