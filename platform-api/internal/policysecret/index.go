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

package policysecret

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// policyKey identifies a policy as an attachment names it.
type policyKey struct {
	name    string
	version string
}

// Index resolves a policy name and version to its write-only parameter paths.
//
// It is built from the gateway manifests an organization's gateways have
// reported, which is where a policy definition's declarations reach the control
// plane. It is immutable once built: a manifest that arrives later replaces the
// organization's index rather than mutating it, so a resolver handed out
// mid-request keeps answering consistently for the whole of that request.
type Index struct {
	byKey map[policyKey][]Path
	// byName is the union of every version's paths, used when an attachment
	// names no version or names one no gateway has reported. A parameter that
	// is write-only in any version of a policy is treated as write-only for all
	// of them: over-withholding a parameter is a display defect, under-
	// withholding one hands back a credential.
	byName map[string][]Path
}

// manifestEntry is the part of a stored manifest entry this package reads.
// The manifest carries more; nothing else here is relevant to a parameter's
// write-only declaration.
type manifestEntry struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	WriteOnlyParams []Path `json:"writeOnlyParams,omitempty"`
}

// NewIndex builds an index from an organization's stored gateway manifests.
//
// A manifest that does not parse is an error rather than a skipped entry: a
// half-built index silently stops withholding whatever the unreadable manifest
// declared, and the caller is better placed to decide that a read should fail
// than this package is to decide it should leak.
func NewIndex(manifests [][]byte) (*Index, error) {
	index := &Index{
		byKey:  map[policyKey][]Path{},
		byName: map[string][]Path{},
	}
	for i, raw := range manifests {
		if len(raw) == 0 {
			continue
		}
		var entries []manifestEntry
		if err := json.Unmarshal(raw, &entries); err != nil {
			return nil, fmt.Errorf("failed to parse gateway manifest %d: %w", i, err)
		}
		for _, entry := range entries {
			if len(entry.WriteOnlyParams) == 0 {
				continue
			}
			name := normalizeName(entry.Name)
			key := policyKey{name: name, version: normalizeVersion(entry.Version)}
			index.byKey[key] = mergePaths(index.byKey[key], entry.WriteOnlyParams)
			index.byName[name] = mergePaths(index.byName[name], entry.WriteOnlyParams)
		}
	}
	return index, nil
}

// WriteOnlyPaths implements Resolver.
func (i *Index) WriteOnlyPaths(policyName, version string) []Path {
	if i == nil {
		return nil
	}
	name := normalizeName(policyName)
	if paths, ok := i.byKey[policyKey{name: name, version: normalizeVersion(version)}]; ok {
		return paths
	}
	return i.byName[name]
}

// normalizeName matches how a policy name is stored in the manifest, which is
// lower-cased on the way in. An attachment naming "AWS-Authentication" names
// the same policy as one naming "aws-authentication".
func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// normalizeVersion ignores a leading "v", which a definition carries ("v1.4.1")
// and an attachment often does not ("1.4.1").
func normalizeVersion(version string) string {
	return strings.TrimPrefix(strings.TrimSpace(version), "v")
}

// mergePaths appends the paths of into, skipping duplicates. Two gateways in
// one organization report the same catalog, so the same path arrives repeatedly.
func mergePaths(into []Path, from []Path) []Path {
	for _, candidate := range from {
		if len(candidate) == 0 {
			continue
		}
		duplicate := false
		for _, existing := range into {
			if pathsEqual(existing, candidate) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			into = append(into, candidate)
		}
	}
	return into
}

// pathsEqual reports whether two paths name the same parameter.
func pathsEqual(a, b Path) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Loader returns the stored gateway manifests of one organization.
//
// Returning them raw rather than decoded keeps this package independent of the
// repository layer and of the manifest's other fields.
type Loader func(orgID string) ([][]byte, error)

// Store is the per-organization cache of resolved indexes.
//
// A policy definition changes only when a gateway reports a new manifest, which
// is rare and observable, so the index is built on first use and held until
// that happens. Without the cache, every read of every API would re-read every
// gateway manifest in the organization to answer a question whose answer almost
// never changes.
//
// It is per replica and holds nothing a restart cannot rebuild.
type Store struct {
	mu     sync.RWMutex
	byOrg  map[string]*Index
	loader Loader
}

// NewStore returns a Store that builds its indexes with loader.
func NewStore(loader Loader) *Store {
	return &Store{byOrg: map[string]*Index{}, loader: loader}
}

// For returns the resolver for an organization, building its index if this
// replica has not already.
//
// An error here means the write-only declarations could not be read, and the
// caller must fail the read rather than continue: a resolver that answers "no
// parameter is write-only" because its manifests were unreadable is
// indistinguishable, in the response it produces, from one that answers
// correctly for a policy with no secrets.
func (s *Store) For(orgID string) (Resolver, error) {
	s.mu.RLock()
	cached, ok := s.byOrg[orgID]
	s.mu.RUnlock()
	if ok {
		return cached, nil
	}

	manifests, err := s.loader(orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to load gateway manifests for organization: %w", err)
	}
	index, err := NewIndex(manifests)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	// A concurrent build may have stored one first. Either is correct — both
	// read the same manifests — so the first one in wins and the other is
	// dropped, which keeps a resolver already handed out from being orphaned
	// by this one.
	if existing, ok := s.byOrg[orgID]; ok {
		index = existing
	} else {
		s.byOrg[orgID] = index
	}
	s.mu.Unlock()
	return index, nil
}

// Invalidate drops an organization's index, so the next read rebuilds it.
//
// Called when a gateway reports a manifest: that is the only thing that changes
// which parameters a policy declares write-only.
func (s *Store) Invalidate(orgID string) {
	s.mu.Lock()
	delete(s.byOrg, orgID)
	s.mu.Unlock()
}
