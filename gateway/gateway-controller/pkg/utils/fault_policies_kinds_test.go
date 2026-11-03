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

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

// Every non-REST kind is normalised into a RestAPI before the runtime transformer builds
// route chains from it. So the ONE thing each conversion has to do for fault policies is
// carry the list across — everything downstream (chain building, the engine, protocol
// rendering) is already kind-agnostic.
//
// These tests exist because that carrying is a single assignment per conversion, which is
// exactly the kind of line that gets dropped in a refactor and produces a silent
// "my fault policy never runs" with nothing to grep for.

func faultList() *[]api.Policy {
	return &[]api.Policy{
		{Name: "fault-notifier", Version: "v1"},
		{Name: "error-formatter", Version: "v1"},
	}
}

func assertCarried(t *testing.T, got *[]api.Policy, kind string) {
	t.Helper()
	require.NotNil(t, got, "%s: fault policies were dropped during normalisation", kind)
	require.Len(t, *got, 2, "%s: wrong number of fault entries", kind)
	assert.Equal(t, "fault-notifier", (*got)[0].Name, "%s: order must be preserved", kind)
	assert.Equal(t, "error-formatter", (*got)[1].Name, "%s: order must be preserved", kind)
}

func TestMCPTransformer_CarriesFaultPolicies(t *testing.T) {
	mcp := &api.MCPProxyConfiguration{}
	mcp.Spec.FaultPolicies = faultList()

	var out api.RestAPI
	got, err := (&MCPTransformer{}).Transform(mcp, &out)
	require.NoError(t, err)
	assertCarried(t, got.Spec.FaultPolicies, "Mcp")
}

// And absence must stay absence rather than becoming an empty slice: an API that declares no
// fault policies must produce the same chain it did before the field existed.
func TestMCPTransformer_NoFaultPoliciesStaysNil(t *testing.T) {
	var out api.RestAPI
	got, err := (&MCPTransformer{}).Transform(&api.MCPProxyConfiguration{}, &out)
	require.NoError(t, err)
	assert.Nil(t, got.Spec.FaultPolicies,
		"an unset list must stay unset, not become empty")
}

// The LLM provider path, using the same fixture shape the transformer's own tests use.
func TestLLMProviderTransformer_CarriesFaultPolicies(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "fault-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
			},
			AccessControl:       api.LLMAccessControl{Mode: api.AllowAll},
			GlobalFaultPolicies: faultList(),
		},
	}

	out := &api.RestAPI{}
	got, err := transformer.Transform(provider, out)
	require.NoError(t, err)
	assertCarried(t, got.Spec.FaultPolicies, "LlmProvider")
}

// Fault policies must NOT be merged into the normal policy list. An entry that reached
// Policies would run on every successful response, which is the one thing the separate
// field exists to prevent.
func TestLLMProviderTransformer_FaultPoliciesStayOutOfTheNormalChain(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "fault-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
			},
			AccessControl:       api.LLMAccessControl{Mode: api.AllowAll},
			GlobalFaultPolicies: faultList(),
		},
	}

	out := &api.RestAPI{}
	got, err := transformer.Transform(provider, out)
	require.NoError(t, err)

	if got.Spec.Policies != nil {
		for _, pol := range *got.Spec.Policies {
			assert.NotEqual(t, "fault-notifier", pol.Name,
				"a fault entry leaked into the normal policy list")
			assert.NotEqual(t, "error-formatter", pol.Name,
				"a fault entry leaked into the normal policy list")
		}
	}
}

// Operation-scoped fault entries on an LLM kind.
//
// The LLM kinds synthesize their operations rather than taking them from the spec, so
// "attach this fault entry to /chat/completions" cannot be a straight copy the way it is for
// a RestApi — the operation does not exist until the transform has run. These tests pin the
// two properties that follow from that.

func llmProviderWithOperationFaults(faults []api.OperationPolicy) *api.LLMProviderConfiguration {
	return &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "fault-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
			},
			AccessControl:          api.LLMAccessControl{Mode: api.AllowAll},
			OperationFaultPolicies: &faults,
		},
	}
}

