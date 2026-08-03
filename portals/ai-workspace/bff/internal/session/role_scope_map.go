/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package session

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Role-to-scope grant table (auth.authorization.role_to_scope_mapping).
//
// The BFF's counterpart of platform-api's internal/middleware/role_scope_map.go and
// api-portal's src/config/roleScopeMap.js — the same file, in the same shape, read by
// a third component.
//
// Why the BFF needs it: in role authorization mode a caller's effective scopes are not
// in their token. An external IDP emits roles — Microsoft Entra ID cannot mint the
// platform's ap:* scopes at all — and the Platform API derives the effective scopes by
// expanding those roles through this table on every request. The SPA gates every
// action on the scopes /api/session reports, so without the same expansion here it
// would see only whatever the IDP put in the scope claim and show every operation as
// blocked, even where the Platform API would have authorized the call.
//
// The Platform API remains the only enforcement point; this expansion is for display
// and UI gating. Both components must therefore read the SAME file, or the UI's view
// of what a role grants drifts from what is actually enforced.

// maxMappingBytes caps the grant table read. A few hundred lines of YAML is normal;
// the ceiling guards against pointing the setting at something enormous by mistake.
const maxMappingBytes = 1 << 20 // 1 MiB

// roleScopeEntry is a single entry: an IDP role name and the scopes it grants.
type roleScopeEntry struct {
	Name   string   `yaml:"name"`
	Scopes []string `yaml:"scopes"`
}

// roleScopeConfig is the top-level structure of role-to-scope-mapping.yaml.
type roleScopeConfig struct {
	Roles []roleScopeEntry `yaml:"roles"`
}

// LoadRoleScopeMap reads a role-to-scope-mapping.yaml file and returns a map from IDP
// role name to the scopes that role grants. A token may carry several roles; callers
// union the lists at read time (see ExpandRoles).
//
// The path is operator-supplied configuration, not request input, so it is not confined
// to the {{ file }} allowlist — an operator may mount the grant table wherever they
// like. Traversal sequences are still rejected on the raw input, before normalization,
// since filepath.Clean would collapse them into a path that passes a later check.
func LoadRoleScopeMap(path string) (map[string][]string, error) {
	if path == "" || strings.ContainsRune(path, '\x00') {
		return nil, fmt.Errorf("role_to_scope_mapping is not a usable file path")
	}
	segments := strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' })
	if slices.Contains(segments, "..") {
		return nil, fmt.Errorf("role_to_scope_mapping %q must not contain traversal sequences", path)
	}
	cleaned := filepath.Clean(path)

	// Opened once and validated through the descriptor: a separate os.Stat followed by a
	// path-based read checks one file and reads another if the path is replaced in
	// between. IsRegular also rejects FIFOs and devices, where the reported size is 0 and
	// the ceiling below would otherwise never trigger — the LimitReader is what bounds
	// the read, the size check only fails fast.
	f, err := os.Open(cleaned)
	if err != nil {
		return nil, fmt.Errorf("role_to_scope_mapping file %q could not be read: %w", path, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("role_to_scope_mapping file %q could not be read: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("role_to_scope_mapping %q is not a regular file", path)
	}
	if info.Size() > maxMappingBytes {
		return nil, fmt.Errorf("role_to_scope_mapping file %q exceeds the maximum allowed size", path)
	}

	data, err := io.ReadAll(io.LimitReader(f, maxMappingBytes+1))
	if err != nil {
		return nil, fmt.Errorf("role_to_scope_mapping file %q could not be read: %w", path, err)
	}
	if int64(len(data)) > maxMappingBytes {
		return nil, fmt.Errorf("role_to_scope_mapping file %q exceeds the maximum allowed size", path)
	}

	var cfg roleScopeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("role_to_scope_mapping file %q is not valid YAML: %w", path, err)
	}
	if len(cfg.Roles) == 0 {
		return nil, fmt.Errorf("role_to_scope_mapping file %q must contain a non-empty top-level \"roles\" list", path)
	}

	m := make(map[string][]string, len(cfg.Roles))
	for i, entry := range cfg.Roles {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			return nil, fmt.Errorf("role_to_scope_mapping %q: entry %d has no \"name\"", path, i)
		}
		// Rejected rather than last-wins: with two entries for one role, one is
		// silently inert and which one depends on file order.
		if _, dup := m[name]; dup {
			return nil, fmt.Errorf("role_to_scope_mapping %q: role %q is declared more than once", path, name)
		}
		scopes := make([]string, 0, len(entry.Scopes))
		seen := make(map[string]struct{}, len(entry.Scopes))
		for _, scope := range entry.Scopes {
			trimmed := strings.TrimSpace(scope)
			if trimmed == "" {
				continue
			}
			if _, dup := seen[trimmed]; dup {
				continue
			}
			seen[trimmed] = struct{}{}
			scopes = append(scopes, trimmed)
		}
		m[name] = scopes
	}
	return m, nil
}

// ExpandRoles unions the scope lists granted by each of roles, preserving first-seen
// order and dropping duplicates. An unknown role contributes nothing, so a token
// carrying only roles the operator never mapped yields no scopes — the same
// deny-by-default outcome the Platform API reaches for the same token.
func ExpandRoles(roles []string, roleScopeMap map[string][]string) []string {
	if len(roles) == 0 || roleScopeMap == nil {
		return []string{}
	}
	out := make([]string, 0, len(roles)*8)
	seen := make(map[string]struct{})
	for _, role := range roles {
		for _, scope := range roleScopeMap[role] {
			if _, dup := seen[scope]; dup {
				continue
			}
			seen[scope] = struct{}{}
			out = append(out, scope)
		}
	}
	return out
}
