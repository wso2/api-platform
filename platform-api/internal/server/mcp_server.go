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
	"maps"
	"net/http"
	"regexp"
	"slices"

	"github.com/wso2/api-platform/platform-api/internal/mcpserver"
)

// startMCPServer registers /api/v0.9/mcp on mux. It returns the paths the
// Platform API's sign-in must skip and the middleware that goes in the chain
// after CORS. It runs after the plugins start and the core routes are
// installed, so the scope table and the route overrides are complete.
func startMCPServer(slogger *slog.Logger, mux *http.ServeMux, overrides map[string]routeOverride,
	deps mcpserver.Deps) ([]string, func(http.Handler) http.Handler, error) {

	srv, err := mcpserver.New(deps)
	if err != nil {
		return nil, nil, err
	}
	if err := checkOverrides(srv.ToolRoutes(), overrides); err != nil {
		return nil, nil, err
	}
	slogger.Info("MCP server enabled", "path", mcpserver.Path)
	return srv.Register(mux), srv.Wrap, nil
}

// checkOverrides refuses to start when a plugin decorates a route a tool uses:
// the tool calls the services, so the decorator would not run. Plugin
// middleware is not checked; for /mcp it sees POST /api/v0.9/mcp.
func checkOverrides(toolRoutes map[string][]string, overrides map[string]routeOverride) error {
	pluginFor := make(map[string]string, len(overrides))
	for pattern, ov := range overrides {
		pluginFor[withoutWildcardNames(pattern)] = ov.plugin
	}
	for _, tool := range slices.Sorted(maps.Keys(toolRoutes)) {
		for _, route := range toolRoutes[tool] {
			if plugin, ok := pluginFor[withoutWildcardNames(route)]; ok {
				return fmt.Errorf("mcp server: tool %s uses %s, which plugin %q overrides; "+
					"the tool would skip that plugin's code. Replace the tool in the plugin, or turn MCP off",
					tool, route, plugin)
			}
		}
	}
	return nil
}

var wildcardName = regexp.MustCompile(`\{[^}]*\}`)

// withoutWildcardNames blanks wildcard names: a plugin's {projectId} and a
// tool's {id} are the same route.
func withoutWildcardNames(route string) string {
	return wildcardName.ReplaceAllString(route, "{}")
}
