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

package kernel

import (
	"context"
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocconfigv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/testutils"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	"go.opentelemetry.io/otel/trace/noop"
)

// =============================================================================
// newPolicyExecutionContext Tests
// =============================================================================

func TestNewPolicyExecutionContext(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{
		Policies:    []policy.Policy{},
		PolicySpecs: []policy.PolicySpec{},
	}

	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	require.NotNil(t, execCtx)
	assert.Equal(t, "test-route", execCtx.routeKey)
	assert.Equal(t, chain, execCtx.policyChain)
	assert.NotNil(t, execCtx.analyticsMetadata)
	assert.Empty(t, execCtx.analyticsMetadata)
}

// =============================================================================
// handlePolicyError Tests
// =============================================================================

func TestHandlePolicyError(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.requestID = "req-123"

	resp := execCtx.handlePolicyError(context.Background(), assert.AnError, "request_headers")

	require.NotNil(t, resp)

	// Check immediate response
	immResp := resp.GetImmediateResponse()
	require.NotNil(t, immResp)
	assert.Equal(t, uint32(500), uint32(immResp.Status.Code))
	assert.NotNil(t, immResp.Headers)
	assert.NotNil(t, immResp.Body)

	// Body should contain error ID
	bodyStr := string(immResp.Body)
	assert.Contains(t, bodyStr, "Internal Server Error")
	assert.Contains(t, bodyStr, "error_id")
}

// =============================================================================
// getModeOverride Tests
// =============================================================================

func TestGetModeOverride_NoBodyRequired(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{
		RequiresRequestBody:  false,
		RequiresResponseBody: false,
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	mode := execCtx.getModeOverride()

	require.NotNil(t, mode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_NONE, mode.RequestBodyMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_NONE, mode.ResponseBodyMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_SEND, mode.ResponseHeaderMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_SKIP, mode.RequestTrailerMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_SKIP, mode.ResponseTrailerMode)
}

func TestGetModeOverride_RequestBodyRequired(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{
		RequiresRequestBody:  true,
		RequiresResponseBody: false,
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	mode := execCtx.getModeOverride()

	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED, mode.RequestBodyMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_NONE, mode.ResponseBodyMode)
}

func TestGetModeOverride_ResponseBodyRequired(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{
		RequiresRequestBody:  false,
		RequiresResponseBody: true,
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	mode := execCtx.getModeOverride()

	assert.Equal(t, extprocconfigv3.ProcessingMode_NONE, mode.RequestBodyMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED, mode.ResponseBodyMode)
}

func TestGetModeOverride_BothBodiesRequired(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{
		RequiresRequestBody:  true,
		RequiresResponseBody: true,
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	mode := execCtx.getModeOverride()

	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED, mode.RequestBodyMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED, mode.ResponseBodyMode)
}

func TestGetModeOverride_ResponseHeaderProcessing(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	mockPol := &testutils.ConfigurableMockPolicy{
		MockMode: policy.ProcessingMode{
			ResponseHeaderMode: policy.HeaderModeProcess,
		},
	}

	chain := &registry.PolicyChain{
		Policies: []policy.Policy{mockPol},
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	mode := execCtx.getModeOverride()

	// Response header mode should still be SEND (optimization not implemented yet)
	assert.Equal(t, extprocconfigv3.ProcessingMode_SEND, mode.ResponseHeaderMode)
}

// =============================================================================
// buildRequestContext Tests
// =============================================================================

func TestBuildRequestContext_BasicHeaders(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	headers := &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":path", RawValue: []byte("/api/pets")},
				{Key: ":method", RawValue: []byte("POST")},
				{Key: ":authority", RawValue: []byte("api.example.com")},
				{Key: ":scheme", RawValue: []byte("https")},
				{Key: "content-type", RawValue: []byte("application/json")},
			},
		},
		EndOfStream: false,
	}

	routeMetadata := RouteMetadata{
		RouteName:  "test-route",
		APIName:    "PetStore",
		APIVersion: "v1.0",
		Vhost:      "api.example.com",
	}

	execCtx.buildRequestContext(headers, routeMetadata)

	require.NotNil(t, execCtx.requestContext)
	assert.Equal(t, "/api/pets", execCtx.requestContext.Path)
	assert.Equal(t, "POST", execCtx.requestContext.Method)
	assert.Equal(t, "api.example.com", execCtx.requestContext.Authority)
	assert.Equal(t, "https", execCtx.requestContext.Scheme)
	assert.Equal(t, "api.example.com", execCtx.requestContext.Vhost)

	// Check SharedContext
	require.NotNil(t, execCtx.requestContext.SharedContext)
	assert.Equal(t, "PetStore", execCtx.requestContext.SharedContext.APIName)
	assert.Equal(t, "v1.0", execCtx.requestContext.SharedContext.APIVersion)

	// Request ID should be generated
	assert.NotEmpty(t, execCtx.requestID)
	assert.Len(t, execCtx.requestID, 36)
}

