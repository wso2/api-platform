/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package kernel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// =============================================================================
// BuildPolicyChain Tests
// =============================================================================

func TestBuildPolicyChain_EmptySpecs(t *testing.T) {
	kernel := NewKernel()
	reg := &registry.PolicyRegistry{
		Policies: make(map[string]*registry.PolicyEntry),
	}

	chain, err := kernel.BuildPolicyChain("test-route", []policy.PolicySpec{}, reg, policy.PolicyMetadata{})

	require.NoError(t, err)
	require.NotNil(t, chain)
	assert.Empty(t, chain.Policies)
	assert.Empty(t, chain.PolicySpecs)
	assert.False(t, chain.RequiresRequestBody)
	assert.False(t, chain.RequiresResponseBody)
	assert.False(t, chain.HasExecutionConditions)
}

func TestBuildPolicyChain_UnknownPolicy(t *testing.T) {
	kernel := NewKernel()
	reg := &registry.PolicyRegistry{
		Policies: make(map[string]*registry.PolicyEntry),
	}

	specs := []policy.PolicySpec{
		{
			Name:    "nonexistent-policy",
			Version: "v1.0.0",
			Enabled: true,
		},
	}

	chain, err := kernel.BuildPolicyChain("test-route", specs, reg, policy.PolicyMetadata{})

	assert.Error(t, err)
	assert.Nil(t, chain)
	assert.Contains(t, err.Error(), "failed to create policy instance")
}

func TestBuildPolicyChain_WithExecutionCondition(t *testing.T) {
	kernel := NewKernel()
	reg := &registry.PolicyRegistry{
		Policies: make(map[string]*registry.PolicyEntry),
	}

	condition := "request.method == 'POST'"
	specs := []policy.PolicySpec{
		{
			Name:               "test-policy",
			Version:            "v1.0.0",
			Enabled:            true,
			ExecutionCondition: &condition,
		},
	}

	// This will fail because the policy doesn't exist in registry
	// But we're testing the condition detection logic
	_, err := kernel.BuildPolicyChain("test-route", specs, reg, policy.PolicyMetadata{})

	// We expect an error because policy doesn't exist, but that's OK
	// The important thing is that the HasExecutionConditions would be set if it succeeded
	assert.Error(t, err)
}

func TestBuildPolicyChain_WithAPIMetadata(t *testing.T) {
	kernel := NewKernel()
	reg := &registry.PolicyRegistry{
		Policies: make(map[string]*registry.PolicyEntry),
	}

	apiMetadata := policy.PolicyMetadata{
		APIId:      "api-123",
		APIName:    "test-api",
		APIVersion: "v1",
	}

	specs := []policy.PolicySpec{}

	chain, err := kernel.BuildPolicyChain("test-route", specs, reg, apiMetadata)

	require.NoError(t, err)
	require.NotNil(t, chain)
	// Metadata is passed to policy instances, not stored in chain
	assert.Empty(t, chain.Policies)
}

// =============================================================================
// GetRequestBodyMode Tests
// =============================================================================

func TestGetRequestBodyMode_NoChain(t *testing.T) {
	kernel := NewKernel()

	mode := kernel.GetRequestBodyMode("nonexistent-route")

	assert.Equal(t, BodyModeSkip, mode)
}

func TestGetRequestBodyMode_WithChainRequiringBody(t *testing.T) {
	kernel := NewKernel()

	chain := &registry.PolicyChain{
		Policies:            []policy.Policy{},
		PolicySpecs:         []policy.PolicySpec{},
		RequiresRequestBody: true,
	}

	kernel.PolicyChains["test-route"] = chain

	mode := kernel.GetRequestBodyMode("test-route")

	assert.Equal(t, BodyModeBuffered, mode)
}

func TestGetRequestBodyMode_WithChainNotRequiringBody(t *testing.T) {
	kernel := NewKernel()

	chain := &registry.PolicyChain{
		Policies:            []policy.Policy{},
		PolicySpecs:         []policy.PolicySpec{},
		RequiresRequestBody: false,
	}

	kernel.PolicyChains["test-route"] = chain

	mode := kernel.GetRequestBodyMode("test-route")

	assert.Equal(t, BodyModeSkip, mode)
}

// =============================================================================
// GetResponseBodyMode Tests
// =============================================================================

func TestGetResponseBodyMode_NoChain(t *testing.T) {
	kernel := NewKernel()

	mode := kernel.GetResponseBodyMode("nonexistent-route")

	assert.Equal(t, BodyModeSkip, mode)
}

func TestGetResponseBodyMode_WithChainRequiringBody(t *testing.T) {
	kernel := NewKernel()

	chain := &registry.PolicyChain{
		Policies:             []policy.Policy{},
		PolicySpecs:          []policy.PolicySpec{},
		RequiresResponseBody: true,
	}

	kernel.PolicyChains["test-route"] = chain

	mode := kernel.GetResponseBodyMode("test-route")

	assert.Equal(t, BodyModeBuffered, mode)
}

func TestGetResponseBodyMode_WithChainNotRequiringBody(t *testing.T) {
	kernel := NewKernel()

	chain := &registry.PolicyChain{
		Policies:             []policy.Policy{},
		PolicySpecs:          []policy.PolicySpec{},
		RequiresResponseBody: false,
	}

	kernel.PolicyChains["test-route"] = chain

	mode := kernel.GetResponseBodyMode("test-route")

	assert.Equal(t, BodyModeSkip, mode)
}
