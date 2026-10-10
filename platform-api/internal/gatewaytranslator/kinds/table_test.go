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

package kinds

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/gwversion"
)

// Every definition is keyed by the gateway kind it declares, every Below and
// MinGatewayVersion is an LTS version or gwversion.NoLTSRelease, and every step
// has a name — the
// invariants the engine and the doc rely on.
func TestAll_DefinitionsAreWellFormed(t *testing.T) {
	require.NotEmpty(t, All)
	for key, k := range All {
		assert.Equalf(t, key, k.GatewayKind, "table key and GatewayKind must agree")
		if k.MinGatewayVersion != "" {
			assert.Truef(t, isMinimum(k.MinGatewayVersion), "%s: MinGatewayVersion %q", key, k.MinGatewayVersion)
		}
		for i, s := range k.Steps {
			assert.Truef(t, isMinimum(s.Below), "%s step %d: Below %q", key, i, s.Below)
			assert.NotEmptyf(t, s.Name, "%s step %d has no name", key, i)
			assert.NotNilf(t, s.Apply, "%s step %d has no Apply", key, i)
		}
	}
}

func TestAll_AgentIsUnderItsGatewayKind(t *testing.T) {
	_, ok := Lookup(constants.GatewayKindAgent)
	assert.True(t, ok)
	_, ok = Lookup(constants.AgentProxy)
	assert.False(t, ok, "the translator consumes gateway documents; AgentProxy is not one")
}

func TestLookup_Miss(t *testing.T) {
	_, ok := Lookup("SomeFutureKind")
	assert.False(t, ok)
}

// Kinds that only exist from a later gateway release say so; the rest run
// everywhere.
func TestAll_MinimumGatewayVersions(t *testing.T) {
	assert.Equal(t, gwversion.MinAgentKindGatewayVersion, All[constants.GatewayKindAgent].MinGatewayVersion)
	assert.Equal(t, gwversion.MinWebBrokerKindGatewayVersion, All[constants.WebBrokerApi].MinGatewayVersion)
	for _, kind := range []string{constants.RestApi, constants.MCPProxy, constants.LLMProvider, constants.LLMProxy, constants.WebSubApi} {
		assert.Emptyf(t, All[kind].MinGatewayVersion, "%s exists on every gateway release", kind)
	}
}

// isMinimum reports whether min is usable as a capability minimum.
func isMinimum(min string) bool {
	if min == gwversion.NoLTSRelease {
		return true
	}
	v, ok := gwversion.Parse(min)
	return ok && v.IsLTS()
}
