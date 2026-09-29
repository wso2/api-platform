/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package transform

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/chainkey"
	"github.com/wso2/api-platform/gateway/common/agentproto"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

func agentFaultList(names ...string) *[]api.Policy {
	out := make([]api.Policy, 0, len(names))
	for _, n := range names {
		out = append(out, api.Policy{Name: n, Version: "v1"})
	}
	return &out
}

func withAgentFaultPolicies(names ...string) agentOption {
	return func(cfg *api.AgentConfiguration) {
		cfg.Spec.A2a.OperationConfigs.FaultPolicies = agentFaultList(names...)
	}
}

func withAgentOperationFaultPolicies(op agentproto.Operation, names ...string) agentOption {
	return func(cfg *api.AgentConfiguration) {
		cfg.Spec.A2a.OperationConfigs.Operations = &[]api.A2AOperationConfig{
			{Name: api.A2AOperationName(op), FaultPolicies: agentFaultList(names...)},
		}
	}
}

func withCardFaultPolicies(names ...string) agentOption {
	return func(cfg *api.AgentConfiguration) {
		cfg.Spec.A2a.AgentCard.Public.FaultPolicies = agentFaultList(names...)
	}
}

func agentChain(t *testing.T, rdc *models.RuntimeDeployConfig, op agentproto.Operation) *models.PolicyChain {
	t.Helper()
	chain := rdc.PolicyChains[chainkey.For(testAgentUUID, "main.local", string(op))]
	require.NotNil(t, chain, "no chain for operation %s", op)
	return chain
}

// An Agent declares fault policies at the same two levels every other kind does, and they
// merge the same way: the operation's entries first, then the agent's. Asserted on the chain
// the RESOLVER will bind — an A2A operation chain is filed under a composed key, not a route
// key, so a list attached to the wrong one would deploy cleanly and never run.
func TestAgentTransformer_FaultPoliciesMergeBothLevels(t *testing.T) {
	rdc, err := agentTransformerWithPolicies("set-headers", "remove-headers").Transform(
		testAgent(
			withAgentFaultPolicies("set-headers"),
			withAgentOperationFaultPolicies(agentproto.SendMessage, "remove-headers"),
		))
	require.NoError(t, err)

	assert.Equal(t, []string{"remove-headers", "set-headers"},
		faultNames(agentChain(t, rdc, agentproto.SendMessage)),
		"the operation's entries run before the agent's")

	// An operation that declared none still gets the agent's.
	assert.Equal(t, []string{"set-headers"},
		faultNames(agentChain(t, rdc, agentproto.GetTask)),
		"an operation with no list of its own still runs the agent-wide one")
}

// Declaring neither level leaves the list empty rather than absent-but-populated, which is
// what keeps an Agent that wants no fault handling paying for none.
func TestAgentTransformer_NoFaultPoliciesLeavesTheChainEmpty(t *testing.T) {
	rdc, err := agentTransformer().Transform(testAgent())
	require.NoError(t, err)

	assert.Empty(t, faultNames(agentChain(t, rdc, agentproto.SendMessage)))
}

// The card's list is its own. Discovery is reachable before any operation is invoked, so an
// agent that guards its operations while leaving its card open must not have the operations'
// handlers fire for a discovery failure — and the reverse, an operation must not inherit the
// card's.
func TestAgentTransformer_CardFaultPoliciesDoNotCrossWithOperations(t *testing.T) {
	rdc, err := agentTransformerWithPolicies("set-headers", "remove-headers").Transform(
		testAgent(
			withAgentFaultPolicies("set-headers"),
			withCardFaultPolicies("remove-headers"),
		))
	require.NoError(t, err)

	assert.Equal(t, []string{"set-headers"},
		faultNames(agentChain(t, rdc, agentproto.SendMessage)),
		"an operation must not inherit the card's handlers")

	var cardChain *models.PolicyChain
	for key, chain := range rdc.PolicyChains {
		if key == "GET|/weather/.well-known/agent-card.json|main.local" {
			cardChain = chain
		}
	}
	require.NotNil(t, cardChain, "the card route must have a chain")
	assert.Equal(t, []string{"remove-headers"}, faultNames(cardChain),
		"the card runs its own handlers and not the operations'")
}
