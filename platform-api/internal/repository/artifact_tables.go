/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied.  See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 *
 */

package repository

import (
	"fmt"
	"strings"
	"sync"
)

// ArtifactTableEntry describes a kind-specific child table that backs artifact rows.
// Core tables are pre-registered; plugins register their own entries during Init.
type ArtifactTableEntry struct {
	// Table is the SQL table name, e.g. "websub_apis".
	Table string
	// KindAlias is the value stored in artifacts.type, e.g. "WebSubApi".
	KindAlias string
	// KindKeys are the acceptable kind strings for Exists() lookup (typically the
	// HTTP handle form "websub-api" and the Go constant form "WebSubApi").
	KindKeys []string
}

// ArtifactTableRegistry maintains the set of kind-specific tables that back artifact rows.
// Core tables (rest_apis, llm_providers, llm_proxies, mcp_proxies) are pre-registered;
// plugins call Register during Init to contribute their own tables.
type ArtifactTableRegistry struct {
	mu      sync.RWMutex
	entries []ArtifactTableEntry
}

// NewArtifactTableRegistry returns a registry pre-seeded with the four core artifact tables.
func NewArtifactTableRegistry() *ArtifactTableRegistry {
	r := &ArtifactTableRegistry{}
	r.Register(ArtifactTableEntry{
		Table:     "rest_apis",
		KindAlias: "RestApi",
		KindKeys:  []string{"rest-api", "RestApi"},
	})
	r.Register(ArtifactTableEntry{
		Table:     "llm_providers",
		KindAlias: "LlmProvider",
		KindKeys:  []string{"llm-provider", "LlmProvider"},
	})
	r.Register(ArtifactTableEntry{
		Table:     "llm_proxies",
		KindAlias: "LlmProxy",
		KindKeys:  []string{"llm-proxy", "LlmProxy"},
	})
	r.Register(ArtifactTableEntry{
		Table:     "mcp_proxies",
		KindAlias: "Mcp",
		KindKeys:  []string{"mcp-proxy", "MCPProxy", "Mcp"},
	})
	return r
}

// Register appends an entry. Plugins call this during Init before the server
// starts accepting requests.
func (r *ArtifactTableRegistry) Register(e ArtifactTableEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, e)
}

// Entries returns a snapshot copy of all registered entries in registration order.
func (r *ArtifactTableRegistry) Entries() []ArtifactTableEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cp := make([]ArtifactTableEntry, len(r.entries))
	for i, e := range r.entries {
		keys := make([]string, len(e.KindKeys))
		copy(keys, e.KindKeys)
		cp[i] = e
		cp[i].KindKeys = keys
	}
	return cp
}

// TableByKindKey returns the entry whose KindKeys slice contains key, and true if found.
func (r *ArtifactTableRegistry) TableByKindKey(key string) (ArtifactTableEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, e := range r.entries {
		for _, k := range e.KindKeys {
			if k == key {
				return e, true
			}
		}
	}
	return ArtifactTableEntry{}, false
}

// IsValidKindAlias reports whether alias matches any registered entry's KindAlias.
func (r *ArtifactTableRegistry) IsValidKindAlias(alias string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, e := range r.entries {
		if e.KindAlias == alias {
			return true
		}
	}
	return false
}

// UnionAllSelect builds a "SELECT cols FROM t1 UNION ALL SELECT cols FROM t2 …" fragment
// for all registered tables. cols must not include a trailing space.
// The result has no leading or trailing newline and is suitable for use inside a subquery.
func (r *ArtifactTableRegistry) UnionAllSelect(cols ...string) string {
	entries := r.Entries()
	colList := strings.Join(cols, ", ")
	parts := make([]string, len(entries))
	for i, e := range entries {
		parts[i] = fmt.Sprintf("SELECT %s FROM %s", colList, e.Table)
	}
	return strings.Join(parts, "\n\t\t\t\tUNION ALL ")
}
