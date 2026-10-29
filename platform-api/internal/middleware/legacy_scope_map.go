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

// legacy_scope_map.go — pre-permission-tree scope names and their translation.
//
// The OpenAPI spec declares only permission-tree scopes (ap:api:rest:manage).
// Tokens, IDP roles and role files may still carry the names that preceded the
// tree (ap:rest_api:manage). This file loads resources/legacy-scope-mapping.yaml,
// validates it against the scope registry at startup, and expands a request's
// held scopes through it so ScopeEnforcer and HasEffectiveScope see the tree
// permission each old name grants. Nothing here changes which scopes an
// operation accepts; that list is generated from resources/permission-tree.yaml.

package middleware

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// LegacyScopeMap maps each pre-permission-tree scope name (ap:rest_api:manage,
// ap:llm_proxy:deployment:read, ...) to the permission-tree scopes it grants
// (ap:api:rest:manage, ap:api:llm:deployment:read, ...). The OpenAPI spec
// declares only the tree names; this map is what keeps a token, an IDP role or
// a role-to-scope mapping that still carries the old names working:
// ScopeEnforcer expands a request's held scopes through it before matching
// (see Expand), and ValidateRoleScopeMap accepts a legacy name wherever it
// accepts a declared scope.
//
// It is loaded from resources/legacy-scope-mapping.yaml, a frozen list — no
// new legacy name is ever added — and a nil map disables the translation.
type LegacyScopeMap map[string][]string

// legacyScopeConfig is the top-level structure of legacy-scope-mapping.yaml.
type legacyScopeConfig struct {
	Scopes map[string][]string `yaml:"scopes"`
}

// LoadLegacyScopeMap reads a legacy-scope-mapping.yaml file.
func LoadLegacyScopeMap(path string) (LegacyScopeMap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("legacy-scope-mapping.yaml: read %q: %w", path, err)
	}
	var cfg legacyScopeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("legacy-scope-mapping.yaml: parse %q: %w", path, err)
	}
	if len(cfg.Scopes) == 0 {
		return nil, fmt.Errorf("legacy-scope-mapping.yaml: %q declares no scopes — remove legacy_scope_mapping_path from the configuration to disable legacy scope names instead", path)
	}
	return LegacyScopeMap(cfg.Scopes), nil
}

// ValidateLegacyScopeMap checks the map against the OpenAPI scope registry,
// failing fast at startup rather than at request time. Every legacy name must
// be a well-formed scope in this server's namespace that no operation declares
// any more (a name still in the spec needs no translation, and translating it
// would silently widen it), and every scope it maps to must be declared by an
// operation — a mapping onto an undeclared scope grants nothing and would
// surface as a 403 for every holder of the legacy name.
//
// Run it after plugin specs are merged into the registry: a legacy name of a
// plugin resource maps to a permission only that plugin declares.
func ValidateLegacyScopeMap(m LegacyScopeMap, registry *ScopeRegistry) error {
	known := registry.AllScopes()
	for _, legacy := range slices.Sorted(maps.Keys(m)) {
		if !wellFormedScope.MatchString(legacy) || !strings.HasPrefix(legacy, PlatformScopePrefix) {
			return fmt.Errorf("legacy-scope-mapping.yaml: malformed legacy scope %q — expected \"%s<name>\"", legacy, PlatformScopePrefix)
		}
		if _, declared := known[legacy]; declared {
			return fmt.Errorf("legacy-scope-mapping.yaml: %q is still declared by an operation in the OpenAPI spec — a declared scope needs no translation; remove it from the mapping", legacy)
		}
		targets := m[legacy]
		if len(targets) == 0 {
			return fmt.Errorf("legacy-scope-mapping.yaml: %q maps to no scope", legacy)
		}
		for _, target := range targets {
			if !wellFormedScope.MatchString(target) || !strings.HasPrefix(target, PlatformScopePrefix) {
				return fmt.Errorf("legacy-scope-mapping.yaml: %q maps to malformed scope %q", legacy, target)
			}
			if _, ok := known[target]; !ok {
				return fmt.Errorf("legacy-scope-mapping.yaml: %q maps to %q, which no operation in the OpenAPI spec declares", legacy, target)
			}
		}
	}
	return nil
}

// Expand returns scopes followed by the permission-tree scopes each legacy
// scope among them grants, without duplicates. A scope the map does not know is
// passed through unchanged, so a token holding tree permissions is unaffected,
// and the result is never narrower than the input. A nil map returns scopes
// as-is.
func (m LegacyScopeMap) Expand(scopes []string) []string {
	if len(m) == 0 || len(scopes) == 0 {
		return scopes
	}
	seen := make(map[string]struct{}, len(scopes))
	out := make([]string, 0, len(scopes))
	add := func(s string) {
		if _, dup := seen[s]; dup {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	for _, s := range scopes {
		add(s)
	}
	for _, s := range scopes {
		for _, granted := range m[s] {
			add(granted)
		}
	}
	return out
}