func TestBuildRequestContext_WithRequestID(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	headers := &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":path", RawValue: []byte("/api/pets")},
				{Key: "x-request-id", RawValue: []byte("custom-request-id")},
			},
		},
	}

	execCtx.buildRequestContext(headers, RouteMetadata{})

	// Should use existing request ID
	assert.Equal(t, "custom-request-id", execCtx.requestID)
	assert.Equal(t, "custom-request-id", execCtx.requestContext.SharedContext.RequestID)
}

func TestBuildRequestContext_EndOfStream(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	headers := &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":path", RawValue: []byte("/api/pets")},
			},
		},
		EndOfStream: true,
	}

	execCtx.buildRequestContext(headers, RouteMetadata{})

	// Body should be marked as end of stream with no content
	require.NotNil(t, execCtx.requestContext.Body)
	assert.True(t, execCtx.requestContext.Body.EndOfStream)
	assert.False(t, execCtx.requestContext.Body.Present)
	assert.Nil(t, execCtx.requestContext.Body.Content)
}

func TestBuildRequestContext_WithTemplateAndProvider(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	headers := &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":path", RawValue: []byte("/api/chat")},
			},
		},
	}

	routeMetadata := RouteMetadata{
		RouteName:      "test-route",
		TemplateHandle: "gpt-4",
		ProviderName:   "openai",
		ProjectID:      "proj-123",
	}

	execCtx.buildRequestContext(headers, routeMetadata)

	// Check SharedContext metadata
	require.NotNil(t, execCtx.requestContext.SharedContext.Metadata)
	assert.Equal(t, "gpt-4", execCtx.requestContext.SharedContext.Metadata["template_handle"])
	assert.Equal(t, "openai", execCtx.requestContext.SharedContext.Metadata["provider_name"])
	assert.Equal(t, "proj-123", execCtx.requestContext.SharedContext.ProjectID)
}

func TestBuildRequestContext_MultipleHeaderValues(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	headers := &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":path", RawValue: []byte("/api/pets")},
				{Key: "accept", RawValue: []byte("application/json")},
				{Key: "accept", RawValue: []byte("text/plain")},
			},
		},
	}

	execCtx.buildRequestContext(headers, RouteMetadata{})

	// Should have both accept values
	acceptValues := execCtx.requestContext.Headers.GetAll()["accept"]
	assert.Len(t, acceptValues, 2)
	assert.Contains(t, acceptValues, "application/json")
	assert.Contains(t, acceptValues, "text/plain")
}

// =============================================================================
// buildResponseContext Tests
// =============================================================================

func TestBuildResponseContext_BasicHeaders(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	// First build request context
	reqHeaders := &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":path", RawValue: []byte("/api/pets")},
				{Key: ":method", RawValue: []byte("GET")},
			},
		},
	}
	execCtx.buildRequestContext(reqHeaders, RouteMetadata{})

	// Now build response context
	respHeaders := &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":status", RawValue: []byte("200")},
				{Key: "content-type", RawValue: []byte("application/json")},
			},
		},
		EndOfStream: false,
	}

	execCtx.buildResponseContext(respHeaders)

	require.NotNil(t, execCtx.responseContext)
	assert.Equal(t, 200, execCtx.responseContext.ResponseStatus)

	// Should have same SharedContext as request
	assert.Equal(t, execCtx.requestContext.SharedContext, execCtx.responseContext.SharedContext)

	// Should have request data
	assert.Equal(t, "/api/pets", execCtx.responseContext.RequestPath)
	assert.Equal(t, "GET", execCtx.responseContext.RequestMethod)
}

