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

package utils

import (
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/constants"
)

// The import order is keyed by the kind the gateway pushes. A gateway Agent is
// ranked explicitly — after every kind it could conceivably depend on — while the
// control-plane kind AgentProxy is never a pushed kind and so has no rank of its
// own; it falls to the unknown-kind position.
func TestArtifactImportRank_AgentIsRankedByGatewayKind(t *testing.T) {
	unknown := ArtifactImportRank("NotAKind")
	agent := ArtifactImportRank(constants.GatewayKindAgent)

	if agent == unknown {
		t.Fatalf("ArtifactImportRank(%q) = %d, the unknown-kind fallback; want an explicit rank", constants.GatewayKindAgent, agent)
	}
	for _, kind := range []string{
		constants.LLMProviderTemplate, constants.LLMProvider, constants.LLMProxy,
		constants.MCPProxy, constants.RestApi, constants.WebSubApi, constants.WebBrokerApi,
	} {
		if r := ArtifactImportRank(kind); r >= agent {
			t.Errorf("ArtifactImportRank(%q) = %d, want it ahead of Agent (%d)", kind, r, agent)
		}
	}
	if r := ArtifactImportRank(constants.AgentProxy); r != unknown {
		t.Errorf("ArtifactImportRank(%q) = %d, want the unknown-kind rank %d: it is not a gateway kind", constants.AgentProxy, r, unknown)
	}
}
