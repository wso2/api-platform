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

package middleware

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// ValidateScopeRegistryRoutes checks that every operation the scope registry
// declares, when probed against the router, resolves to a route whose pattern
// the registry can find again.
//
// ScopeEnforcer is deny-by-default: a matched route with no registry entry is
// rejected. So a declared operation whose probe lands on a *structurally
// different* route — one registered under a different path shape or base path —
// means the real route has no entry of its own and would be denied at runtime.
// Checking the mapping at startup turns that into a loud refusal to start
// (GO-AUTH-011) rather than a 403 discovered on a live endpoint.
//
// A declared operation the router matches nothing for is not an error: it is a
// dead registry entry protecting a route that does not exist, and requests to
// that path already get the router's 404. The reverse direction — a route
// registered with no declared scope — needs no check here either: ScopeEnforcer
// already fails those closed at request time.
func ValidateScopeRegistryRoutes(routes RouteMatcher, registry *ScopeRegistry) error {
	if routes == nil || registry == nil {
		return nil
	}

	var problems []string
	for _, op := range registry.Operations() {
		// Path parameters are substituted with a literal segment so the router
		// has something concrete to match; wildcard patterns are skipped since
		// no single probe path represents them.
		if strings.Contains(op.Path, "...") {
			continue
		}
		probePath := strings.ReplaceAll(op.Path, "{}", "scope-route-probe")

		req, err := http.NewRequest(op.Method, probePath, nil)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s %s: not a probeable route: %v", op.Method, op.Path, err))
			continue
		}

		_, pattern := routes.Handler(req)
		if pattern == "" {
			// Nothing registered for this operation — a dead entry, not a route
			// that would be wrongly denied.
			continue
		}

		matchedMethod, matchedPath, _ := strings.Cut(pattern, " ")
		if matchedMethod != op.Method || normalizePathParams(matchedPath) != op.Path {
			problems = append(problems, fmt.Sprintf(
				"%s %s: declared in the OpenAPI spec, but the router matched %q — "+
					"the matched route carries no scope requirement and would be denied",
				op.Method, op.Path, pattern))
		}
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("scope registry does not match the registered routes:\n  %s",
			strings.Join(problems, "\n  "))
	}
	return nil
}