func TestLLMProvider_OperationFaultPoliciesAttachToNamedPath(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	provider := llmProviderWithOperationFaults([]api.OperationPolicy{{
		Name:    "fault-notifier",
		Version: "v1",
		Paths: []api.OperationPolicyPath{{
			Path:    "/chat/completions",
			Methods: []api.OperationPolicyPathMethods{"POST"},
		}},
	}})

	got, err := transformer.Transform(provider, &api.RestAPI{})
	require.NoError(t, err)

	// An allow_all provider derives only catch-all `/*` operations, so a specific path has no
	// operation to attach to until the transform materializes one — exactly as it does for a
	// normal operationPolicies entry. Asserting on `/*` instead would pass for the wrong
	// reason: the catch-all matches directly and the materialization is never exercised. An
	// earlier version of this test did exactly that, and hid the fact that the feature did not
	// work for any specific path.
	var target *api.Operation
	for i := range got.Spec.Operations {
		op := &got.Spec.Operations[i]
		if op.EffectivePath() == "/chat/completions" {
			target = op
		}
	}
	require.NotNil(t, target, "the named path was never materialized")
	require.NotNil(t, target.FaultPolicies)
	require.Len(t, *target.FaultPolicies, 1)
	assert.Equal(t, "fault-notifier", (*target.FaultPolicies)[0].Name)
	require.NotNil(t, target.Method)
	assert.Equal(t, "POST", string(*target.Method))
}

// Scoping: the entry lands ONLY on the path it named. The catch-all operations that serve
// every other path must stay clean, or an operation-scoped handler would fire API-wide.
func TestLLMProvider_OperationFaultPoliciesDoNotLeakToOtherRoutes(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	got, err := transformer.Transform(llmProviderWithOperationFaults([]api.OperationPolicy{{
		Name:    "fault-notifier",
		Version: "v1",
		Paths: []api.OperationPolicyPath{{
			Path:    "/chat/completions",
			Methods: []api.OperationPolicyPathMethods{"POST"},
		}},
	}}), &api.RestAPI{})
	require.NoError(t, err)

	for i := range got.Spec.Operations {
		op := &got.Spec.Operations[i]
		if op.EffectivePath() == "/chat/completions" {
			continue
		}
		assert.Nil(t, op.FaultPolicies,
			"an operation-scoped fault entry reached %s %s", *op.Method, op.EffectivePath())
	}
}

func TestLLMProvider_NoOperationFaultPoliciesLeavesOperationsUntouched(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	got, err := transformer.Transform(llmProviderWithOperationFaults(nil), &api.RestAPI{})
	require.NoError(t, err)
	for _, op := range got.Spec.Operations {
		assert.Nil(t, op.FaultPolicies)
	}
}

// Both levels together on an LLM kind: the API-level list lands on the derived spec and the
// operation-level entries land on the operations. Those are exactly the two inputs the REST
// transformer's mergeFaultPolicies consumes — and it puts the operation's first, which is what
// makes resource-level precede API-level for an LLM kind without any LLM-specific ordering
// code. The ordering itself is pinned by TestRestAPITransformer_FaultPoliciesRecordsAttachedLevel.
func TestLLMProvider_BothFaultLevelsReachTheDerivedRestAPI(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	provider := llmProviderWithOperationFaults([]api.OperationPolicy{{
		Name:    "error-formatter",
		Version: "v1",
		Paths: []api.OperationPolicyPath{{
			Path:    "/*",
			Methods: []api.OperationPolicyPathMethods{"POST"},
		}},
	}})
	provider.Spec.GlobalFaultPolicies = &[]api.Policy{{Name: "fault-notifier", Version: "v1"}}

	got, err := transformer.Transform(provider, &api.RestAPI{})
	require.NoError(t, err)

	// API level: carried across under the RestApi's own spelling.
	require.NotNil(t, got.Spec.FaultPolicies)
	require.Len(t, *got.Spec.FaultPolicies, 1)
	assert.Equal(t, "fault-notifier", (*got.Spec.FaultPolicies)[0].Name)

	// Operation level: on the operations, and never merged into the API-level list — that
	// merge is the REST transformer's job, per route.
	found := false
	for _, op := range got.Spec.Operations {
		if op.FaultPolicies == nil {
			continue
		}
		found = true
		require.Len(t, *op.FaultPolicies, 1)
		assert.Equal(t, "error-formatter", (*op.FaultPolicies)[0].Name)
	}
	assert.True(t, found, "the operation-level entry reached no operation")
	assert.Len(t, *got.Spec.FaultPolicies, 1,
		"the operation-level entry must not have leaked into the API-level list")
}
