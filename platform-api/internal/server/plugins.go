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

package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/plugin"
	"github.com/wso2/api-platform/platform-api/pdk"
)

// pluginShutdownTimeout bounds the graceful shutdown of plugins when startup
// aborts, so one misbehaving plugin cannot hang the failure path.
const pluginShutdownTimeout = 10 * time.Second

// pluginWiring is everything the server needs from the plugin tiers once they are
// initialized: the combined plugin list (for shutdown), the middleware to splice
// into the request chain at each allowed position, and the public path prefixes to
// add to the auth skip-path list.
type pluginWiring struct {
	// plugins holds internal plugins as-is and external plugins wrapped in
	// externalPlugin, in initialization order.
	plugins []plugin.Plugin
	// preChain runs outermost, before CORS/auth; postChain innermost, after scope
	// enforcement and before the mux.
	preChain, postChain []pdk.Middleware
	// authSkipPaths are validated public path prefixes contributed by plugins.
	authSkipPaths []string
}

// initPlugins initializes both plugin tiers against the shared mux and scope
// registry and returns the resulting wiring. It is tier-agnostic: internal plugins
// receive internalDeps, external plugins receive pdkDeps (capabilities only), and
// everything else — spec validation, route registration, skip paths, middleware —
// is identical for both.
//
// On any failure it shuts down the plugins it already initialized, in reverse
// initialization order, before returning the error; the caller is responsible for
// shutting them down if a *later* startup step fails.
func initPlugins(
	slogger *slog.Logger,
	mux *http.ServeMux,
	scopeRegistry *middleware.ScopeRegistry,
	internalDeps *plugin.Deps,
	pdkDeps *pdk.Deps,
	internalPlugins []plugin.Plugin,
	externalPlugins []pdk.Plugin,
) (*pluginWiring, error) {
	// Combine both tiers into one list: internal plugins as-is, external plugins
	// wrapped so they present the internal plugin.Plugin shape. The loop below is
	// tier-agnostic; only the Deps each receives differs (see externalPlugin).
	all := make([]plugin.Plugin, 0, len(internalPlugins)+len(externalPlugins))
	all = append(all, internalPlugins...)
	for _, ep := range externalPlugins {
		if ep == nil {
			continue
		}
		all = append(all, &externalPlugin{p: ep, pdkDeps: pdkDeps})
	}

	w := &pluginWiring{}
	// Plugins whose Init has already succeeded. If a later plugin aborts startup,
	// these are shut down in reverse initialization order before returning, so a
	// failed startup does not leave plugin-owned goroutines or connections behind.
	var initialized []plugin.Plugin
	fail := func(format string, args ...any) (*pluginWiring, error) {
		shutdownPlugins(slogger, initialized)
		return nil, fmt.Errorf(format, args...)
	}

	for _, p := range all {
		if err := p.Init(internalDeps); err != nil {
			return fail("plugin %q failed to initialize: %w", p.Name(), err)
		}
		initialized = append(initialized, p)

		// Merge plugin-contributed scopes into the main registry, which is what
		// ScopeEnforcer consults on each request. The spec must be present and
		// must load (GO-AUTH-007).
		spec := p.OpenAPISpec()
		if len(spec) == 0 {
			return fail("plugin %q returned an empty OpenAPI spec", p.Name())
		}
		pluginRegistry, regErr := middleware.LoadScopeRegistryFromBytes(spec)
		if regErr != nil {
			return fail("plugin %q OpenAPI spec failed to load into the scope registry: %w", p.Name(), regErr)
		}
		scopeRegistry.Merge(pluginRegistry)

		p.RegisterRoutes(mux)
		slogger.Info("Plugin initialized", "name", p.Name())

		// Declared public paths are collected here and appended to the config's
		// skip-path list by the caller, before the auth middleware is built, so the
		// list is complete when the chain is assembled.
		if sp, ok := p.(plugin.AuthSkipPathProvider); ok {
			for _, path := range sp.AuthSkipPaths() {
				if err := validateAuthSkipPath(path); err != nil {
					return fail("plugin %q declared an invalid auth skip path %q: %w", p.Name(), path, err)
				}
				w.authSkipPaths = append(w.authSkipPaths, path)
			}
		}

		// Collect plugin middleware into the two allowed positions. pdk mirrors
		// this interface and externalPlugin forwards it, so both tiers are handled
		// by this one assertion.
		if mp, ok := p.(plugin.MiddlewareProvider); ok {
			for _, m := range mp.Middleware() {
				// A nil Wrap is a malformed entry, not a way to opt out: skipping
				// it silently would drop middleware the author believes is wired
				// (a panic recovery, an IP allow-list) with no signal, and letting
				// it through panics when the chain is composed around the mux.
				// Abort for the same reason an unknown position does below.
				if m.Wrap == nil {
					return fail("plugin %q declared middleware at chain position %d with a nil Wrap; "+
						"return an empty slice to contribute no middleware", p.Name(), m.Position)
				}
				switch m.Position {
				case pdk.BeforePlatformChain:
					w.preChain = append(w.preChain, m.Wrap)
				case pdk.AfterPlatformChain:
					w.postChain = append(w.postChain, m.Wrap)
				default:
					return fail("plugin %q declared middleware at unknown chain position %d", p.Name(), m.Position)
				}
			}
		}
	}

	w.plugins = all
	return w, nil
}

// validateAuthSkipPath rejects path prefixes that would widen the auth bypass
// beyond the narrow, specific prefix a plugin is allowed to declare. Skip-path
// matching is a prefix match (see middleware.LocalJWTAuthMiddleware), so "" or "/"
// would disable authentication for every route on the server (GO-AUTH-004).
func validateAuthSkipPath(path string) error {
	switch {
	case path == "":
		return fmt.Errorf("path is empty, which matches every request")
	case path == "/":
		return fmt.Errorf("path is the root prefix, which matches every request")
	case !strings.HasPrefix(path, "/"):
		return fmt.Errorf("path must start with %q", "/")
	case strings.Contains(path, ".."):
		return fmt.Errorf("path must not contain %q", "..")
	}
	return nil
}

// shutdownPlugins shuts plugins down in reverse initialization order, logging but
// not propagating individual errors — it runs on failure paths where the original
// error is the one that matters. It is used both when plugin initialization itself
// aborts and when a later startup step fails after plugins are already running.
func shutdownPlugins(slogger *slog.Logger, plugins []plugin.Plugin) {
	ctx, cancel := context.WithTimeout(context.Background(), pluginShutdownTimeout)
	defer cancel()
	for i := len(plugins) - 1; i >= 0; i-- {
		if err := plugins[i].Shutdown(ctx); err != nil {
			slogger.Error("Plugin shutdown error during failed startup",
				"plugin", plugins[i].Name(), "error", err)
		}
	}
}
