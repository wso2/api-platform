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

// TEMP-READ-ONLY-MODE: this whole file is part of the temporary organization-scoped
// read-only mode used while Bijira migrates from Platform API v1 to v2. Delete it
// when the mode is removed — see README.md "Read-only (maintenance) mode — temporary"
// for the removal checklist.

package config

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ReadOnly holds the organization-scoped read-only (maintenance) mode
// configuration. While Enabled is true every organization is read-only — its
// write requests are rejected with HTTP 503 while reads keep working — except
// the organizations listed in WritableOrganizations. While Enabled is false the
// feature is fully off and the list has no effect.
//
// Organizations are identified by their platform UUID (the organization_uuid
// column every tenant-scoped table carries, and the value the auth chain
// resolves the token's organization claim into), never by handle. Entries are
// canonicalised to lowercase UUID form at load time (see validateReadOnlyConfig),
// which also refuses to start on an entry that is not a UUID: a handle passed by
// mistake would otherwise silently leave that organization read-only.
//
// It is a restart-time setting: the config is loaded once, so toggling it means
// restarting the process — and every replica must be given identical values, or
// one replica keeps accepting writes. The startup log announces the mode and the
// writable list so per-replica drift is visible.
type ReadOnly struct {
	Enabled               bool     `koanf:"enabled"`
	WritableOrganizations []string `koanf:"writable_organizations"`
}

// IsReadOnlyOrg reports whether write operations for the given organization
// UUID must be rejected: read-only mode is enabled and the organization is not
// in the writable list. Nil-safe, so a nil receiver (tests, components that were
// never wired) behaves as "disabled". Matching is case-insensitive on the
// trimmed value so it stays correct even for a value that did not pass through
// validateReadOnlyConfig. An empty orgUUID is read-only whenever the mode is
// enabled — an organization that cannot be identified is never one of the
// listed writable ones (fail closed); callers that legitimately have no
// organization yet must decide before asking.
func (r *ReadOnly) IsReadOnlyOrg(orgUUID string) bool {
	if r == nil || !r.Enabled {
		return false
	}
	id := strings.TrimSpace(orgUUID)
	if id == "" {
		return true
	}
	for _, o := range r.WritableOrganizations {
		if strings.EqualFold(strings.TrimSpace(o), id) {
			return false
		}
	}
	return true
}

// validateReadOnlyConfig trims, canonicalises and de-duplicates
// read_only.writable_organizations. It runs whether or not the mode is enabled,
// so a bad entry is caught before the flag is ever flipped on, and it fails
// startup on any entry that is not a UUID: a handle passed by mistake must not
// silently leave that organization read-only once the mode is enabled. A value
// arriving through an {{ env }} token is split on commas by the decode hook and
// reaches here untrimmed, so whitespace around entries is tolerated.
func validateReadOnlyConfig(cfg *ReadOnly) error {
	seen := make(map[string]struct{}, len(cfg.WritableOrganizations))
	normalized := make([]string, 0, len(cfg.WritableOrganizations))
	for _, raw := range cfg.WritableOrganizations {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		parsed, err := uuid.Parse(entry)
		if err != nil {
			return fmt.Errorf("read_only.writable_organizations entry %q is not a valid UUID "+
				"(organizations must be identified by their platform UUID, not their handle): %w", entry, err)
		}
		id := parsed.String()
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	if len(normalized) == 0 {
		cfg.WritableOrganizations = nil
		return nil
	}
	cfg.WritableOrganizations = normalized
	return nil
}
