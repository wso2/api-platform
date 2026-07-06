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
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ScopeRegistry maps (HTTP method, net/http path pattern) to the acceptable scopes for
// that operation. Scopes are OR-evaluated: the caller needs at least one.
type ScopeRegistry struct {
	// scopes is keyed by "METHOD:/api/v1/path/{param}".
	scopes map[string][]string
}

// Lookup returns the required scopes for the given HTTP method and path pattern
// (e.g. r.Method and the path portion of r.Pattern). found is false when the route
// is not in the OpenAPI spec, meaning no scope requirement was declared.
func (r *ScopeRegistry) Lookup(method, path string) ([]string, bool) {
	key := strings.ToUpper(method) + ":" + path
	scopes, ok := r.scopes[key]
	return scopes, ok
}

// AllScopes returns the set of every scope name declared across all operations.
// Used at startup to validate that roles.yaml only references known scopes.
func (r *ScopeRegistry) AllScopes() map[string]struct{} {
	known := make(map[string]struct{})
	for _, scopes := range r.scopes {
		for _, s := range scopes {
			known[s] = struct{}{}
		}
	}
	return known
}

// openAPIDoc is the minimal subset of an OpenAPI 3.x document we need to parse.
type openAPIDoc struct {
	Servers []struct {
		URL string `yaml:"url"`
	} `yaml:"servers"`
	Paths map[string]map[string]openAPIOperation `yaml:"paths"`
}

// openAPIOperation captures the per-operation security requirements. Each entry in
// Security is a map from scheme name to scope list; multiple scopes under a single
// scheme are OR-evaluated (any one scope is sufficient), following the WSO2 convention.
type openAPIOperation struct {
	Security []map[string][]string `yaml:"security"`
}

// Merge copies all scope entries from other into r, overwriting on key conflicts.
// Used to merge plugin-contributed OpenAPI specs into the main registry.
func (r *ScopeRegistry) Merge(other *ScopeRegistry) {
	if other == nil {
		return
	}
	for k, v := range other.scopes {
		r.scopes[k] = v
	}
}

// LoadScopeRegistryFromBytes parses an OpenAPI 3.x YAML document from in-memory
// bytes and returns a populated ScopeRegistry. Intended for plugins that embed
// their own OpenAPI spec via go:embed.
func LoadScopeRegistryFromBytes(data []byte) (*ScopeRegistry, error) {
	var doc openAPIDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("openapi scope registry: parse embedded spec: %w", err)
	}

	basePath := ""
	if len(doc.Servers) > 0 {
		basePath = extractBasePath(doc.Servers[0].URL)
	}

	registry := &ScopeRegistry{scopes: make(map[string][]string)}
	for oaPath, methods := range doc.Paths {
		httpPath := basePath + oaPath
		for method, op := range methods {
			scopes := collectScopes(op.Security)
			if len(scopes) == 0 {
				continue
			}
			key := strings.ToUpper(method) + ":" + httpPath
			registry.scopes[key] = scopes
		}
	}
	return registry, nil
}

// LoadScopeRegistry parses the OpenAPI spec at specPath and returns a ScopeRegistry
// populated from the standard security field on each operation. The first servers[].url
// is used to derive the base path prefix that maps spec paths to actual net/http route
// patterns (e.g. /api/v1 + /projects → /api/v1/projects).
func LoadScopeRegistry(specPath string) (*ScopeRegistry, error) {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("openapi scope registry: read %q: %w", specPath, err)
	}

	var doc openAPIDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("openapi scope registry: parse %q: %w", specPath, err)
	}

	basePath := ""
	if len(doc.Servers) > 0 {
		basePath = extractBasePath(doc.Servers[0].URL)
	}

	registry := &ScopeRegistry{scopes: make(map[string][]string)}

	for oaPath, methods := range doc.Paths {
		// Keep OpenAPI {param} syntax — it matches net/http ServeMux path values directly.
		httpPath := basePath + oaPath
		for method, op := range methods {
			scopes := collectScopes(op.Security)
			if len(scopes) == 0 {
				continue
			}
			key := strings.ToUpper(method) + ":" + httpPath
			registry.scopes[key] = scopes
		}
	}

	return registry, nil
}

// collectScopes flattens all scopes from the security requirement objects into a
// single de-duplicated list. Multiple scopes within one requirement object are
// treated as OR (WSO2 convention), so we collect them all into one list for the
// existing OR-check middleware to evaluate.
func collectScopes(security []map[string][]string) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, requirement := range security {
		for _, scopes := range requirement {
			for _, s := range scopes {
				if _, exists := seen[s]; !exists {
					seen[s] = struct{}{}
					result = append(result, s)
				}
			}
		}
	}
	return result
}

// extractBasePath returns the path component of a URL string (e.g. "/api/v1"),
// stripping the scheme and host.
func extractBasePath(rawURL string) string {
	s := rawURL
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.Index(s, "/"); i >= 0 {
		path := s[i:]
		return strings.TrimRight(path, "/")
	}
	return ""
}
