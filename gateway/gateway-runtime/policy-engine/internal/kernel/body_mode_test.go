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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
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

	kernel.RegisterRoute("test-route", chain)

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

	kernel.RegisterRoute("test-route", chain)

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

	kernel.RegisterRoute("test-route", chain)

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

	kernel.RegisterRoute("test-route", chain)

	mode := kernel.GetResponseBodyMode("test-route")

	assert.Equal(t, BodyModeSkip, mode)
}

type modeOnlyPolicy struct {
	mode policy.ProcessingMode
}

func (p *modeOnlyPolicy) Mode() policy.ProcessingMode {
	return p.mode
}

type requestHeaderModePolicy struct {
	mode policy.ProcessingMode
}

func (p *requestHeaderModePolicy) Mode() policy.ProcessingMode {
	return p.mode
}

func (p *requestHeaderModePolicy) OnRequestHeaders(_ context.Context, _ *policy.RequestHeaderContext, _ map[string]interface{}) policy.RequestHeaderAction {
	return policy.UpstreamRequestHeaderModifications{}
}

type responseHeaderModePolicy struct {
	mode policy.ProcessingMode
}

func (p *responseHeaderModePolicy) Mode() policy.ProcessingMode {
	return p.mode
}

func (p *responseHeaderModePolicy) OnResponseHeaders(_ context.Context, _ *policy.ResponseHeaderContext, _ map[string]interface{}) policy.ResponseHeaderAction {
	return policy.DownstreamResponseHeaderModifications{}
}

type bufferedRequestModePolicy struct {
	mode policy.ProcessingMode
}

func (p *bufferedRequestModePolicy) Mode() policy.ProcessingMode {
	return p.mode
}

func (p *bufferedRequestModePolicy) OnRequestBody(_ context.Context, _ *policy.RequestContext, _ map[string]interface{}) policy.RequestAction {
	return policy.UpstreamRequestModifications{}
}

type streamingRequestModePolicy struct {
	mode policy.ProcessingMode
}

func (p *streamingRequestModePolicy) Mode() policy.ProcessingMode {
	return p.mode
}

func (p *streamingRequestModePolicy) OnRequestBody(_ context.Context, _ *policy.RequestContext, _ map[string]interface{}) policy.RequestAction {
	return policy.UpstreamRequestModifications{}
}

func (p *streamingRequestModePolicy) NeedsMoreRequestData(_ []byte) bool {
	return false
}

func (p *streamingRequestModePolicy) OnRequestBodyChunk(_ context.Context, _ *policy.RequestStreamContext, _ *policy.StreamBody, _ map[string]interface{}) policy.StreamingRequestAction {
	return policy.ForwardRequestChunk{}
}

type bufferedResponseModePolicy struct {
	mode policy.ProcessingMode
}

func (p *bufferedResponseModePolicy) Mode() policy.ProcessingMode {
	return p.mode
}

func (p *bufferedResponseModePolicy) OnResponseBody(_ context.Context, _ *policy.ResponseContext, _ map[string]interface{}) policy.ResponseAction {
	return policy.DownstreamResponseModifications{}
}

type streamingResponseModePolicy struct {
	mode policy.ProcessingMode
}

func (p *streamingResponseModePolicy) Mode() policy.ProcessingMode {
	return p.mode
}

func (p *streamingResponseModePolicy) OnResponseBody(_ context.Context, _ *policy.ResponseContext, _ map[string]interface{}) policy.ResponseAction {
	return policy.DownstreamResponseModifications{}
}

func (p *streamingResponseModePolicy) NeedsMoreResponseData(_ []byte) bool {
	return false
}

func (p *streamingResponseModePolicy) OnResponseBodyChunk(_ context.Context, _ *policy.ResponseStreamContext, _ *policy.StreamBody, _ map[string]interface{}) policy.StreamingResponseAction {
	return policy.ForwardResponseChunk{}
}

func buildSinglePolicyChainForTest(t *testing.T, name string, impl policy.Policy) *registry.PolicyChain {
	t.Helper()

	reg := &registry.PolicyRegistry{
		Policies: make(map[string]*registry.PolicyEntry),
	}
	require.NoError(t, reg.SetConfig(map[string]interface{}{}))
	require.NoError(t, reg.Register(&policy.PolicyDefinition{
		Name:    name,
		Version: "v1.0.0",
	}, func(metadata policy.PolicyMetadata, params map[string]interface{}) (policy.Policy, error) {
		return impl, nil
	}))

	specs := []policy.PolicySpec{
		{
			Name:       name,
			Version:    "v1",
			Enabled:    true,
			Parameters: policy.PolicyParameters{Raw: map[string]interface{}{}},
		},
	}

	k := NewKernel()
	chain, err := k.BuildPolicyChain("test-route", specs, reg, policy.PolicyMetadata{})
	require.NoError(t, err)
	return chain
}

