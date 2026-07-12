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

// Package server wires the BFF HTTP surface: static SPA serving, the same-origin
// reverse proxy to the Platform API, and the file-based / OIDC auth endpoints.
package server

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"ai-workspace-bff/internal/auth"
	"ai-workspace-bff/internal/config"
	"ai-workspace-bff/internal/proxy"
	"ai-workspace-bff/internal/session"
)

// refreshLock is the single-flight coordinator for refreshing one access JWT.
// The first caller performs the refresh and records the outcome; concurrent
// waiters keyed by the same (old) JWT read the cached result instead of
// re-reading the store, whose old entry has since been re-keyed/deleted.
type refreshLock struct {
	sync.Mutex
	done   bool
	result *session.Session
	err    error
}

// Server holds the BFF dependencies and HTTP handler.
type Server struct {
	cfg       *config.Config
	store     session.Store
	fileBased *auth.FileBased
	oidc      *auth.OIDC
	proxy     *httputil.ReverseProxy
	handler   http.Handler

	refreshMu    sync.Mutex
	refreshLocks map[string]*refreshLock
}

// New builds a Server from config. It creates the upstream HTTP client, the
// session store, the file-based authenticator, and (when enabled) the OIDC
// authenticator — discovering the IDP endpoints up front.
func New(ctx context.Context, cfg *config.Config) (*Server, error) {
	target, err := url.Parse(cfg.PlatformAPI.URL)
	if err != nil {
		return nil, err
	}

	transport, err := proxy.NewTransport(proxy.TLSClientOptions{
		CAFile:     cfg.PlatformAPI.CAFile,
		SkipVerify: cfg.PlatformAPI.TLSSkipVerify,
	})
	if err != nil {
		return nil, err
	}
	upstream := &http.Client{Transport: transport, Timeout: 60 * time.Second}

	s := &Server{
		cfg:          cfg,
		fileBased:    auth.NewFileBased(upstream, cfg.PlatformAPI.URL, cfg.PlatformAPI.LoginPath, cfg.Session.AbsoluteTTL),
		proxy:        proxy.ReverseProxy(target, cfg.ProxyPrefix, transport),
		refreshLocks: make(map[string]*refreshLock),
	}

	if cfg.OIDC.Enabled {
		// The session store exists only to hold OIDC refresh/id tokens for renewal.
		// File-based sessions are fully self-contained in the cookie JWT.
		s.store = session.NewMemoryStore()
		o, err := auth.NewOIDC(
			ctx, upstream,
			cfg.OIDC.Issuer, cfg.OIDC.ClientID, cfg.OIDC.ClientSecret,
			cfg.OIDC.RedirectURL, cfg.OIDC.PostLogoutRedirectURL, cfg.OIDC.Scopes,
			oidcClaimMapping(cfg.OIDC.Claims), cfg.Session.AbsoluteTTL,
		)
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

// Close releases background resources (session sweeper and, when enabled, the
// OIDC transaction sweeper).
func (s *Server) Close() error {
	if s.oidc != nil {
		s.oidc.Close()
	}
	if s.store != nil {
		return s.store.Close()
	}
	return nil
}

// oidcClaimMapping builds the claim mapping for OIDC tokens from config. Each
// field overrides the session-package default only when set, so an operator can
// point a single claim (e.g. the display name) at the right key via the
// OIDC_CLAIM_* env vars without re-specifying the rest.
func oidcClaimMapping(c config.ClaimMappingConfig) session.ClaimMapping {
	m := session.DefaultClaimMapping()
	if c.Username != "" {
		m.Username = c.Username
	}
	if c.Email != "" {
		m.Email = c.Email
	}
	if c.Role != "" {
		m.Role = c.Role
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
