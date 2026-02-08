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

package xdsclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/kernel"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	policyenginev1 "github.com/wso2/api-platform/sdk/gateway/policyengine/v1"
)

// =============================================================================
// NewResourceHandler Tests
// =============================================================================

func TestNewResourceHandler(t *testing.T) {
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}

	handler := NewResourceHandler(k, reg)

	require.NotNil(t, handler)
	assert.NotNil(t, handler.kernel)
	assert.NotNil(t, handler.registry)
	assert.NotNil(t, handler.configLoader)
	assert.NotNil(t, handler.apiKeyHandler)
	assert.NotNil(t, handler.lazyResourceHandler)
}

// =============================================================================
// convertStoredConfigToPolicyChains Tests
// =============================================================================

func TestConvertStoredConfigToPolicyChains_Empty(t *testing.T) {
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}
	handler := NewResourceHandler(k, reg)

	stored := &StoredPolicyConfig{
		Configuration: policyenginev1.Configuration{
			Routes: []policyenginev1.PolicyChain{},
		},
	}

	result := handler.convertStoredConfigToPolicyChains(stored)

	assert.Empty(t, result)
}

func TestConvertStoredConfigToPolicyChains_MultipleRoutes(t *testing.T) {
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}
	handler := NewResourceHandler(k, reg)

	stored := &StoredPolicyConfig{
		ID: "test-api",
		Configuration: policyenginev1.Configuration{
			Routes: []policyenginev1.PolicyChain{
				{RouteKey: "route-1", Policies: []policyenginev1.PolicyInstance{}},
				{RouteKey: "route-2", Policies: []policyenginev1.PolicyInstance{}},
			},
		},
	}

	result := handler.convertStoredConfigToPolicyChains(stored)

	require.Len(t, result, 2)
	assert.Equal(t, "route-1", result[0].RouteKey)
	assert.Equal(t, "route-2", result[1].RouteKey)
}

// =============================================================================
// validatePolicyChainConfig Tests
// =============================================================================

func TestValidatePolicyChainConfig_EmptyRouteKey(t *testing.T) {
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}
	handler := NewResourceHandler(k, reg)

	config := &policyenginev1.PolicyChain{
		RouteKey: "",
	}

	err := handler.validatePolicyChainConfig(config)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "route_key is required")
}

func TestValidatePolicyChainConfig_PolicyMissingName(t *testing.T) {
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}
	handler := NewResourceHandler(k, reg)

	config := &policyenginev1.PolicyChain{
		RouteKey: "test-route",
		Policies: []policyenginev1.PolicyInstance{
			{Name: "", Version: "v1.0.0"},
		},
	}

	err := handler.validatePolicyChainConfig(config)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "policy[0]: name is required")
}

func TestValidatePolicyChainConfig_PolicyMissingVersion(t *testing.T) {
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}
	handler := NewResourceHandler(k, reg)

	config := &policyenginev1.PolicyChain{
		RouteKey: "test-route",
		Policies: []policyenginev1.PolicyInstance{
			{Name: "test-policy", Version: ""},
		},
	}

	err := handler.validatePolicyChainConfig(config)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "policy[0]: version is required")
}

func TestValidatePolicyChainConfig_PolicyNotInRegistry(t *testing.T) {
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}
	handler := NewResourceHandler(k, reg)

	config := &policyenginev1.PolicyChain{
		RouteKey: "test-route",
		Policies: []policyenginev1.PolicyInstance{
			{Name: "nonexistent-policy", Version: "v1.0.0"},
		},
	}

	err := handler.validatePolicyChainConfig(config)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "policy[0]")
}

func TestValidatePolicyChainConfig_NoPolicies(t *testing.T) {
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}
	handler := NewResourceHandler(k, reg)

	config := &policyenginev1.PolicyChain{
		RouteKey: "test-route",
		Policies: []policyenginev1.PolicyInstance{},
	}

	err := handler.validatePolicyChainConfig(config)

	assert.NoError(t, err) // Empty policy list is valid
}

// =============================================================================
// getAllRouteKeys Tests
// =============================================================================

func TestGetAllRouteKeys(t *testing.T) {
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}
	handler := NewResourceHandler(k, reg)

	// Currently returns empty slice as xDS State of the World sends all routes
	result := handler.getAllRouteKeys()

	assert.NotNil(t, result)
	assert.Empty(t, result)
}

// =============================================================================
// HandlePolicyChainUpdate Tests
// =============================================================================

func TestHandlePolicyChainUpdate_EmptyResources(t *testing.T) {
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}
	handler := NewResourceHandler(k, reg)

	ctx := context.Background()
	err := handler.HandlePolicyChainUpdate(ctx, []*anypb.Any{}, "v1")

	assert.NoError(t, err)
}

func TestHandlePolicyChainUpdate_WrongTypeURL(t *testing.T) {
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}
	handler := NewResourceHandler(k, reg)

	ctx := context.Background()
	resources := []*anypb.Any{
		{TypeUrl: "wrong.type.url", Value: []byte{}},
	}

	err := handler.HandlePolicyChainUpdate(ctx, resources, "v1")

	// Should skip resources with wrong type URL but not fail
	assert.NoError(t, err)
}

