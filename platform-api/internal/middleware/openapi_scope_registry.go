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
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ScopeRegistry maps (HTTP method, net/http path pattern) to the acceptable scopes for
// that operation. Scopes are OR-evaluated: the caller needs at least one.
type ScopeRegistry struct {
	// scopes is keyed by "METHOD:/api/v1/path/{}", with path parameter names
	// normalized away by normalizePathParams.
	scopes map[string][]string
}

// pathParamName matches a "{name}" path-parameter placeholder, excluding the
// "{name...}" wildcard form, which matches a different set of paths and so must
// stay distinct.
var pathParamName = regexp.MustCompile(`\{[^/{}.]+\}`)

// normalizePathParams strips path parameter names from a route pattern, so
// "/gateways/{gatewayId}" and "/gateways/{apiId}" produce the same key.
//
// A parameter's name is not part of a route's identity: net/http matches on the
// pattern's structure, and the name only determines what r.PathValue reads it
// back as. Keying the registry on the name means an OpenAPI spec that documents
// "{apiId}" while the route registers "{webSubApiId}" silently fails every
// lookup — the same shape of failure as looking up an unmatched route, and just
// as invisible.
func normalizePathParams(path string) string {
	return pathParamName.ReplaceAllString(path, "{}")
}

// Lookup returns the required scopes for the given HTTP method and path pattern
// (e.g. r.Method and the path portion of r.Pattern). found is false when the route
// is not in the OpenAPI spec, meaning no scope requirement was declared.
func (r *ScopeRegistry) Lookup(method, path string) ([]string, bool) {
	key := strings.ToUpper(method) + ":" + normalizePathParams(path)
	scopes, ok := r.scopes[key]
	return scopes, ok
}

// Len returns the number of (method, path) operations that carry a scope
// requirement. Operations with no security block contribute no entry.
func (r *ScopeRegistry) Len() int {
	return len(r.scopes)
}

// Operation is a single (method, path pattern) entry in the registry.
type Operation struct {
	Method string
	Path   string
}

// Operations returns every (method, path) the registry declares a scope
// requirement for. Used at startup to verify each declared operation actually
// resolves to a registered route.
func (r *ScopeRegistry) Operations() []Operation {
	ops := make([]Operation, 0, len(r.scopes))
	for key := range r.scopes {
		method, path, found := strings.Cut(key, ":")
		if !found {
			continue
		}
		ops = append(ops, Operation{Method: method, Path: path})
	}
	return ops
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
//
// A path item's fields are held as raw nodes rather than decoded straight into
// openAPIOperation: alongside its operations, a path item may carry "parameters"
// (a sequence), "summary", "$ref" and others. Decoding those into an operation
// struct fails, and because that failure is per-document, a single path-level
// "parameters" block makes the entire spec unloadable — which, for a spec whose
// only purpose here is to declare required scopes, means no scope in it is ever
// enforced.
type openAPIDoc struct {
	Servers []struct {
		URL string `yaml:"url"`
	} `yaml:"servers"`
	Paths map[string]map[string]yaml.Node `yaml:"paths"`
}

// httpMethods is the set of path-item keys that denote an operation. Anything
// else in a path item is metadata and carries no security requirement.
var httpMethods = map[string]struct{}{
	"get": {}, "put": {}, "post": {}, "delete": {},
	"options": {}, "head": {}, "patch": {}, "trace": {},
}

// operationsIn decodes the operation entries of a path item, skipping the
// path-item metadata keys that are not operations.
func operationsIn(pathItem map[string]yaml.Node, oaPath string) (map[string]openAPIOperation, error) {
	ops := make(map[string]openAPIOperation, len(pathItem))
	for field, node := range pathItem {
		if _, isMethod := httpMethods[strings.ToLower(field)]; !isMethod {
			continue
		}
		var op openAPIOperation
		if err := node.Decode(&op); err != nil {
			return nil, fmt.Errorf("path %q, operation %q: %w", oaPath, field, err)
		}
		ops[field] = op
	}
	return ops, nil
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
	for oaPath, pathItem := range doc.Paths {
		httpPath := basePath + oaPath
		methods, opErr := operationsIn(pathItem, oaPath)
		if opErr != nil {
			return nil, fmt.Errorf("openapi scope registry: parse embedded spec: %w", opErr)
		}
		for method, op := range methods {
			scopes := collectScopes(op.Security)
			if len(scopes) == 0 {
				continue
			}
			key := strings.ToUpper(method) + ":" + normalizePathParams(httpPath)
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

	for oaPath, pathItem := range doc.Paths {
		// Keep OpenAPI {param} syntax — it matches net/http ServeMux path values directly.
		httpPath := basePath + oaPath
		methods, opErr := operationsIn(pathItem, oaPath)
		if opErr != nil {
			return nil, fmt.Errorf("openapi scope registry: parse %q: %w", specPath, opErr)
		}
		for method, op := range methods {
			scopes := collectScopes(op.Security)
			if len(scopes) == 0 {
				continue
			}
			key := strings.ToUpper(method) + ":" + normalizePathParams(httpPath)
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