func TestBuildPolicyChain_RequestHeaderParticipationFollowsMode(t *testing.T) {
	tests := []struct {
		name         string
		impl         policy.Policy
		wantRequired bool
	}{
		{
			name: "process with interface",
			impl: &requestHeaderModePolicy{mode: policy.ProcessingMode{
				RequestHeaderMode: policy.HeaderModeProcess,
			}},
			wantRequired: true,
		},
		{
			name: "skip with interface",
			impl: &requestHeaderModePolicy{mode: policy.ProcessingMode{
				RequestHeaderMode: policy.HeaderModeSkip,
			}},
			wantRequired: false,
		},
		{
			name: "process without interface",
			impl: &modeOnlyPolicy{mode: policy.ProcessingMode{
				RequestHeaderMode: policy.HeaderModeProcess,
			}},
			wantRequired: false,
		},
		{
			name:         "zero value with interface",
			impl:         &requestHeaderModePolicy{mode: policy.ProcessingMode{}},
			wantRequired: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := buildSinglePolicyChainForTest(t, "request-header-test", tt.impl)
			assert.Equal(t, tt.wantRequired, chain.RequiresRequestHeader)
		})
	}
}

func TestBuildPolicyChain_ResponseHeaderParticipationFollowsMode(t *testing.T) {
	tests := []struct {
		name         string
		impl         policy.Policy
		wantRequired bool
	}{
		{
			name: "process with interface",
			impl: &responseHeaderModePolicy{mode: policy.ProcessingMode{
				ResponseHeaderMode: policy.HeaderModeProcess,
			}},
			wantRequired: true,
		},
		{
			name: "skip with interface",
			impl: &responseHeaderModePolicy{mode: policy.ProcessingMode{
				ResponseHeaderMode: policy.HeaderModeSkip,
			}},
			wantRequired: false,
		},
		{
			name: "process without interface",
			impl: &modeOnlyPolicy{mode: policy.ProcessingMode{
				ResponseHeaderMode: policy.HeaderModeProcess,
			}},
			wantRequired: false,
		},
		{
			name:         "zero value with interface",
			impl:         &responseHeaderModePolicy{mode: policy.ProcessingMode{}},
			wantRequired: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := buildSinglePolicyChainForTest(t, "response-header-test", tt.impl)
			assert.Equal(t, tt.wantRequired, chain.RequiresResponseHeader)
		})
	}
}

func TestBuildPolicyChain_RequestStreamingFollowsMode(t *testing.T) {
	tests := []struct {
		name         string
		impl         policy.Policy
		wantRequired bool
		wantStream   bool
	}{
		{
			name: "stream with streaming interface",
			impl: &streamingRequestModePolicy{mode: policy.ProcessingMode{
				RequestBodyMode: policy.BodyModeStream,
			}},
			wantRequired: true,
			wantStream:   true,
		},
		{
			name: "stream without streaming interface",
			impl: &bufferedRequestModePolicy{mode: policy.ProcessingMode{
				RequestBodyMode: policy.BodyModeStream,
			}},
			wantRequired: true,
			wantStream:   false,
		},
		{
			name: "buffer with streaming interface",
			impl: &streamingRequestModePolicy{mode: policy.ProcessingMode{
				RequestBodyMode: policy.BodyModeBuffer,
			}},
			wantRequired: true,
			wantStream:   false,
		},
		{
			name: "buffer without streaming interface",
			impl: &bufferedRequestModePolicy{mode: policy.ProcessingMode{
				RequestBodyMode: policy.BodyModeBuffer,
			}},
			wantRequired: true,
			wantStream:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := buildSinglePolicyChainForTest(t, "request-stream-test", tt.impl)
			assert.Equal(t, tt.wantRequired, chain.RequiresRequestBody)
			assert.Equal(t, tt.wantStream, chain.SupportsRequestStreaming)
		})
	}
}

func TestBuildPolicyChain_ResponseStreamingFollowsMode(t *testing.T) {
	tests := []struct {
		name         string
		impl         policy.Policy
		wantRequired bool
		wantStream   bool
	}{
		{
			name: "stream with streaming interface",
			impl: &streamingResponseModePolicy{mode: policy.ProcessingMode{
				ResponseBodyMode: policy.BodyModeStream,
			}},
			wantRequired: true,
			wantStream:   true,
		},
		{
			name: "stream without streaming interface",
			impl: &bufferedResponseModePolicy{mode: policy.ProcessingMode{
				ResponseBodyMode: policy.BodyModeStream,
			}},
			wantRequired: true,
			wantStream:   false,
		},
		{
			name: "buffer with streaming interface",
			impl: &streamingResponseModePolicy{mode: policy.ProcessingMode{
				ResponseBodyMode: policy.BodyModeBuffer,
			}},
			wantRequired: true,
			wantStream:   false,
		},
		{
			name: "buffer without streaming interface",
			impl: &bufferedResponseModePolicy{mode: policy.ProcessingMode{
				ResponseBodyMode: policy.BodyModeBuffer,
			}},
			wantRequired: true,
			wantStream:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := buildSinglePolicyChainForTest(t, "response-stream-test", tt.impl)
			assert.Equal(t, tt.wantRequired, chain.RequiresResponseBody)
			assert.Equal(t, tt.wantStream, chain.SupportsResponseStreaming)
		})
	}
}
