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
// endpoints and session lifecycle, static SPA serving, and the same-origin
// reverse proxy to the Platform API and any additional named upstream.
package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"api-control-plane-bff/internal/auth"
	"api-control-plane-bff/internal/config"
	"api-control-plane-bff/internal/proxy"
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

// mountedProxy pairs a same-origin prefix with the reverse proxy serving it.
type mountedProxy struct {
	prefix string
	rp     *httputil.ReverseProxy
}

// Server holds the BFF dependencies and HTTP handler.
type Server struct {
	cfg       *config.Config
	claims    session.ClaimMapping
	store     session.Store
	fileBased *auth.FileBased
	oidc      *auth.OIDC
	proxies   []mountedProxy
	handler   http.Handler

	refreshMu    sync.Mutex
	refreshLocks map[string]*refreshLock
}

// New builds a Server from config. It creates the upstream HTTP client, the
// session store, the file-based authenticator, and (when enabled) the OIDC
// authenticator — discovering the IDP endpoints up front when discovery is
// configured.
func New(ctx context.Context, cfg *config.Config) (*Server, error) {
	primaryTransport, err := proxy.NewTransport(proxy.TLSClientOptions{
		CAFile:     cfg.ControlPlane.CAFile,
		SkipVerify: cfg.ControlPlane.TLSSkipVerify,
	})
	if err != nil {
		return nil, err
	}
	upstream := &http.Client{Transport: primaryTransport, Timeout: 60 * time.Second}

	// Shared by both auth modes: OIDC tokens from the configured IDP, and the
	// tokens the Platform API's file-based login endpoint signs with these same
	// mapped claim names. Building it once keeps the two readers from drifting
	// apart.
	claims := buildClaimMapping(cfg.Auth.ClaimMappings)

	primaryTarget, err := url.Parse(cfg.ControlPlane.URL)
	if err != nil {
		return nil, fmt.Errorf("parse control_plane.url: %w", err)
	}
	proxies := []mountedProxy{
		{prefix: cfg.ControlPlane.ProxyPrefix, rp: proxy.ReverseProxy(primaryTarget, cfg.ControlPlane.ProxyPrefix, primaryTransport)},
	}

	// Additional named upstreams (e.g. a billing service) are proxied
	// same-origin alongside the primary one, each with its own transport so a
	// per-upstream TLS trust setting never leaks onto a different upstream.
	// Optional; empty for every deployment that only talks to the primary.
	for _, u := range cfg.ControlPlane.Upstreams {
		upstreamTransport, err := proxy.NewTransport(proxy.TLSClientOptions{
			CAFile:     u.CAFile,
			SkipVerify: u.TLSSkipVerify,
		})
		if err != nil {
			return nil, fmt.Errorf("build transport for upstream %q: %w", u.Name, err)
		}
		target, err := url.Parse(u.URL)
		if err != nil {
			return nil, fmt.Errorf("parse url for upstream %q: %w", u.Name, err)
		}
		prefix := u.Prefix
		if prefix == "" {
			prefix = cfg.ControlPlane.ProxyPrefix + "/" + u.Name
		}
		proxies = append(proxies, mountedProxy{prefix: prefix, rp: proxy.ReverseProxy(target, prefix, upstreamTransport)})
	}

	s := &Server{
		cfg:          cfg,
		claims:       claims,
		fileBased:    auth.NewFileBased(upstream, cfg.ControlPlane.URL, cfg.ControlPlane.PortalBasePath, cfg.Session.AbsoluteTTL, claims),
		proxies:      proxies,
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

// routes builds the mux and wraps it with the global middleware chain.
func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// Health (no auth, no CSRF).
	mux.HandleFunc("GET /healthz", handleHealth)

	// Runtime config consumed by the SPA before app init.
	mux.HandleFunc("GET /api-platform.env.config.js", s.handleRuntimeConfig)
	mux.HandleFunc("GET /api-platform.common.config.js", s.handleCommonConfig)

	// Auth endpoints.
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
	mux.HandleFunc("GET /api/session", s.handleSession)
	mux.HandleFunc("GET /api/auth/login", s.handleOIDCLogin)
	mux.HandleFunc("GET /api/auth/callback", s.handleOIDCCallback)

	// Same-origin reverse proxy(ies): the primary control plane, plus any
	// named upstream. Each Rewrite hook already strips its own prefix, so the
	// subtree is registered directly.
	for _, p := range s.proxies {
		mux.HandleFunc(p.prefix+"/", s.proxyHandler(p.rp))
	}

	// SPA static files + client-side routing fallback. Must be registered
	// last since it's the catch-all for everything not matched above.
	mux.Handle("/", spaHandler(s.cfg.Server.StaticDir))

	return chain(mux,
		recoverPanic,
		requestID,
		logRequests,
		s.securityHeaders,
		s.requireCSRF,
	)
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