func TestBuildResponseContext_EndOfStream(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	// Build request context first
	reqHeaders := &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{},
	}
	execCtx.buildRequestContext(reqHeaders, RouteMetadata{})

	// Build response context with end of stream
	respHeaders := &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":status", RawValue: []byte("204")},
			},
		},
		EndOfStream: true,
	}

	execCtx.buildResponseContext(respHeaders)

	require.NotNil(t, execCtx.responseContext.ResponseBody)
	assert.True(t, execCtx.responseContext.ResponseBody.EndOfStream)
	assert.False(t, execCtx.responseContext.ResponseBody.Present)
}

// =============================================================================
// processRequestHeaders Split Execution Tests
// =============================================================================

func TestProcessRequestHeaders_SplitExecution_HeaderOnlyPoliciesRunInHeadersPhase(t *testing.T) {
	// When body buffering is needed, header-only policies (like set-headers for upstream auth)
	// should execute in the headers phase so their header mutations are applied by Envoy.
	kernel := NewKernel()
	tracer := noop.NewTracerProvider().Tracer("test")
	chainExecutor := executor.NewChainExecutor(nil, nil, tracer)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	// Create a header-only policy that sets an auth header
	authPolicy := &testutils.ConfigurableMockPolicy{
		Name:    "set-headers",
		Version: "v0",
		MockMode: policy.ProcessingMode{
			RequestHeaderMode: policy.HeaderModeProcess,
			RequestBodyMode:   policy.BodyModeSkip,
		},
		OnReqFn: func(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
			return policy.UpstreamRequestModifications{
				SetHeaders: map[string]string{"x-api-key": "sk-test-key"},
			}
		},
	}

	// Create a body-requiring policy
	bodyPolicy := &testutils.ConfigurableMockPolicy{
		Name:    "llm-cost",
		Version: "v1",
		MockMode: policy.ProcessingMode{
			RequestHeaderMode: policy.HeaderModeProcess,
			RequestBodyMode:   policy.BodyModeBuffer,
		},
		OnReqFn: func(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
			return policy.UpstreamRequestModifications{}
		},
	}

	chain := &registry.PolicyChain{
		Policies:            []policy.Policy{bodyPolicy, authPolicy},
		PolicySpecs:         []policy.PolicySpec{
			{Name: "llm-cost", Version: "v1", Enabled: true},
			{Name: "set-headers", Version: "v0", Enabled: true},
		},
		RequiresRequestBody: true,
	}

	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	// Build request context with body coming (EndOfStream = false)
	headers := &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":path", RawValue: []byte("/v1/messages")},
				{Key: ":method", RawValue: []byte("POST")},
			},
		},
		EndOfStream: false,
	}
	execCtx.buildRequestContext(headers, RouteMetadata{})

	resp, err := execCtx.processRequestHeaders(context.Background())

	require.NoError(t, err)
	require.NotNil(t, resp)

	// Response should be a HeadersResponse (not empty) with header mutations
	headersResp := resp.GetRequestHeaders()
	require.NotNil(t, headersResp, "expected HeadersResponse, got %T", resp.Response)

	// Should have header mutation with the auth header
	require.NotNil(t, headersResp.Response, "expected CommonResponse with header mutations")
	require.NotNil(t, headersResp.Response.HeaderMutation, "expected header mutation")

	// Find the x-api-key header in mutations
	found := false
	for _, hdr := range headersResp.Response.HeaderMutation.SetHeaders {
		if hdr.Header.Key == "x-api-key" {
			assert.Equal(t, "sk-test-key", string(hdr.Header.RawValue))
			found = true
		}
	}
	assert.True(t, found, "x-api-key header should be in header mutations")

	// ModeOverride should still request body buffering
	require.NotNil(t, resp.ModeOverride)
	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED, resp.ModeOverride.RequestBodyMode)

	// Body policies should be deferred
	assert.Len(t, execCtx.deferredPolicies, 1)
	assert.Len(t, execCtx.deferredSpecs, 1)
	assert.Equal(t, "llm-cost", execCtx.deferredSpecs[0].Name)
}

