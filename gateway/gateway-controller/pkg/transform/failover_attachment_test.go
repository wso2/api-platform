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

	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

const attachmentRouteKey = "GET|/test/hello|main.local"

func failoverPolicyWithTarget(model string) api.Policy {
	return api.Policy{Name: FailoverPolicyName, Version: "v0", Params: &map[string]interface{}{
		"targets": []interface{}{map[string]interface{}{
			"model":     model,
			"fallbacks": []interface{}{map[string]interface{}{"model": model + "-mini"}},
		}},
	}}
}

func attachmentTransformer() *RestAPITransformer {
	return NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{
		FailoverPolicyName + "|v0.1.0":    {Name: FailoverPolicyName, Version: "v0.1.0"},
		"intelligent-model-routing|v1.0.0": {Name: "intelligent-model-routing", Version: "v1.0.0"},
	})
}

func chainNames(t *testing.T, rdc *models.RuntimeDeployConfig) []string {
	t.Helper()
	chain, ok := rdc.PolicyChains[attachmentRouteKey]
	require.True(t, ok, "no policy chain for %s", attachmentRouteKey)
	names := make([]string, 0, len(chain.Policies))
	for _, p := range chain.Policies {
		names = append(names, p.Name)
	}
	return names
}

func countNamed(names []string, want string) int {
	n := 0
	for _, name := range names {
		if name == want {
			n++
		}
	}
	return n
}

// An API-level attachment must generate the same route configuration an operation-level one
// does. Before FR-030r the route's Failover was nil while the policy still sat in the executing
// chain, so the policy ran, cost a gRPC round trip and forced request-body buffering, while
// Envoy had no retry policy and no priority_group plugin to act on anything it published.
func TestFailoverAttachment_APILevelProducesRouteConfiguration(t *testing.T) {
	rdc, err := attachmentTransformer().Transform(
		makeRestAPIStoredConfig([]api.Policy{failoverPolicyWithTarget("gpt-4o")}, nil))
	require.NoError(t, err)

	route := rdc.Routes[attachmentRouteKey]
	require.NotNil(t, route)
	require.NotNil(t, route.Failover, "an API-level attachment must resolve a failover configuration")
	require.Len(t, route.Failover.Targets, 2, "both chain members must resolve")
	require.Equal(t, "gpt-4o", route.Failover.Targets[0].Model)
	require.Equal(t, "gpt-4o-mini", route.Failover.Targets[1].Model)
}

// The ordering correction is gated on a resolved failover configuration, so the nil above also
// switched it off: failover landed FIRST, because API-level entries are prepended to the chain.
// That is the exact inversion the correction exists to prevent — the gateway runs every policy's
// request-header phase before any policy's body phase, so a body-phase routing policy running
// after failover publishes its decision too late to be read.
func TestFailoverAttachment_APILevelIsStillMovedLast(t *testing.T) {
	rdc, err := attachmentTransformer().Transform(makeRestAPIStoredConfig(
		[]api.Policy{failoverPolicyWithTarget("gpt-4o")},
		[]api.Policy{{Name: "intelligent-model-routing", Version: "v1"}},
	))
	require.NoError(t, err)

	require.Equal(t,
		[]string{"intelligent-model-routing", FailoverPolicyName},
		chainNames(t, rdc),
		"an API-level attachment must still be moved last, after the routing policy it consumes")
}

// A route carries ONE failover configuration, so two attachments cannot compose. The API-level
// instance is discarded rather than kept alongside: two instances hold different targets, resolve
// different chains, and both grade the response — and grading is not idempotent, so the duplicate
// charges one request's outcome to a target's health twice.
func TestFailoverAttachment_OperationOverridesAPILevelAndLeavesOneInstance(t *testing.T) {
	rdc, err := attachmentTransformer().Transform(makeRestAPIStoredConfig(
		[]api.Policy{failoverPolicyWithTarget("global-model")},
		[]api.Policy{failoverPolicyWithTarget("operation-model")},
	))
	require.NoError(t, err)

	require.Equal(t, 1, countNamed(chainNames(t, rdc), FailoverPolicyName),
		"both levels attached; the route must carry exactly one model-failover instance")

	route := rdc.Routes[attachmentRouteKey]
	require.NotNil(t, route.Failover)
	require.Equal(t, "operation-model", route.Failover.Targets[0].Model,
		"the operation's own attachment must win over the API-level one")

	chain := rdc.PolicyChains[attachmentRouteKey]
	for _, p := range chain.Policies {
		if p.Name == FailoverPolicyName {
			targets, _ := p.Params["targets"].([]interface{})
			require.Len(t, targets, 1)
			first, _ := targets[0].(map[string]interface{})
			require.Equal(t, "operation-model", first["model"],
				"the surviving instance must be the operation's, not the API's")
		}
	}
}

// An operation that writes no failover of its own is still covered by the API-level attachment —
// that is the whole point of attaching it there.
func TestFailoverAttachment_APILevelCoversOperationsThatDoNotOverride(t *testing.T) {
	attachment := ResolveFailoverAttachment(
		&[]api.Policy{failoverPolicyWithTarget("gpt-4o")}, nil)
	require.True(t, attachment.Attached())
	require.True(t, attachment.APILevel)
	require.False(t, attachment.Overridden)

	none := ResolveFailoverAttachment(nil, &[]api.Policy{{Name: "model-round-robin", Version: "v1"}})
	require.False(t, none.Attached(), "neither level attaching must stay 'not attached'")
	require.Nil(t, none.Policies)
}

// The per-attempt upstream instance is installed from the API-level attachment too. Without it
// the downstream instance publishes a ranking that nothing upstream applies.
func TestPerAttemptPolicies_InstalledFromAPILevelAttachment(t *testing.T) {
	apiLevel := []api.Policy{{Name: FailoverPolicyName, Version: "v0"}}
	ops := []api.Operation{{Policies: &[]api.Policy{{Name: "model-round-robin", Version: "v1"}}}}

	got := perAttemptPolicies(&apiLevel, &ops)
	require.Len(t, got, 1, "an API-level attachment must install the per-attempt instance")
	require.Equal(t, FailoverPolicyName, got[0].Name)
	require.Equal(t, "v0", got[0].Version, "the publisher's own version, so the two halves cannot drift")

	// Still exactly one when both levels attach, for the same reason as the chain.
	both := perAttemptPolicies(&apiLevel, &[]api.Operation{
		{Policies: &[]api.Policy{{Name: FailoverPolicyName, Version: "v0"}}},
	})
	require.Len(t, both, 1)
}

// The fault-chain entry is the third artifact an attachment generates, and it is cloned from the
// resolved chain entry — so it follows the same nil that governed the other two. With
// handle_upstream_faults enabled the engine runs the fault chain instead of the response
// policies for a response of 400 or above, which for this policy is exactly the set it grades;
// without this entry an API-level attachment would stop recording health the moment that flag
// was turned on, and no target could ever be suspended.
func TestFailoverAttachment_APILevelGetsTheFaultChainEntry(t *testing.T) {
	rdc, err := attachmentTransformer().Transform(
		makeRestAPIStoredConfig([]api.Policy{failoverPolicyWithTarget("gpt-4o")}, nil))
	require.NoError(t, err)

	chain, ok := rdc.PolicyChains[attachmentRouteKey]
	require.True(t, ok)
	require.NotEmpty(t, chain.FaultPolicies, "an API-level attachment must still get a fault entry")
	require.Equal(t, FailoverPolicyName, chain.FaultPolicies[0].Name,
		"model-failover must be FIRST on the fault chain, so the exhaustion error stays a "+
			"default an operator's own entries can reshape")
}
