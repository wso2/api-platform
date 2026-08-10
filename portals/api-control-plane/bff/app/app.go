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

// Package app is api-control-plane-bff's public surface for a host binary
// that wants to embed and extend it — the Go equivalent of api-control-
// plane's own src/index.ts on the frontend side (see
// docs/remote-app-wrapper-app-architecture.md there). Every other package in
// this module is under internal/ and can never be imported from outside this
// module's own tree, by Go's own import-visibility rule — this package is
// the only one a host module (e.g. cloud-control-plane's own BFF) may
// import. Keep this file's surface minimal: it should only ever grow to
// support a concrete extension need, never speculatively.
package app

import (
	"context"
	"net/http"

	"api-control-plane-bff/internal/config"
	"api-control-plane-bff/internal/server"
	"api-control-plane-bff/internal/session"
)

// Config is api-control-plane-bff's full configuration shape (koanf-loaded
// TOML + APIP_ACP_ env overlay) — identical to what the standalone binary
// loads. A host typically points LoadConfig at its own config.toml with
// cloud-specific [auth.oidc] / [control_plane.upstreams] sections.
type Config = config.Config

// LoadConfig loads and validates a Config exactly as the standalone main.go
// does (same koanf sources, same env prefix, same defaults, same
// [server.http]/[server.https] validation).
func LoadConfig(paths ...string) (*Config, error) { return config.Load(paths...) }

// SessionUser is the resolved caller identity available via
// SessionFromContext — the same shape GET /api/session returns to the
// browser (name, email, role, scopes, and org when present).
type SessionUser = session.User

// SessionFromContext returns the caller's resolved session for this request,
// if it carried a valid, unexpired session cookie. Works identically for a
// default route and for an Options.ExtraRoutes handler — both run behind the
// same session-resolving middleware (see internal/server/middleware.go's
// sessionContext). ok is false for an unauthenticated request; a handler
// that requires auth must check ok itself; this function makes no
// authorization decision on its own.
func SessionFromContext(ctx context.Context) (SessionUser, bool) {
	return session.FromContext(ctx)
}

// Options extends the BFF's default route set. See
// internal/server/options.go for the full field-by-field contract
// (ExtraRoutes, DisabledRoutes, RouteOverrides, WrapRoute). The zero value
// reproduces standalone behavior exactly.
type Options = server.Options

// Server is the subset of the BFF's lifecycle a host needs: the fully-wired
// http.Handler, and Close to release background resources on shutdown.
// Declared as an interface here — rather than re-exporting *server.Server
// directly — so this package's public signatures never spell out an
// internal type name; *server.Server already satisfies it.
type Server interface {
	Handler() http.Handler
	Close() error
}

// New builds the BFF exactly as the standalone binary does, with opts
// layered on top: opts.ExtraRoutes are registered alongside the default
// route set, opts.DisabledRoutes are skipped, opts.RouteOverrides/WrapRoute
// replace or wrap a default route's handler. Every extra/overriding handler
// runs inside the same middleware chain (CSRF, security headers, session
// resolution) as every default route — there is no second auth path for a
// host to implement.
func New(ctx context.Context, cfg *Config, opts Options) (Server, error) {
	return server.New(ctx, cfg, opts)
}
