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
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/router"
)

// installCoreRoutes registers every recorded core route on the real mux, wrapped
// where a plugin has claimed the pattern with a route override.
//
// It runs after initPlugins, so plugins have already registered their own routes
// directly on the mux and declared which core patterns they decorate. Each
// pattern is registered exactly once — with the decorator around the original
// core handler, or with the core handler alone — so the mux resolves path
// wildcards before the decorator runs and r.PathValue is still correct inside
// the core handler.
//
// Every failure here is a startup error: an override naming a pattern core does
// not register is a wiring mistake that would otherwise do nothing at all, and a
// plugin route colliding with a core pattern would otherwise panic.
func installCoreRoutes(
	slogger *slog.Logger,
	mux *http.ServeMux,
	core *router.Recorder,
	overrides map[string]routeOverride,
) (err error) {
	// Reject unknown patterns before installing anything, so a typo fails
	// startup with a clear message rather than silently decorating nothing.
	// Sorted for a deterministic error across runs.
	unknown := make([]string, 0, len(overrides))
	for pattern := range overrides {
		if !core.Has(pattern) {
			unknown = append(unknown, pattern)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		claims := make([]string, 0, len(unknown))
		for _, pattern := range unknown {
			claims = append(claims, fmt.Sprintf("plugin %q -> %q", overrides[pattern].plugin, pattern))
		}
		return fmt.Errorf("%d route override(s) name a pattern that is not a core route "+
			"(patterns are matched exactly, including method and path): %s",
			len(unknown), strings.Join(claims, "; "))
	}

	// A duplicate pattern can now only come from a plugin route registered on
	// the mux during initPlugins that collides with a core one. ServeMux panics
	// on that; convert it into a startup error naming the pattern.
	var installing string
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("failed to register core route %q: it is already registered, "+
				"most likely by a plugin route claiming the same pattern (%v)", installing, r)
		}
	}()

	for _, rt := range core.Routes() {
		installing = rt.Pattern
		handler := rt.Handler
		if ov, claimed := overrides[rt.Pattern]; claimed {
			handler = ov.wrap(rt.Handler)
			if handler == nil {
				return fmt.Errorf("plugin %q returned a nil handler when overriding route %q",
					ov.plugin, rt.Pattern)
			}
			slogger.Info("Plugin overrides route", "plugin", ov.plugin, "pattern", rt.Pattern)
		}
		mux.Handle(rt.Pattern, handler)
	}
	return nil
}
