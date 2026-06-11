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

	"gopkg.in/yaml.v3"
)

// roleScopeEntry is a single entry in roles.yaml: an IDP role name and the
// platform scopes it grants.
type roleScopeEntry struct {
	Name   string   `yaml:"name"`
	Scopes []string `yaml:"scopes"`
}

// roleScopeConfig is the top-level structure of roles.yaml.
type roleScopeConfig struct {
	Roles []roleScopeEntry `yaml:"roles"`
}

// LoadRoleScopeMap reads a roles.yaml file and returns a map from IDP role name
// to the list of platform scopes that role grants. Each user token may carry
// multiple roles; the caller is expected to union the scope lists at request time.
func LoadRoleScopeMap(path string) (map[string][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("roles.yaml: read %q: %w", path, err)
	}
	var cfg roleScopeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("roles.yaml: parse %q: %w", path, err)
	}
	m := make(map[string][]string, len(cfg.Roles))
	for _, entry := range cfg.Roles {
		if entry.Name == "" {
			return nil, fmt.Errorf("roles.yaml: entry missing required 'name' field in %q", path)
		}
		m[entry.Name] = entry.Scopes
	}
	return m, nil
}

// ValidateRoleScopeMap checks that every scope referenced in the map is declared
// in the OpenAPI spec. An unrecognized scope name is almost certainly a typo that
// would silently deny access, so we fail fast at startup rather than at request time.
func ValidateRoleScopeMap(m map[string][]string, registry *ScopeRegistry) error {
	known := registry.AllScopes()
	for role, scopes := range m {
		for _, s := range scopes {
			if _, ok := known[s]; !ok {
				return fmt.Errorf("roles.yaml: role %q references unknown scope %q — check the OpenAPI spec for valid scope names", role, s)
			}
		}
	}
	return nil
}
