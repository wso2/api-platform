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

// roleScopeEntry is a single entry in roles_to_scope_mapping.yaml: an IDP role name and the
// platform scopes it grants.
type roleScopeEntry struct {
	Name   string   `yaml:"name"`
	Scopes []string `yaml:"scopes"`
}

// roleScopeConfig is the top-level structure of roles_to_scope_mapping.yaml.
type roleScopeConfig struct {
	Roles []roleScopeEntry `yaml:"roles"`
}

// LoadRoleScopeMap reads a roles_to_scope_mapping.yaml file and returns a map from IDP role name
// to the list of platform scopes that role grants. Each user token may carry
// multiple roles; the caller is expected to union the scope lists at request time.
func LoadRoleScopeMap(path string) (map[string][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("roles_to_scope_mapping.yaml: read %q: %w", path, err)
	}
	var cfg roleScopeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("roles_to_scope_mapping.yaml: parse %q: %w", path, err)
	}
	m := make(map[string][]string, len(cfg.Roles))
	for _, entry := range cfg.Roles {
		if entry.Name == "" {
			return nil, fmt.Errorf("roles_to_scope_mapping.yaml: entry missing required 'name' field in %q", path)
		}
		m[entry.Name] = entry.Scopes
	}
	return m, nil
}

// PlatformScopePrefix is the namespace of the scopes this server declares and
// enforces. Scopes in any other namespace belong to a sibling component that
// trusts the same token (the Developer Portal's "dp:" scopes, for example): this
// server only mints them, so it can neither confirm nor deny that they exist.
const PlatformScopePrefix = "ap:"

// wellFormedScope matches "<namespace>:<name>" — one or more colon-separated
// segments after the namespace, with "*" permitted only as the final segment.
// Segments allow hyphens as well as underscores, since a foreign namespace
// (the Developer Portal's "dp:", say) picks its own naming convention. This is
// enough to catch a missing or malformed namespace on a foreign scope, which is
// the only error class detectable without that component's spec.
var wellFormedScope = regexp.MustCompile(`^[a-z0-9_-]+(?::[a-z0-9_-]+)+(?::\*)?$`)

// ValidateRoleScopeMap checks the scopes referenced in the map, failing fast at
// startup rather than at request time — an unrecognized scope name is almost
// certainly a typo that would otherwise surface as a silent 403.
//
// Validation is namespace-scoped. A scope in this server's own namespace
// (PlatformScopePrefix) must be declared in the OpenAPI spec — that includes
// scopes contributed by compiled-in plugins, so this must run after the plugin
// specs are merged. A scope in another component's namespace is checked only for
// well-formedness: the roles file is where a role's grants across the whole
// platform are described, and it is the only place a file-mode user's grants can
// be expressed, so refusing scopes this server does not declare would make them
// ungrantable.
func ValidateRoleScopeMap(m map[string][]string, registry *ScopeRegistry) error {
	known := registry.AllScopes()
	for role, scopes := range m {
		for _, s := range scopes {
			if !wellFormedScope.MatchString(s) {
				return fmt.Errorf("roles_to_scope_mapping.yaml: role %q references malformed scope %q — expected \"<namespace>:<name>\", e.g. %sorganization:manage",
					role, s, PlatformScopePrefix)
			}
			if !strings.HasPrefix(s, PlatformScopePrefix) {
				continue // another component's namespace — not ours to validate
			}
			if _, ok := known[s]; !ok {
				return fmt.Errorf("roles_to_scope_mapping.yaml: role %q references unknown scope %q — check the OpenAPI spec for valid scope names", role, s)
			}
		}
	}
	return nil
}
