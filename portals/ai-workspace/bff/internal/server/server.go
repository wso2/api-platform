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

type refreshLock struct{ sync.Mutex }

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
	target, err := url.Parse(cfg.PlatformAPIURL)
	if err != nil {
		return nil, err
	}

	transport := proxy.NewTransport(cfg.PlatformTLSSkipVerify)
	upstream := &http.Client{Transport: transport, Timeout: 60 * time.Second}

	s := &Server{
		cfg:          cfg,
		store:        session.NewMemoryStore(),
		fileBased:    auth.NewFileBased(upstream, cfg.PlatformAPIURL, cfg.PlatformLoginPath, cfg.Session.AbsoluteTTL),
		proxy:        proxy.ReverseProxy(target, cfg.ProxyPrefix, transport),
		refreshLocks: make(map[string]*refreshLock),
	}

	if cfg.OIDC.Enabled {
		o, err := auth.NewOIDC(
			ctx, upstream,
			cfg.OIDC.Issuer, cfg.OIDC.ClientID, cfg.OIDC.ClientSecret,
			cfg.OIDC.RedirectURL, cfg.OIDC.PostLogoutRedirectURL, cfg.OIDC.Scopes,
			oidcClaimMapping(), cfg.Session.AbsoluteTTL,
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

// Close releases background resources (session sweeper).
func (s *Server) Close() error { return s.store.Close() }

// oidcClaimMapping returns the claim mapping for OIDC tokens. Defaults align with
// the SPA's previous OIDC defaults; org/user keys can be overridden via the same
// VITE_* names if needed in a future iteration.
func oidcClaimMapping() session.ClaimMapping {
	m := session.DefaultClaimMapping()
	m.Username = "given_name"
	m.OrgID = "org_id"
	return m
}