func TestHandlePolicyChainUpdate_InvalidInnerAny(t *testing.T) {
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}
	handler := NewResourceHandler(k, reg)

	ctx := context.Background()
	resources := []*anypb.Any{
		{TypeUrl: PolicyChainTypeURL, Value: []byte("invalid-protobuf")},
	}

	err := handler.HandlePolicyChainUpdate(ctx, resources, "v1")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal inner Any from resource")
}

func TestHandlePolicyChainUpdate_InvalidStructInInnerAny(t *testing.T) {
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}
	handler := NewResourceHandler(k, reg)

	// Create an inner Any with invalid Struct data
	innerAny := &anypb.Any{
		TypeUrl: "type.googleapis.com/google.protobuf.Struct",
		Value:   []byte("invalid-struct"),
	}
	innerAnyBytes, _ := proto.Marshal(innerAny)

	ctx := context.Background()
	resources := []*anypb.Any{
		{TypeUrl: PolicyChainTypeURL, Value: innerAnyBytes},
	}

	err := handler.HandlePolicyChainUpdate(ctx, resources, "v1")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal policy struct")
}

func TestHandlePolicyChainUpdate_ValidEmptyConfig(t *testing.T) {
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}
	handler := NewResourceHandler(k, reg)

	// Create a valid empty stored config
	storedConfig := map[string]interface{}{
		"id":      "test-api",
		"version": 1,
		"configuration": map[string]interface{}{
			"metadata": map[string]interface{}{
				"apiId":   "api-123",
				"apiName": "test-api",
				"version": "v1",
			},
			"routes": []interface{}{},
		},
	}

	policyStruct, err := structpb.NewStruct(storedConfig)
	require.NoError(t, err)

	structBytes, err := proto.Marshal(policyStruct)
	require.NoError(t, err)

	innerAny := &anypb.Any{
		TypeUrl: "type.googleapis.com/google.protobuf.Struct",
		Value:   structBytes,
	}
	innerAnyBytes, err := proto.Marshal(innerAny)
	require.NoError(t, err)

	ctx := context.Background()
	resources := []*anypb.Any{
		{TypeUrl: PolicyChainTypeURL, Value: innerAnyBytes},
	}

	err = handler.HandlePolicyChainUpdate(ctx, resources, "v1")

	assert.NoError(t, err)
}

func TestHandlePolicyChainUpdate_RouteWithInvalidPolicy(t *testing.T) {
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}
	handler := NewResourceHandler(k, reg)

	// Create config with a route that has a nonexistent policy
	storedConfig := map[string]interface{}{
		"id":      "test-api",
		"version": 1,
		"configuration": map[string]interface{}{
			"metadata": map[string]interface{}{
				"apiId":   "api-123",
				"apiName": "test-api",
				"version": "v1",
			},
			"routes": []interface{}{
				map[string]interface{}{
					"routeKey": "test-route",
					"policies": []interface{}{
						map[string]interface{}{
							"name":       "nonexistent-policy",
							"version":    "v1.0.0",
							"enabled":    true,
							"parameters": map[string]interface{}{},
						},
					},
				},
			},
		},
	}

	policyStruct, err := structpb.NewStruct(storedConfig)
	require.NoError(t, err)

	structBytes, err := proto.Marshal(policyStruct)
	require.NoError(t, err)

	innerAny := &anypb.Any{
		TypeUrl: "type.googleapis.com/google.protobuf.Struct",
		Value:   structBytes,
	}
	innerAnyBytes, err := proto.Marshal(innerAny)
	require.NoError(t, err)

	ctx := context.Background()
	resources := []*anypb.Any{
		{TypeUrl: PolicyChainTypeURL, Value: innerAnyBytes},
	}

	// Should not fail - skips invalid routes but continues
	err = handler.HandlePolicyChainUpdate(ctx, resources, "v1")
	assert.NoError(t, err)
}

func TestHandlePolicyChainUpdate_RouteWithEmptyKey(t *testing.T) {
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}
	handler := NewResourceHandler(k, reg)

	// Create config with a route that has empty key
	storedConfig := map[string]interface{}{
		"id":      "test-api",
		"version": 1,
		"configuration": map[string]interface{}{
			"metadata": map[string]interface{}{
				"apiId":   "api-123",
				"apiName": "test-api",
				"version": "v1",
			},
			"routes": []interface{}{
				map[string]interface{}{
					"routeKey": "",
					"policies": []interface{}{},
				},
			},
		},
	}

	policyStruct, err := structpb.NewStruct(storedConfig)
	require.NoError(t, err)

	structBytes, err := proto.Marshal(policyStruct)
	require.NoError(t, err)

	innerAny := &anypb.Any{
		TypeUrl: "type.googleapis.com/google.protobuf.Struct",
		Value:   structBytes,
	}
	innerAnyBytes, err := proto.Marshal(innerAny)
	require.NoError(t, err)

	ctx := context.Background()
	resources := []*anypb.Any{
		{TypeUrl: PolicyChainTypeURL, Value: innerAnyBytes},
	}

	// Should not fail - skips invalid routes (empty key)
	err = handler.HandlePolicyChainUpdate(ctx, resources, "v1")
	assert.NoError(t, err)
}
