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

import "testing"

// TestNewArtifactTableRegistry_AllCoreKindsRegistered guards GraphQL's status
// as a core kind (like RestApi/LlmProvider/LlmProxy/Mcp): NewArtifactTableRegistry
// must register all five unconditionally, with no build tag or plugin Init()
// step able to skip any of them. A future kind silently dropped from this
// constructor would otherwise only surface as a runtime 404 on that kind's
// API-key/deployment/gateway-association endpoints — this test catches it at
// build time instead.
func TestNewArtifactTableRegistry_AllCoreKindsRegistered(t *testing.T) {
	reg := NewArtifactTableRegistry()

	wantKindAliases := []string{"RestApi", "LlmProvider", "LlmProxy", "Mcp", "GraphQLApi"}
	for _, alias := range wantKindAliases {
		if !reg.IsValidKindAlias(alias) {
			t.Errorf("expected core kind %q to be registered, but it wasn't", alias)
		}
	}

	entries := reg.Entries()
	if len(entries) != len(wantKindAliases) {
		t.Errorf("expected exactly %d core tables registered, got %d: %+v", len(wantKindAliases), len(entries), entries)
	}

	// GraphQLApi specifically: confirm both the handle form ("graphql-api")
	// and the Go-constant form ("GraphQLApi") resolve to the graphql_apis
	// table, matching every other core kind's dual-key convention.
	for _, key := range []string{"graphql-api", "GraphQLApi"} {
		entry, ok := reg.TableByKindKey(key)
		if !ok {
			t.Fatalf("expected kind key %q to resolve to a table entry", key)
		}
		if entry.Table != "graphql_apis" {
			t.Errorf("expected kind key %q to resolve to table \"graphql_apis\", got %q", key, entry.Table)
		}
	}
}
