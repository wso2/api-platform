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

package repository

import (
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/constants"
)

// TestAgentProxyKindIsAValidArtifactKind covers the first of the two independent
// registrations a new artifact kind needs. constants.ValidArtifactKinds gates the
// api-portal webhook's data.api.type and the subscription handler's kind check;
// a kind missing here is rejected there with no other signal.
//
// The second registration — an ArtifactTableRegistry entry for agent_proxies,
// which is what makes the kind visible to the UNION queries behind artifact
// listing, API keys, subscriptions and deployment listing — landed with the
// agent_proxies DDL, and is asserted by
// TestAgentProxyIsRegisteredInArtifactTableRegistry below. The two registrations
// are independent and both required.
func TestAgentProxyKindIsAValidArtifactKind(t *testing.T) {
	if !constants.ValidArtifactKinds[constants.AgentProxy] {
		t.Errorf("constants.ValidArtifactKinds is missing %q", constants.AgentProxy)
	}
}

// TestAgentProxyIsRegisteredInArtifactTableRegistry covers the second
// registration. Without the registry entry, agent_proxies is simply absent from
// every UNION the registry builds — artifact lookup by handle or UUID, the
// application/API-key/deployment listings — and an Agent Proxy is invisible to
// all of them with no error anywhere.
func TestAgentProxyIsRegisteredInArtifactTableRegistry(t *testing.T) {
	reg := NewArtifactTableRegistry()

	entry, ok := reg.TableByKindKey(constants.AgentProxy)
	if !ok {
		t.Fatalf("ArtifactTableRegistry has no entry for kind key %q", constants.AgentProxy)
	}
	if entry.Table != "agent_proxies" {
		t.Errorf("AgentProxy entry backs table %q, want %q", entry.Table, "agent_proxies")
	}
	// KindAlias is the value written to artifacts.type, so it must be the kind
	// constant itself — IsValidKindAlias gates artifact creation against it.
	if entry.KindAlias != constants.AgentProxy {
		t.Errorf("AgentProxy entry KindAlias = %q, want %q", entry.KindAlias, constants.AgentProxy)
	}
	if !reg.IsValidKindAlias(constants.AgentProxy) {
		t.Errorf("IsValidKindAlias(%q) = false; artifact creation for this kind would be rejected", constants.AgentProxy)
	}
	// The HTTP handle form is the other accepted lookup key, matching how the
	// other kinds register both spellings.
	if _, ok := reg.TableByKindKey("agent-proxy"); !ok {
		t.Error(`ArtifactTableRegistry has no entry for kind key "agent-proxy"`)
	}
}

// TestAgentProxiesParticipatesInUnionQueries asserts the registry's generated
// UNION fragment actually reaches agent_proxies, for the column sets the
// application, API-key and deployment repositories ask for. Each of those
// columns must exist on the table or the query fails at runtime on every kind,
// not just this one.
func TestAgentProxiesParticipatesInUnionQueries(t *testing.T) {
	reg := NewArtifactTableRegistry()

	for _, cols := range [][]string{
		{"uuid", "handle"},
		{"uuid", "handle", "origin"},
		{"uuid", "handle", "display_name", "version"},
		{"uuid", "handle", "display_name", "version", "created_at", "updated_at"},
	} {
		got := reg.UnionAllSelect(cols...)
		want := "SELECT " + strings.Join(cols, ", ") + " FROM agent_proxies"
		if !strings.Contains(got, want) {
			t.Errorf("UnionAllSelect(%v) does not select from agent_proxies; got:\n%s", cols, got)
		}
	}
}