func TestProcessRequestHeaders_SplitExecution_NoHeaderOnlyPolicies(t *testing.T) {
	// When all policies require body, headers phase should return empty response
	kernel := NewKernel()
	tracer := noop.NewTracerProvider().Tracer("test")
	chainExecutor := executor.NewChainExecutor(nil, nil, tracer)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	bodyPolicy := &testutils.ConfigurableMockPolicy{
		MockMode: policy.ProcessingMode{
			RequestHeaderMode: policy.HeaderModeProcess,
			RequestBodyMode:   policy.BodyModeBuffer,
		},
	}

	chain := &registry.PolicyChain{
		Policies:            []policy.Policy{bodyPolicy},
		PolicySpecs:         []policy.PolicySpec{{Name: "body-pol", Version: "v1", Enabled: true}},
		RequiresRequestBody: true,
	}

	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	headers := &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":path", RawValue: []byte("/test")},
			},
		},
		EndOfStream: false,
	}
	execCtx.buildRequestContext(headers, RouteMetadata{})

	resp, err := execCtx.processRequestHeaders(context.Background())

	require.NoError(t, err)
	require.NotNil(t, resp)

	headersResp := resp.GetRequestHeaders()
	require.NotNil(t, headersResp)
	// Should be empty (no CommonResponse mutations)
	assert.Nil(t, headersResp.Response)
	// ModeOverride should still request buffering
	require.NotNil(t, resp.ModeOverride)
	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED, resp.ModeOverride.RequestBodyMode)
}

func TestProcessRequestBody_UsesDeferredPolicies(t *testing.T) {
	// Body phase should only execute deferred (body-requiring) policies
	kernel := NewKernel()
	tracer := noop.NewTracerProvider().Tracer("test")
	chainExecutor := executor.NewChainExecutor(nil, nil, tracer)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	bodyPolicyExecuted := false
	bodyPolicy := &testutils.ConfigurableMockPolicy{
		MockMode: policy.ProcessingMode{
			RequestHeaderMode: policy.HeaderModeProcess,
			RequestBodyMode:   policy.BodyModeBuffer,
		},
		OnReqFn: func(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
			bodyPolicyExecuted = true
			return policy.UpstreamRequestModifications{}
		},
	}

	authPolicyExecuted := false
	authPolicy := &testutils.ConfigurableMockPolicy{
		MockMode: policy.ProcessingMode{
			RequestHeaderMode: policy.HeaderModeProcess,
			RequestBodyMode:   policy.BodyModeSkip,
		},
		OnReqFn: func(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
			authPolicyExecuted = true
			return policy.UpstreamRequestModifications{
				SetHeaders: map[string]string{"x-api-key": "sk-test-key"},
			}
		},
	}

	chain := &registry.PolicyChain{
		Policies:            []policy.Policy{bodyPolicy, authPolicy},
		PolicySpecs:         []policy.PolicySpec{
			{Name: "llm-cost", Version: "v1", Enabled: true},
			{Name: "set-headers", Version: "v0", Enabled: true},
		},
		RequiresRequestBody: true,
	}

	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	// Build request context
	headers := &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":path", RawValue: []byte("/v1/messages")},
				{Key: ":method", RawValue: []byte("POST")},
			},
		},
		EndOfStream: false,
	}
	execCtx.buildRequestContext(headers, RouteMetadata{})

	// First: process headers phase (splits and executes header-only policies)
	_, err := execCtx.processRequestHeaders(context.Background())
	require.NoError(t, err)

	// Auth policy should have executed in headers phase
	assert.True(t, authPolicyExecuted, "auth policy should execute in headers phase")
	assert.False(t, bodyPolicyExecuted, "body policy should NOT execute in headers phase")

	// Reset flags
	authPolicyExecuted = false

	// Now: process body phase (should only execute deferred body policies)
	body := &extprocv3.HttpBody{
		Body:        []byte(`{"model":"claude-3"}`),
		EndOfStream: true,
	}
	resp, err := execCtx.processRequestBody(context.Background(), body)

	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.True(t, bodyPolicyExecuted, "body policy should execute in body phase")
	assert.False(t, authPolicyExecuted, "auth policy should NOT execute again in body phase")
}

func TestBuildResponseContext_InvalidStatus(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	// Build request context first
	reqHeaders := &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{},
	}
	execCtx.buildRequestContext(reqHeaders, RouteMetadata{})

	// Build response context with invalid status
	respHeaders := &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":status", RawValue: []byte("invalid")},
			},
		},
	}

	// Should not panic, status will be 0
	execCtx.buildResponseContext(respHeaders)

	assert.Equal(t, 0, execCtx.responseContext.ResponseStatus)
}
