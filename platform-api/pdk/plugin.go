/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

// Package pdk ("Plugin Development Kit") is the public contract an external
// extension (for example api-cloud) builds against. It holds only interfaces,
// small value types, and request helpers — no platform logic — and imports only
// the public api and config packages plus the standard library.
//
// This is the external tier of the two-tier plugin model:
// external plugins implement pdk.Plugin and receive pdk.Deps (capabilities as
// public interfaces), never raw repositories or internal service types. It is
// the surface we promise to keep stable. In-tree plugins use internal/plugin
// instead, with full access, and are rebuilt with the repo.
package pdk

import (
	"context"
	"net/http"
)

// Plugin is the contract an external extension implements. Every method
// signature uses only public types, so a Plugin can live in a separate module
// without importing platform-api's internal/ packages.
type Plugin interface {
	// Name returns a short identifier for the plugin (e.g. "api-cloud").
	Name() string

	// Init receives the platform capabilities (pdk.Deps). Called once at startup
	// before routes are registered; return an error to abort startup.
	Init(deps *Deps) error

	// RegisterRoutes mounts the plugin's HTTP routes on the shared mux. Only
	// called after Init has succeeded.
	RegisterRoutes(mux *http.ServeMux)

	// OpenAPISpec returns the plugin's OpenAPI 3.x YAML bytes, merged into the
	// platform scope registry to enforce per-route scopes. Return nil when the
	// plugin needs no scope enforcement.
	OpenAPISpec() []byte

	// Shutdown is called during graceful server shutdown.
	Shutdown(ctx context.Context) error
}

// AuthSkipPathProvider is an optional interface a Plugin may also implement to
// declare public (unauthenticated) path prefixes. The server appends them to the
// auth skip-path list before the auth middleware is built. Keep prefixes narrow
// and specific — this is an auth-bypass surface (GO-AUTH-004).
type AuthSkipPathProvider interface {
	AuthSkipPaths() []string
}
