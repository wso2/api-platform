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
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocconfigv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/testutils"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// newTestExecutor returns a ChainExecutor with a no-op tracer suitable for tests
// that actually execute policies (processRequestBody / processResponseBody).
func newTestExecutor() *executor.ChainExecutor {
	return executor.NewChainExecutor(nil, nil, noop.NewTracerProvider().Tracer(""))
}

// =============================================================================
// newPolicyExecutionContext Tests
// =============================================================================

func TestNewPolicyExecutionContext(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

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

func TestPolicyExecutionContext_CloseStreamDecompressors(t *testing.T) {
	requestDecomp := newStreamDecompressor("gzip", testMaxDecompressedBytes)
	responseDecomp := newStreamDecompressor("br", testMaxDecompressedBytes)
	execCtx := &PolicyExecutionContext{
		requestStreamDecomp:  requestDecomp,
		responseStreamDecomp: responseDecomp,
	}

	execCtx.closeStreamDecompressors()
	execCtx.closeStreamDecompressors() // cleanup is idempotent

	assert.Nil(t, execCtx.requestStreamDecomp)
	assert.Nil(t, execCtx.responseStreamDecomp)
	select {
	case <-requestDecomp.decoderDone:
	case <-time.After(2 * time.Second):
		t.Fatal("request decoder goroutine was not stopped")
	}
	select {
	case <-responseDecomp.decoderDone:
	case <-time.After(2 * time.Second):
		t.Fatal("response decoder goroutine was not stopped")
	}
}

// =============================================================================
// handlePolicyError Tests
// =============================================================================

func TestHandlePolicyError(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

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
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

	chain := &registry.PolicyChain{
		RequiresRequestBody:  false,
		RequiresResponseBody: false,
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.phase = phaseRequestHeaders

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
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

	chain := &registry.PolicyChain{
		RequiresRequestBody:  true,
		RequiresResponseBody: false,
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.phase = phaseRequestHeaders

	mode := execCtx.getModeOverride()

	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED, mode.RequestBodyMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_NONE, mode.ResponseBodyMode)
}

func TestGetModeOverride_ResponseBodyRequired(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

	chain := &registry.PolicyChain{
		RequiresRequestBody:  false,
		RequiresResponseBody: true,
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.phase = phaseRequestHeaders

	mode := execCtx.getModeOverride()

	assert.Equal(t, extprocconfigv3.ProcessingMode_NONE, mode.RequestBodyMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED, mode.ResponseBodyMode)
}

func TestGetModeOverride_BothBodiesRequired(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

	chain := &registry.PolicyChain{
		RequiresRequestBody:  true,
		RequiresResponseBody: true,
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.phase = phaseRequestHeaders

	mode := execCtx.getModeOverride()

	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED, mode.RequestBodyMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED, mode.ResponseBodyMode)
}

func TestGetModeOverride_ResponseHeaderProcessing(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

	mockPol := &testutils.ConfigurableMockPolicy{
		MockMode: policy.ProcessingMode{
			ResponseHeaderMode: policy.HeaderModeProcess,
		},
	}

	chain := &registry.PolicyChain{
		Policies: []policy.Policy{mockPol},
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.phase = phaseRequestHeaders

	mode := execCtx.getModeOverride()

	// Response header mode should still be SEND (optimization not implemented yet)
	assert.Equal(t, extprocconfigv3.ProcessingMode_SEND, mode.ResponseHeaderMode)
}

// =============================================================================
// responseStreamingEnabled / getModeOverride streaming-decision Tests
//
// Regression coverage: getModeOverride must derive ResponseBodyMode purely from ec.isStreamingResponse
// (set once by responseStreamingEnabled in processResponseHeaders), never re-derive
// its own decision — otherwise Envoy's negotiated body mode can disagree with which
// body-phase handler the kernel actually runs, which Envoy rejects as a
// content-length/mutated-body mismatch.
// =============================================================================

func TestGetModeOverride_MCPStreamingUpstreamStaysBuffered(t *testing.T) {
	kernel := NewKernel()
	server := NewExternalProcessorServer(kernel, executor.NewChainExecutor(nil, nil, nil), config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

	chain := &registry.PolicyChain{
		RequiresResponseBody:      true,
		SupportsResponseStreaming: true,
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	execCtx.buildRequestContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}}, RouteMetadata{APIKind: string(policy.APIKindMCP)})
	execCtx.buildResponseContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":status", RawValue: []byte("200")},
				{Key: "content-type", RawValue: []byte("text/event-stream")},
			},
		},
		EndOfStream: false,
	})

	execCtx.isStreamingResponse = execCtx.responseStreamingEnabled(false)
	require.False(t, execCtx.isStreamingResponse, "MCP responses must never be upgraded to streaming")

	execCtx.phase = phaseResponseHeaders
	mode := execCtx.getModeOverride()
	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED, mode.ResponseBodyMode)
}

func TestGetModeOverride_NonMCPStreamingUpstreamUpgrades(t *testing.T) {
	kernel := NewKernel()
	server := NewExternalProcessorServer(kernel, executor.NewChainExecutor(nil, nil, nil), config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

	chain := &registry.PolicyChain{
		RequiresResponseBody:      true,
		SupportsResponseStreaming: true,
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	execCtx.buildRequestContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}}, RouteMetadata{APIKind: string(policy.APIKindRestApi)})
	execCtx.buildResponseContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":status", RawValue: []byte("200")},
				{Key: "content-type", RawValue: []byte("text/event-stream")},
			},
		},
		EndOfStream: false,
	})

	execCtx.isStreamingResponse = execCtx.responseStreamingEnabled(false)
	require.True(t, execCtx.isStreamingResponse, "non-MCP streaming upstream responses should still upgrade")

	execCtx.phase = phaseResponseHeaders
	mode := execCtx.getModeOverride()
	assert.Equal(t, extprocconfigv3.ProcessingMode_FULL_DUPLEX_STREAMED, mode.ResponseBodyMode)
}

// =============================================================================
// buildRequestContext Tests
// =============================================================================

func TestBuildRequestContext_BasicHeaders(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

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

	execCtx.buildRequestContexts(headers, routeMetadata)

	require.NotNil(t, execCtx.requestBodyCtx)
	assert.Equal(t, "/api/pets", execCtx.requestBodyCtx.Path)
	assert.Equal(t, "POST", execCtx.requestBodyCtx.Method)
	assert.Equal(t, "api.example.com", execCtx.requestBodyCtx.Authority)
	assert.Equal(t, "https", execCtx.requestBodyCtx.Scheme)
	assert.Equal(t, "api.example.com", execCtx.requestBodyCtx.Vhost)

	// Check SharedContext
	require.NotNil(t, execCtx.sharedCtx)
	assert.Equal(t, "PetStore", execCtx.sharedCtx.APIName)
	assert.Equal(t, "v1.0", execCtx.sharedCtx.APIVersion)

	// Request ID should be generated
	assert.NotEmpty(t, execCtx.requestID)
	assert.Len(t, execCtx.requestID, 36)
}

func TestBuildRequestContext_WithRequestID(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

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

	execCtx.buildRequestContexts(headers, RouteMetadata{})

	// Should use existing request ID
	assert.Equal(t, "custom-request-id", execCtx.requestID)
	assert.Equal(t, "custom-request-id", execCtx.sharedCtx.RequestID)
}

func TestBuildRequestContext_EndOfStream(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

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

	execCtx.buildRequestContexts(headers, RouteMetadata{})

	// Body should be marked as end of stream with no content
	require.NotNil(t, execCtx.requestBodyCtx.Body)
	assert.True(t, execCtx.requestBodyCtx.Body.EndOfStream)
	assert.False(t, execCtx.requestBodyCtx.Body.Present)
	assert.Nil(t, execCtx.requestBodyCtx.Body.Content)
}

func TestBuildRequestContext_WithTemplateAndProvider(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

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

	execCtx.buildRequestContexts(headers, routeMetadata)

	// Check SharedContext metadata
	require.NotNil(t, execCtx.sharedCtx.Metadata)
	assert.Equal(t, "gpt-4", execCtx.sharedCtx.Metadata["template_handle"])
	assert.Equal(t, "openai", execCtx.sharedCtx.Metadata["provider_name"])
	assert.Equal(t, "proj-123", execCtx.sharedCtx.ProjectID)
}

func TestBuildRequestContext_MultipleHeaderValues(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

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

	execCtx.buildRequestContexts(headers, RouteMetadata{})

	// Should have both accept values
	acceptValues := execCtx.requestBodyCtx.Headers.GetAll()["accept"]
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
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

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
	execCtx.buildRequestContexts(reqHeaders, RouteMetadata{})

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

	execCtx.buildResponseContexts(respHeaders)

	require.NotNil(t, execCtx.responseBodyCtx)
	assert.Equal(t, 200, execCtx.responseBodyCtx.ResponseStatus)

	// Should have same SharedContext as request
	assert.Equal(t, execCtx.sharedCtx, execCtx.responseBodyCtx.SharedContext)

	// Should have request data
	assert.Equal(t, "/api/pets", execCtx.responseBodyCtx.RequestPath)
	assert.Equal(t, "GET", execCtx.responseBodyCtx.RequestMethod)
}

func TestBuildResponseContext_EndOfStream(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	// Build request context first
	reqHeaders := &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{},
	}
	execCtx.buildRequestContexts(reqHeaders, RouteMetadata{})

	// Build response context with end of stream
	respHeaders := &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":status", RawValue: []byte("204")},
			},
		},
		EndOfStream: true,
	}

	execCtx.buildResponseContexts(respHeaders)

	require.NotNil(t, execCtx.responseBodyCtx.ResponseBody)
	assert.True(t, execCtx.responseBodyCtx.ResponseBody.EndOfStream)
	assert.False(t, execCtx.responseBodyCtx.ResponseBody.Present)
}

func TestBuildResponseContext_InvalidStatus(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	// Build request context first
	reqHeaders := &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{},
	}
	execCtx.buildRequestContexts(reqHeaders, RouteMetadata{})

	// Build response context with invalid status
	respHeaders := &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":status", RawValue: []byte("invalid")},
			},
		},
	}

	// Should not panic, status will be 0
	execCtx.buildResponseContexts(respHeaders)

	assert.Equal(t, 0, execCtx.responseBodyCtx.ResponseStatus)
}

// =============================================================================
// Content-Encoding Detection Tests
// =============================================================================

func TestBuildRequestContext_DetectsContentEncoding(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	headers := &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":path", RawValue: []byte("/api/chat")},
				{Key: "content-encoding", RawValue: []byte("gzip")},
			},
		},
	}

	execCtx.buildRequestContexts(headers, RouteMetadata{})

	assert.Equal(t, "gzip", execCtx.requestContentEncoding)
}

func TestBuildRequestContext_DetectsBrotliEncoding(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	headers := &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":path", RawValue: []byte("/api/chat")},
				{Key: "content-encoding", RawValue: []byte("br")},
			},
		},
	}

	execCtx.buildRequestContexts(headers, RouteMetadata{})

	assert.Equal(t, "br", execCtx.requestContentEncoding)
}

func TestBuildRequestContext_NoContentEncoding(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	headers := &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":path", RawValue: []byte("/api/chat")},
				{Key: "content-type", RawValue: []byte("application/json")},
			},
		},
	}

	execCtx.buildRequestContexts(headers, RouteMetadata{})

	assert.Empty(t, execCtx.requestContentEncoding)
}

func TestBuildResponseContext_DetectsContentEncoding(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	execCtx.buildRequestContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}}, RouteMetadata{})

	respHeaders := &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":status", RawValue: []byte("200")},
				{Key: "content-encoding", RawValue: []byte("gzip")},
			},
		},
	}

	execCtx.buildResponseContexts(respHeaders)

	assert.Equal(t, "gzip", execCtx.responseContentEncoding)
}

func TestBuildResponseContext_DetectsBrotliEncoding(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	execCtx.buildRequestContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}}, RouteMetadata{})

	respHeaders := &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":status", RawValue: []byte("200")},
				{Key: "content-encoding", RawValue: []byte("br")},
			},
		},
	}

	execCtx.buildResponseContexts(respHeaders)

	assert.Equal(t, "br", execCtx.responseContentEncoding)
}

func TestBuildResponseContext_NoContentEncoding(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	execCtx.buildRequestContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}}, RouteMetadata{})

	respHeaders := &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":status", RawValue: []byte("200")},
				{Key: "content-type", RawValue: []byte("application/json")},
			},
		},
	}

	execCtx.buildResponseContexts(respHeaders)

	assert.Empty(t, execCtx.responseContentEncoding)
}

// =============================================================================
// Body Decompression in processResponseBody Tests
// =============================================================================

func TestProcessResponseBody_DecompressesGzip(t *testing.T) {
	kernel := NewKernel()
	server := NewExternalProcessorServer(kernel, newTestExecutor(), config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

	// Policy that requires and captures the response body
	var capturedBody []byte
	mockPolicy := &testutils.ConfigurableMockPolicy{
		MockMode: policy.ProcessingMode{
			ResponseBodyMode: policy.BodyModeBuffer,
		},
		OnRespFn: func(ctx *policy.ResponseContext, _ map[string]interface{}) policy.ResponseAction {
			if ctx.ResponseBody != nil {
				capturedBody = ctx.ResponseBody.Content
			}
			return policy.DownstreamResponseModifications{}
		},
	}

	chain := &registry.PolicyChain{
		RequiresResponseBody: true,
		Policies:             []policy.Policy{mockPolicy},
		PolicySpecs:          []policy.PolicySpec{{Enabled: true}},
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	// Build request context
	execCtx.buildRequestContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}}, RouteMetadata{})

	// Build response context with gzip content-encoding
	execCtx.buildResponseContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":status", RawValue: []byte("200")},
				{Key: "content-encoding", RawValue: []byte("gzip")},
			},
		},
	})

	// Compress a JSON response body
	originalJSON := []byte(`{"model":"gpt-4","usage":{"prompt_tokens":10,"completion_tokens":20}}`)
	compressed := gzipCompress(originalJSON)

	_, err := execCtx.processResponseBody(context.Background(), &extprocv3.HttpBody{
		Body:        compressed,
		EndOfStream: true,
	})

	require.NoError(t, err)
	assert.Equal(t, originalJSON, capturedBody)
}

func TestProcessResponseBody_DecompressesBrotli(t *testing.T) {
	kernel := NewKernel()
	server := NewExternalProcessorServer(kernel, newTestExecutor(), config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

	var capturedBody []byte
	mockPolicy := &testutils.ConfigurableMockPolicy{
		MockMode: policy.ProcessingMode{
			ResponseBodyMode: policy.BodyModeBuffer,
		},
		OnRespFn: func(ctx *policy.ResponseContext, _ map[string]interface{}) policy.ResponseAction {
			if ctx.ResponseBody != nil {
				capturedBody = ctx.ResponseBody.Content
			}
			return policy.DownstreamResponseModifications{}
		},
	}

	chain := &registry.PolicyChain{
		RequiresResponseBody: true,
		Policies:             []policy.Policy{mockPolicy},
		PolicySpecs:          []policy.PolicySpec{{Enabled: true}},
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	execCtx.buildRequestContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}}, RouteMetadata{})
	execCtx.buildResponseContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":status", RawValue: []byte("200")},
				{Key: "content-encoding", RawValue: []byte("br")},
			},
		},
	})

	originalJSON := []byte(`{"model":"claude-3","usage":{"input_tokens":5,"output_tokens":15}}`)
	compressed := brotliCompress(originalJSON)

	_, err := execCtx.processResponseBody(context.Background(), &extprocv3.HttpBody{
		Body:        compressed,
		EndOfStream: true,
	})

	require.NoError(t, err)
	assert.Equal(t, originalJSON, capturedBody)
}

// =============================================================================
// Unsupported / undecodable Content-Encoding — fail-closed behaviour
// =============================================================================

// newEncodingTestContext builds an execution context whose chain requires both
// bodies, with a policy that records whether it ever ran.
func newEncodingTestContext(t *testing.T, streaming bool) (*PolicyExecutionContext, *bool) {
	t.Helper()

	policyRan := new(bool)
	bodyMode := policy.BodyModeBuffer
	if streaming {
		bodyMode = policy.BodyModeStream
	}
	mockPolicy := &testutils.ConfigurableMockPolicy{
		MockMode: policy.ProcessingMode{
			RequestBodyMode:  bodyMode,
			ResponseBodyMode: bodyMode,
		},
		OnReqFn: func(_ *policy.RequestContext, _ map[string]interface{}) policy.RequestAction {
			*policyRan = true
			return policy.UpstreamRequestModifications{}
		},
		OnRespFn: func(_ *policy.ResponseContext, _ map[string]interface{}) policy.ResponseAction {
			*policyRan = true
			return policy.DownstreamResponseModifications{}
		},
	}

	kernel := NewKernel()
	server := NewExternalProcessorServer(kernel, newTestExecutor(), config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)
	chain := &registry.PolicyChain{
		RequiresRequestBody:       true,
		RequiresResponseBody:      true,
		SupportsRequestStreaming:  streaming,
		SupportsResponseStreaming: streaming,
		Policies:                  []policy.Policy{mockPolicy},
		PolicySpecs:               []policy.PolicySpec{{Enabled: true}},
	}
	return newPolicyExecutionContext(server, "test-route", chain), policyRan
}

func postRequestHeaders(encoding string) *extprocv3.HttpHeaders {
	headers := []*corev3.HeaderValue{
		{Key: ":path", RawValue: []byte("/api/chat")},
		{Key: ":method", RawValue: []byte("POST")},
		{Key: "content-type", RawValue: []byte("application/json")},
	}
	if encoding != "" {
		headers = append(headers, &corev3.HeaderValue{Key: "content-encoding", RawValue: []byte(encoding)})
	}
	return &extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{Headers: headers}}
}

func okResponseHeaders(encoding string) *extprocv3.HttpHeaders {
	headers := []*corev3.HeaderValue{
		{Key: ":status", RawValue: []byte("200")},
		{Key: "content-type", RawValue: []byte("application/json")},
	}
	if encoding != "" {
		headers = append(headers, &corev3.HeaderValue{Key: "content-encoding", RawValue: []byte(encoding)})
	}
	return &extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{Headers: headers}}
}

// An encoding the kernel cannot round-trip must be flagged on BOTH sides. The
// request side is the one that matters most: the value is chosen by the caller,
// so treating it as "no encoding" would let anyone disable every request-body
// policy on the route by setting a header.
func TestBuildContexts_FlagsUnsupportedEncoding(t *testing.T) {
	for _, encoding := range []string{"compress", "snappy", "gzip, br"} {
		t.Run(encoding, func(t *testing.T) {
			execCtx, _ := newEncodingTestContext(t, false)

			execCtx.buildRequestContexts(postRequestHeaders(encoding), RouteMetadata{})
			execCtx.buildResponseContexts(okResponseHeaders(encoding))

			assert.Empty(t, execCtx.requestContentEncoding)
			assert.True(t, execCtx.requestEncodingUnsupported)
			assert.Empty(t, execCtx.responseContentEncoding)
			assert.True(t, execCtx.responseEncodingUnsupported)
		})
	}
}

// Content codings are case-insensitive tokens (RFC 9110 §8.4.1). Before this was
// normalised on the request side, "GZIP" was stored verbatim, missed every
// lowercase codec switch, and reached policies as raw compressed bytes.
func TestBuildContexts_NormalizesEncodingCase(t *testing.T) {
	for _, encoding := range []string{"GZIP", "Gzip", " gzip "} {
		t.Run(encoding, func(t *testing.T) {
			execCtx, _ := newEncodingTestContext(t, false)

			execCtx.buildRequestContexts(postRequestHeaders(encoding), RouteMetadata{})
			execCtx.buildResponseContexts(okResponseHeaders(encoding))

			assert.Equal(t, "gzip", execCtx.requestContentEncoding)
			assert.False(t, execCtx.requestEncodingUnsupported)
			assert.Equal(t, "gzip", execCtx.responseContentEncoding)
			assert.False(t, execCtx.responseEncodingUnsupported)
		})
	}
}

// identity/absent means the body is not encoded — no rejection, policies run.
func TestBuildContexts_IdentityEncodingNotFlagged(t *testing.T) {
	for _, encoding := range []string{"identity", ""} {
		t.Run("encoding="+encoding, func(t *testing.T) {
			execCtx, _ := newEncodingTestContext(t, false)

			execCtx.buildRequestContexts(postRequestHeaders(encoding), RouteMetadata{})
			execCtx.buildResponseContexts(okResponseHeaders(encoding))

			assert.Empty(t, execCtx.requestContentEncoding)
			assert.False(t, execCtx.requestEncodingUnsupported)
			assert.Empty(t, execCtx.responseContentEncoding)
			assert.False(t, execCtx.responseEncodingUnsupported)
		})
	}
}

// The bypass this guards against: an undecodable request body must be rejected
// at the header phase — before any policy runs and before a byte reaches the
// upstream — rather than forwarded with body policies silently skipped.
func TestProcessRequestHeaders_UnsupportedEncodingRejected(t *testing.T) {
	execCtx, policyRan := newEncodingTestContext(t, false)
	execCtx.buildRequestContexts(postRequestHeaders("snappy"), RouteMetadata{})

	resp, err := execCtx.processRequestHeaders(context.Background())
	require.NoError(t, err)

	immediate := resp.GetImmediateResponse()
	require.NotNil(t, immediate, "an undecodable request must be rejected, not forwarded")
	assert.Equal(t, typev3.StatusCode_UnsupportedMediaType, immediate.GetStatus().GetCode())
	assert.False(t, *policyRan)
	// The client learns nothing about the encoding or the policy chain.
	assert.NotContains(t, string(immediate.GetBody()), "snappy")
}

// The upstream, not the client, chose this encoding — so it is a 502, and it is
// caught at the response-header phase while a status can still be chosen.
func TestProcessResponseHeaders_UnsupportedEncodingRejected(t *testing.T) {
	execCtx, policyRan := newEncodingTestContext(t, false)
	execCtx.buildRequestContexts(postRequestHeaders(""), RouteMetadata{})

	resp, err := execCtx.processResponseHeaders(context.Background(), okResponseHeaders("zstd-unknown-variant"))
	require.NoError(t, err)

	immediate := resp.GetImmediateResponse()
	require.NotNil(t, immediate, "an undecodable response must be rejected, not forwarded")
	assert.Equal(t, typev3.StatusCode_BadGateway, immediate.GetStatus().GetCode())
	assert.False(t, *policyRan)
	assert.NotContains(t, string(immediate.GetBody()), "zstd-unknown-variant")
}

// With no body policy attached there is nothing to bypass, so an encoding the
// kernel cannot read is none of its business and must pass through untouched.
// Rejecting here would break routes that never inspect bodies at all.
func TestProcessHeaders_UnsupportedEncodingAllowedWithoutBodyPolicies(t *testing.T) {
	kernel := NewKernel()
	server := NewExternalProcessorServer(kernel, newTestExecutor(), config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)
	execCtx := newPolicyExecutionContext(server, "test-route", &registry.PolicyChain{})

	execCtx.buildRequestContexts(postRequestHeaders("snappy"), RouteMetadata{})
	reqResp, err := execCtx.processRequestHeaders(context.Background())
	require.NoError(t, err)
	assert.Nil(t, reqResp.GetImmediateResponse())

	respResp, err := execCtx.processResponseHeaders(context.Background(), okResponseHeaders("snappy"))
	require.NoError(t, err)
	assert.Nil(t, respResp.GetImmediateResponse())
}

// The cheapest bypass of all: declare a supported encoding, send bytes that are
// not in it. Policies would receive undecodable bytes, match nothing, and the
// body would be forwarded anyway.
func TestProcessRequestBody_UndecodableBodyRejected(t *testing.T) {
	execCtx, policyRan := newEncodingTestContext(t, false)
	execCtx.buildRequestContexts(postRequestHeaders("gzip"), RouteMetadata{})

	resp, err := execCtx.processRequestBody(context.Background(), &extprocv3.HttpBody{
		Body:        []byte(`{"prompt":"not actually gzipped"}`),
		EndOfStream: true,
	})
	require.NoError(t, err)

	immediate := resp.GetImmediateResponse()
	require.NotNil(t, immediate, "a body that is not in its declared encoding must be rejected")
	assert.Equal(t, typev3.StatusCode_BadRequest, immediate.GetStatus().GetCode())
	assert.False(t, *policyRan)
}

func TestProcessResponseBody_UndecodableBodyRejected(t *testing.T) {
	execCtx, policyRan := newEncodingTestContext(t, false)
	execCtx.buildRequestContexts(postRequestHeaders(""), RouteMetadata{})
	execCtx.buildResponseContexts(okResponseHeaders("br"))

	// Brotli accepts many byte sequences, so use one that reliably fails.
	resp, err := execCtx.processResponseBody(context.Background(), &extprocv3.HttpBody{
		Body:        bytes.Repeat([]byte{0xff}, 64),
		EndOfStream: true,
	})
	require.NoError(t, err)

	immediate := resp.GetImmediateResponse()
	require.NotNil(t, immediate, "a body that is not in its declared encoding must be rejected")
	assert.Equal(t, typev3.StatusCode_BadGateway, immediate.GetStatus().GetCode())
	assert.False(t, *policyRan)
}

// Every supported encoding must reach policies as plaintext on the buffered
// request path — the whole point of supporting it rather than rejecting it.
func TestProcessRequestBody_DecompressesEverySupportedEncoding(t *testing.T) {
	plaintext := []byte(`{"prompt":"contact me at user@example.com"}`)

	for _, encoding := range []string{"gzip", "br", "zstd", "deflate"} {
		t.Run(encoding, func(t *testing.T) {
			var capturedBody []byte
			mockPolicy := &testutils.ConfigurableMockPolicy{
				MockMode: policy.ProcessingMode{RequestBodyMode: policy.BodyModeBuffer},
				OnReqFn: func(ctx *policy.RequestContext, _ map[string]interface{}) policy.RequestAction {
					if ctx.Body != nil {
						capturedBody = ctx.Body.Content
					}
					return policy.UpstreamRequestModifications{}
				},
			}

			kernel := NewKernel()
			server := NewExternalProcessorServer(kernel, newTestExecutor(), config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)
			execCtx := newPolicyExecutionContext(server, "test-route", &registry.PolicyChain{
				RequiresRequestBody: true,
				Policies:            []policy.Policy{mockPolicy},
				PolicySpecs:         []policy.PolicySpec{{Enabled: true}},
			})
			execCtx.buildRequestContexts(postRequestHeaders(encoding), RouteMetadata{})

			compressed, err := recompressBody(plaintext, encoding)
			require.NoError(t, err)

			_, err = execCtx.processRequestBody(context.Background(), &extprocv3.HttpBody{
				Body:        compressed,
				EndOfStream: true,
			})
			require.NoError(t, err)
			assert.Equal(t, plaintext, capturedBody)
		})
	}
}

// A "deflate" request carrying RAW deflate (no zlib wrapper) must be detected
// and re-encoded in the same variant. Emitting the other form would hand the
// upstream a body its decoder rejects.
func TestProcessRequestBody_PreservesRawDeflateVariant(t *testing.T) {
	plaintext := []byte(`{"prompt":"raw deflate body"}`)

	kernel := NewKernel()
	server := NewExternalProcessorServer(kernel, newTestExecutor(), config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)
	execCtx := newPolicyExecutionContext(server, "test-route", &registry.PolicyChain{
		RequiresRequestBody: true,
		Policies: []policy.Policy{&testutils.ConfigurableMockPolicy{
			MockMode: policy.ProcessingMode{RequestBodyMode: policy.BodyModeBuffer},
			OnReqFn: func(_ *policy.RequestContext, _ map[string]interface{}) policy.RequestAction {
				return policy.UpstreamRequestModifications{}
			},
		}},
		PolicySpecs: []policy.PolicySpec{{Enabled: true}},
	})
	execCtx.buildRequestContexts(postRequestHeaders("deflate"), RouteMetadata{})

	rawDeflate, err := recompressBody(plaintext, "deflate-raw")
	require.NoError(t, err)

	_, err = execCtx.processRequestBody(context.Background(), &extprocv3.HttpBody{
		Body:        rawDeflate,
		EndOfStream: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "deflate-raw", execCtx.requestContentEncoding,
		"the arriving deflate variant must be pinned so the same one is emitted back")
}

// chunkRecordingRequestPolicy records every request chunk body policies are
// handed, so a test can assert policies saw plaintext rather than compressed
// bytes.
type chunkRecordingRequestPolicy struct {
	seen *strings.Builder
}

func (p *chunkRecordingRequestPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{RequestBodyMode: policy.BodyModeStream}
}

func (p *chunkRecordingRequestPolicy) OnRequestBody(_ context.Context, _ *policy.RequestContext, _ map[string]interface{}) policy.RequestAction {
	return policy.UpstreamRequestModifications{}
}

func (p *chunkRecordingRequestPolicy) NeedsMoreRequestData(_ []byte) bool { return false }

func (p *chunkRecordingRequestPolicy) OnRequestBodyChunk(_ context.Context, _ *policy.RequestStreamContext, chunk *policy.StreamBody, _ map[string]interface{}) policy.StreamingRequestAction {
	p.seen.Write(chunk.Chunk)
	return policy.ForwardRequestChunk{}
}

// End-to-end through the kernel's streaming REQUEST path: chunks arrive
// compressed, policies must see plaintext, and what leaves for the upstream must
// be ONE compressed stream that decodes to the full body. Before the fix the
// request path re-compressed per chunk, so the upstream saw only chunk one.
func TestProcessStreamingRequestBody_RoundTripsAsSingleStream(t *testing.T) {
	chunks := []string{`{"prompt":"part one `, `and part two `, `and part three"}`}
	var wantPlaintext strings.Builder
	for _, c := range chunks {
		wantPlaintext.WriteString(c)
	}

	for _, encoding := range []string{"gzip", "br", "zstd", "deflate"} {
		t.Run(encoding, func(t *testing.T) {
			var seenByPolicies strings.Builder
			mockPolicy := &chunkRecordingRequestPolicy{seen: &seenByPolicies}

			kernel := NewKernel()
			server := NewExternalProcessorServer(kernel, newTestExecutor(), config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)
			execCtx := newPolicyExecutionContext(server, "test-route", &registry.PolicyChain{
				RequiresRequestBody:      true,
				SupportsRequestStreaming: true,
				Policies:                 []policy.Policy{mockPolicy},
				PolicySpecs:              []policy.PolicySpec{{Enabled: true}},
			})
			execCtx.buildRequestContexts(postRequestHeaders(encoding), RouteMetadata{})
			execCtx.isStreamingRequest = true

			// Compress the body as one stream, then split it, mirroring how Envoy
			// delivers an upstream-bound compressed body chunk by chunk.
			whole, err := recompressBody([]byte(wantPlaintext.String()), encoding)
			require.NoError(t, err)
			split := len(whole) / 2

			var wire bytes.Buffer
			for i, in := range [][]byte{whole[:split], whole[split:]} {
				endOfStream := i == 1
				resp, err := execCtx.processRequestBody(context.Background(), &extprocv3.HttpBody{
					Body:        in,
					EndOfStream: endOfStream,
				})
				require.NoError(t, err)
				wire.Write(resp.GetRequestBody().GetResponse().GetBodyMutation().GetStreamedResponse().GetBody())
			}

			assert.Equal(t, wantPlaintext.String(), seenByPolicies.String(),
				"policies must observe plaintext, not compressed bytes")

			// The decisive assertion: one stream, decodable in a single pass.
			got, err := decompressBody(wire.Bytes(), execCtx.requestContentEncoding, 0)
			require.NoError(t, err)
			assert.Equal(t, wantPlaintext.String(), string(got),
				"upstream-bound body must be ONE compressed stream covering every chunk")
		})
	}
}

func TestProcessStreamingResponseBody_DecompressionErrorFailsClosed(t *testing.T) {
	const maxChunkBytes int64 = 64

	kernel := NewKernel()
	server := NewExternalProcessorServer(kernel, newTestExecutor(), config.TracingConfig{}, "", testMaxDecompressedBytes, maxChunkBytes)
	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	execCtx.buildRequestContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}}, RouteMetadata{})
	execCtx.buildResponseContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":status", RawValue: []byte("200")},
				{Key: "content-encoding", RawValue: []byte("gzip")},
			},
		},
	})

	// Feed a real gzip stream without signaling EOS. Splitting the compressed bytes
	// exercises decoder state and boundary accounting across calls. Once enough
	// compressed input has arrived, the ext_proc stream must fail instead of
	// returning a normal empty body and later ending as a successful truncation.
	compressed := gzipCompress(make([]byte, 1024))
	var resp *extprocv3.ProcessingResponse
	var err error
	for i := 0; i < len(compressed) && err == nil; i++ {
		resp, err = execCtx.processStreamingResponseBody(context.Background(), &extprocv3.HttpBody{
			Body:        compressed[i : i+1],
			EndOfStream: false,
		})
	}

	assert.Nil(t, resp, "a decompression failure must not be translated into a normal body response")
	require.ErrorIs(t, err, ErrDecompressedTooLarge)
	assert.False(t, execCtx.streamTerminated, "kernel errors terminate ext_proc instead of using policy EOS suppression")
	assert.Nil(t, execCtx.responseStreamDecomp)
}

func TestProcessResponseBody_DecompressionLimitFailsClosed(t *testing.T) {
	const responseLimit int64 = 64

	kernel := NewKernel()
	server := NewExternalProcessorServer(kernel, newTestExecutor(), config.TracingConfig{}, "", testMaxDecompressedBytes, responseLimit)
	chain := &registry.PolicyChain{
		RequiresResponseBody: true,
		Policies:             []policy.Policy{&testutils.NoopPolicy{}},
		PolicySpecs:          []policy.PolicySpec{{Enabled: true}},
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	execCtx.buildRequestContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}}, RouteMetadata{})
	execCtx.buildResponseContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":status", RawValue: []byte("200")},
				{Key: "content-encoding", RawValue: []byte("gzip")},
			},
		},
	})

	resp, err := execCtx.processResponseBody(context.Background(), &extprocv3.HttpBody{
		Body:        gzipCompress(make([]byte, 1024)),
		EndOfStream: true,
	})

	assert.Nil(t, resp)
	require.ErrorIs(t, err, ErrDecompressedTooLarge)
}

func TestProcessResponseBody_NoEncoding_PassesThrough(t *testing.T) {
	kernel := NewKernel()
	server := NewExternalProcessorServer(kernel, newTestExecutor(), config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

	var capturedBody []byte
	mockPolicy := &testutils.ConfigurableMockPolicy{
		MockMode: policy.ProcessingMode{
			ResponseBodyMode: policy.BodyModeBuffer,
		},
		OnRespFn: func(ctx *policy.ResponseContext, _ map[string]interface{}) policy.ResponseAction {
			if ctx.ResponseBody != nil {
				capturedBody = ctx.ResponseBody.Content
			}
			return policy.DownstreamResponseModifications{}
		},
	}

	chain := &registry.PolicyChain{
		RequiresResponseBody: true,
		Policies:             []policy.Policy{mockPolicy},
		PolicySpecs:          []policy.PolicySpec{{Enabled: true}},
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	execCtx.buildRequestContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}}, RouteMetadata{})
	execCtx.buildResponseContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":status", RawValue: []byte("200")},
			},
		},
	})

	plainJSON := []byte(`{"model":"gpt-4","usage":{"prompt_tokens":10}}`)

	_, err := execCtx.processResponseBody(context.Background(), &extprocv3.HttpBody{
		Body:        plainJSON,
		EndOfStream: true,
	})

	require.NoError(t, err)
	assert.Equal(t, plainJSON, capturedBody)
}

// =============================================================================
// Body Decompression in processRequestBody Tests
// =============================================================================

func TestProcessRequestBody_DecompressesGzip(t *testing.T) {
	kernel := NewKernel()
	server := NewExternalProcessorServer(kernel, newTestExecutor(), config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

	chain := &registry.PolicyChain{
		RequiresRequestBody: true,
		Policies:            []policy.Policy{&testutils.NoopPolicy{}},
		PolicySpecs:         []policy.PolicySpec{{Enabled: true}},
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	// Build request context with gzip content-encoding
	execCtx.buildRequestContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":path", RawValue: []byte("/api/chat")},
				{Key: "content-encoding", RawValue: []byte("gzip")},
			},
		},
	}, RouteMetadata{})

	originalJSON := []byte(`{"messages":[{"role":"user","content":"Hello"}]}`)
	compressed := gzipCompress(originalJSON)

	_, err := execCtx.processRequestBody(context.Background(), &extprocv3.HttpBody{
		Body:        compressed,
		EndOfStream: true,
	})

	require.NoError(t, err)
	// Body set on requestContext should be the decompressed JSON, not the raw compressed bytes
	require.NotNil(t, execCtx.requestBodyCtx.Body)
	assert.Equal(t, originalJSON, execCtx.requestBodyCtx.Body.Content)
}

func TestProcessRequestBody_NoEncoding_PassesThrough(t *testing.T) {
	kernel := NewKernel()
	server := NewExternalProcessorServer(kernel, newTestExecutor(), config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

	chain := &registry.PolicyChain{
		RequiresRequestBody: true,
		Policies:            []policy.Policy{&testutils.NoopPolicy{}},
		PolicySpecs:         []policy.PolicySpec{{Enabled: true}},
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	execCtx.buildRequestContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":path", RawValue: []byte("/api/chat")},
			},
		},
	}, RouteMetadata{})

	plainJSON := []byte(`{"messages":[{"role":"user","content":"Hello"}]}`)

	_, err := execCtx.processRequestBody(context.Background(), &extprocv3.HttpBody{
		Body:        plainJSON,
		EndOfStream: true,
	})

	require.NoError(t, err)
	require.NotNil(t, execCtx.requestBodyCtx.Body)
	assert.Equal(t, plainJSON, execCtx.requestBodyCtx.Body.Content)
}

// =============================================================================
// MCP SSE response body-mutation regression Tests
//
// Reproduces the exact configuration that triggered the HTTP 500
// ("mismatch_between_content_length_and_the_length_of_the_mutated_body"): an MCP
// proxy chain whose only response-body policy is streaming-capable (mirroring the
// always-injected analytics system policy with no user policies attached), fronting
// an upstream that replies with a chunked text/event-stream body. Before the fix,
// isStreamingResponse was set to true independently of the MCP-only BUFFERED
// ModeOverride, so processResponseBody dispatched to the streaming handler and
// emitted a BodyMutation_StreamedResponse while Envoy was in a buffered body
// callback — which Envoy rejects. After the fix, both decisions come from the same
// responseStreamingEnabled() predicate, so a body-mutating policy's output always
// arrives as a BodyMutation_Body with a matching content-length.
// =============================================================================

func TestProcessResponseBody_MCPSSEEmitsBufferedBodyMutation(t *testing.T) {
	kernel := NewKernel()
	server := NewExternalProcessorServer(kernel, newTestExecutor(), config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

	rewrittenBody := []byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"tools\":[]}}\n\n")
	mockPolicy := &testutils.ConfigurableMockPolicy{
		MockMode: policy.ProcessingMode{
			ResponseBodyMode: policy.BodyModeBuffer,
		},
		OnRespFn: func(_ *policy.ResponseContext, _ map[string]interface{}) policy.ResponseAction {
			return policy.DownstreamResponseModifications{Body: rewrittenBody}
		},
	}

	chain := &registry.PolicyChain{
		RequiresResponseBody: true,
		// Mirrors a chain with no user MCP policies attached: the only response-body
		// policy is the always-injected, streaming-capable analytics system policy.
		SupportsResponseStreaming: true,
		Policies:                  []policy.Policy{mockPolicy},
		PolicySpecs:               []policy.PolicySpec{{Enabled: true}},
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	execCtx.buildRequestContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":path", RawValue: []byte("/everything/mcp")},
				{Key: ":method", RawValue: []byte("POST")},
			},
		},
	}, RouteMetadata{APIKind: string(policy.APIKindMCP)})

	headersResp, err := execCtx.processResponseHeaders(context.Background(), &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":status", RawValue: []byte("200")},
				{Key: "content-type", RawValue: []byte("text/event-stream")},
				{Key: "transfer-encoding", RawValue: []byte("chunked")},
			},
		},
		EndOfStream: false,
	})
	require.NoError(t, err)
	require.False(t, execCtx.isStreamingResponse, "MCP responses must stay buffered end-to-end")
	require.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED, headersResp.ModeOverride.ResponseBodyMode)

	upstreamBody := []byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"tools\":[{\"name\":\"add\"}]}}\n\n")
	bodyResp, err := execCtx.processResponseBody(context.Background(), &extprocv3.HttpBody{
		Body:        upstreamBody,
		EndOfStream: true,
	})
	require.NoError(t, err)

	respBody := bodyResp.GetResponseBody()
	require.NotNil(t, respBody)
	mutation := respBody.GetResponse().GetBodyMutation()
	require.NotNil(t, mutation, "a body-mutating policy must produce a body mutation")

	// The critical assertion: never a StreamedResponse mutation while Envoy is running
	// this response in a buffered body callback (see processor_state.cc validateContentLength).
	assert.Nil(t, mutation.GetStreamedResponse())
	assert.Equal(t, rewrittenBody, mutation.GetBody())

	contentLength := ""
	for _, h := range respBody.GetResponse().GetHeaderMutation().GetSetHeaders() {
		if h.GetHeader().GetKey() == "content-length" {
			contentLength = string(h.GetHeader().GetRawValue())
		}
	}
	assert.Equal(t, fmt.Sprintf("%d", len(rewrittenBody)), contentLength)
}

func TestProcessRequestBody_DecompressionLimitReturns413(t *testing.T) {
	const requestLimit int64 = 64

	kernel := NewKernel()
	server := NewExternalProcessorServer(kernel, newTestExecutor(), config.TracingConfig{}, "", requestLimit, testMaxDecompressedBytes)
	chain := &registry.PolicyChain{
		RequiresRequestBody: true,
		Policies:            []policy.Policy{&testutils.NoopPolicy{}},
		PolicySpecs:         []policy.PolicySpec{{Enabled: true}},
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.buildRequestContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":path", RawValue: []byte("/api/chat")},
				{Key: "content-encoding", RawValue: []byte("gzip")},
			},
		},
	}, RouteMetadata{})

	resp, err := execCtx.processRequestBody(context.Background(), &extprocv3.HttpBody{
		Body:        gzipCompress(make([]byte, 1024)),
		EndOfStream: true,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.GetImmediateResponse())
	assert.Equal(t, typev3.StatusCode_PayloadTooLarge, resp.GetImmediateResponse().Status.Code)
}

func TestProcessStreamingRequestBody_DecompressionLimitFailsClosed(t *testing.T) {
	const requestLimit int64 = 64

	kernel := NewKernel()
	server := NewExternalProcessorServer(kernel, newTestExecutor(), config.TracingConfig{}, "", requestLimit, testMaxDecompressedBytes)
	chain := &registry.PolicyChain{
		RequiresRequestBody:      true,
		SupportsRequestStreaming: true,
		Policies:                 []policy.Policy{&testutils.NoopPolicy{}},
		PolicySpecs:              []policy.PolicySpec{{Enabled: true}},
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.buildRequestContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":path", RawValue: []byte("/api/chat")},
				{Key: "content-encoding", RawValue: []byte("gzip")},
				{Key: "transfer-encoding", RawValue: []byte("chunked")},
			},
		},
	}, RouteMetadata{})

	compressed := gzipCompress(make([]byte, 1024))
	var resp *extprocv3.ProcessingResponse
	var err error
	for i := 0; i < len(compressed) && err == nil; i++ {
		resp, err = execCtx.processStreamingRequestBody(context.Background(), &extprocv3.HttpBody{
			Body:        compressed[i : i+1],
			EndOfStream: false,
		})
	}

	assert.Nil(t, resp, "full-duplex limit failures must not promise a late ImmediateResponse")
	require.ErrorIs(t, err, ErrDecompressedTooLarge)
	assert.Nil(t, execCtx.requestStreamDecomp)
}

func TestProcessStreamingRequestBody_MalformedEncodingFailsClosed(t *testing.T) {
	kernel := NewKernel()
	server := NewExternalProcessorServer(kernel, newTestExecutor(), config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)
	chain := &registry.PolicyChain{
		RequiresRequestBody:      true,
		SupportsRequestStreaming: true,
		Policies:                 []policy.Policy{&testutils.NoopPolicy{}},
		PolicySpecs:              []policy.PolicySpec{{Enabled: true}},
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.buildRequestContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":path", RawValue: []byte("/api/chat")},
				{Key: "content-encoding", RawValue: []byte("gzip")},
				{Key: "transfer-encoding", RawValue: []byte("chunked")},
			},
		},
	}, RouteMetadata{})

	compressed := gzipCompress([]byte("valid body with a corrupted trailer"))
	compressed[len(compressed)-1] ^= 0xff
	resp, err := execCtx.processStreamingRequestBody(context.Background(), &extprocv3.HttpBody{
		Body:        compressed,
		EndOfStream: true,
	})

	assert.Nil(t, resp, "malformed compressed fragments must never be forwarded raw")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrDecompressedTooLarge)
	assert.Nil(t, execCtx.requestStreamDecomp)
}

// getRequestHeaders builds GET request headers, optionally Content-Encoded, with
// Envoy's headers-message EndOfStream flag under the test's control — the flag is
// what tells the kernel whether a body follows.
func getRequestHeaders(encoding string, endOfStream bool) *extprocv3.HttpHeaders {
	headers := []*corev3.HeaderValue{
		{Key: ":path", RawValue: []byte("/api/chat")},
		{Key: ":method", RawValue: []byte("GET")},
	}
	if encoding != "" {
		headers = append(headers, &corev3.HeaderValue{Key: "content-encoding", RawValue: []byte(encoding)})
	}
	return &extprocv3.HttpHeaders{
		Headers:     &corev3.HeaderMap{Headers: headers},
		EndOfStream: endOfStream,
	}
}

// A GET may syntactically carry a body, and Envoy reports that as
// EndOfStream=false on the headers message and then delivers a body phase.
// Inferring "bodyless" from the method alone skipped the encoding guard, so raw
// undecodable bytes reached body policies with no rejection anywhere.
func TestProcessRequestHeaders_GetWithBodyRejectsUnsupportedEncoding(t *testing.T) {
	execCtx, policyRan := newEncodingTestContext(t, false)
	execCtx.buildRequestContexts(getRequestHeaders("snappy", false), RouteMetadata{})

	assert.False(t, execCtx.requestHasNoBody(), "EndOfStream=false means a body follows, whatever the method says")

	resp, err := execCtx.processRequestHeaders(context.Background())
	require.NoError(t, err)

	immediate := resp.GetImmediateResponse()
	require.NotNil(t, immediate, "a GET carrying an undecodable body must be rejected like any other body")
	assert.Equal(t, typev3.StatusCode_UnsupportedMediaType, immediate.GetStatus().GetCode())
	assert.False(t, *policyRan)
}

// A bodyless GET (EndOfStream=true) has nothing to decode, so an encoding the
// kernel cannot read is none of its business — body policies run inline with a
// nil body instead of the request being rejected.
func TestProcessRequestHeaders_BodylessGetPassesUnsupportedEncoding(t *testing.T) {
	execCtx, _ := newEncodingTestContext(t, false)
	execCtx.buildRequestContexts(getRequestHeaders("snappy", true), RouteMetadata{})

	assert.True(t, execCtx.requestHasNoBody())

	resp, err := execCtx.processRequestHeaders(context.Background())
	require.NoError(t, err)
	assert.Nil(t, resp.GetImmediateResponse())
}

// Defence in depth, mirroring the response side: if framing promised no body and
// a body arrives anyway under an undecodable encoding, the body phase rejects it
// rather than handing compressed bytes to policies as if they were plaintext.
func TestProcessRequestBody_UnsupportedEncodingRejectedAtBodyPhase(t *testing.T) {
	execCtx, policyRan := newEncodingTestContext(t, false)
	execCtx.buildRequestContexts(getRequestHeaders("snappy", true), RouteMetadata{})

	resp, err := execCtx.processRequestBody(context.Background(), &extprocv3.HttpBody{
		Body:        []byte("opaque snappy bytes"),
		EndOfStream: true,
	})
	require.NoError(t, err)

	immediate := resp.GetImmediateResponse()
	require.NotNil(t, immediate, "a body under an undecodable encoding must never reach policies")
	assert.Equal(t, typev3.StatusCode_UnsupportedMediaType, immediate.GetStatus().GetCode())
	assert.False(t, *policyRan)
	assert.NotContains(t, string(immediate.GetBody()), "snappy")
}

// A header-only response (EndOfStream=true on the response headers) has no body
// to decode, so an encoding the kernel cannot read must pass through — rejecting
// it turned an empty 200 into a 502.
func TestProcessResponseHeaders_HeaderOnlyResponsePassesUnsupportedEncoding(t *testing.T) {
	execCtx, _ := newEncodingTestContext(t, false)
	execCtx.buildRequestContexts(postRequestHeaders(""), RouteMetadata{})

	headers := okResponseHeaders("snappy")
	headers.EndOfStream = true

	resp, err := execCtx.processResponseHeaders(context.Background(), headers)
	require.NoError(t, err)

	assert.True(t, execCtx.responseHasNoBody())
	assert.Nil(t, resp.GetImmediateResponse(), "a bodyless response has nothing to decode and must not be failed")
}

// The first chunk of a stream may legally carry a single byte — one byte short of
// the zlib header needed to tell the two "deflate" variants apart. Choosing the
// decoder from it pinned a raw-deflate stream to the zlib decoder, which then
// rejected the stream. The kernel must buffer until the variant is decidable.
func TestProcessStreamingRequestBody_RawDeflateWithOneByteFirstChunk(t *testing.T) {
	plaintext := `{"prompt":"raw deflate streamed one byte at a time"}`

	var seenByPolicies strings.Builder
	kernel := NewKernel()
	server := NewExternalProcessorServer(kernel, newTestExecutor(), config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)
	execCtx := newPolicyExecutionContext(server, "test-route", &registry.PolicyChain{
		RequiresRequestBody:      true,
		SupportsRequestStreaming: true,
		Policies:                 []policy.Policy{&chunkRecordingRequestPolicy{seen: &seenByPolicies}},
		PolicySpecs:              []policy.PolicySpec{{Enabled: true}},
	})
	execCtx.buildRequestContexts(postRequestHeaders("deflate"), RouteMetadata{})
	execCtx.isStreamingRequest = true

	rawDeflate, err := recompressBody([]byte(plaintext), encodingDeflateRaw)
	require.NoError(t, err)

	// First chunk is one byte: not enough evidence for the variant.
	var wire bytes.Buffer
	for i, in := range [][]byte{rawDeflate[:1], rawDeflate[1:]} {
		resp, err := execCtx.processRequestBody(context.Background(), &extprocv3.HttpBody{
			Body:        in,
			EndOfStream: i == 1,
		})
		require.NoError(t, err, "chunk %d must not fail the stream", i)
		wire.Write(resp.GetRequestBody().GetResponse().GetBodyMutation().GetStreamedResponse().GetBody())
	}

	assert.Equal(t, encodingDeflateRaw, execCtx.requestContentEncoding,
		"the raw variant must be detected once enough bytes have arrived")
	assert.Equal(t, plaintext, seenByPolicies.String(), "policies must observe the full plaintext")

	got, err := decompressBody(wire.Bytes(), execCtx.requestContentEncoding, 0)
	require.NoError(t, err)
	assert.Equal(t, plaintext, string(got),
		"the upstream-bound body must round-trip in the variant the client sent")
}

// A streaming body that declares a Content-Encoding but delivers zero bytes must
// reach the decoder rather than skipping it: the streaming and buffered paths
// must agree on whether such a body is valid for that codec (gzip/br/deflate
// have a mandatory header and fail; zstd's decoder accepts an empty stream).
// Skipping decoder construction entirely made streaming silently forward a body
// under a Content-Encoding nothing had validated or read.
func TestProcessStreamingBody_EmptyEncodedStreamMatchesBufferedVerdict(t *testing.T) {
	for _, encoding := range []string{"gzip", "br", "zstd", "deflate"} {
		t.Run(encoding, func(t *testing.T) {
			// The buffered path is the reference verdict for this codec.
			_, bufferedErr := decompressBody(nil, encoding, testMaxDecompressedBytes)

			kernel := NewKernel()
			server := NewExternalProcessorServer(kernel, newTestExecutor(), config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)
			chain := &registry.PolicyChain{
				RequiresRequestBody:       true,
				RequiresResponseBody:      true,
				SupportsRequestStreaming:  true,
				SupportsResponseStreaming: true,
				Policies:                  []policy.Policy{&testutils.NoopPolicy{}},
				PolicySpecs:               []policy.PolicySpec{{Enabled: true}},
			}

			reqCtx := newPolicyExecutionContext(server, "test-route", chain)
			reqCtx.buildRequestContexts(postRequestHeaders(encoding), RouteMetadata{})
			reqCtx.isStreamingRequest = true
			_, reqErr := reqCtx.processStreamingRequestBody(context.Background(), &extprocv3.HttpBody{
				Body:        nil,
				EndOfStream: true,
			})

			respCtx := newPolicyExecutionContext(server, "test-route", chain)
			respCtx.buildRequestContexts(postRequestHeaders(""), RouteMetadata{})
			respCtx.buildResponseContexts(okResponseHeaders(encoding))
			respCtx.isStreamingResponse = true
			_, respErr := respCtx.processStreamingResponseBody(context.Background(), &extprocv3.HttpBody{
				Body:        nil,
				EndOfStream: true,
			})

			if bufferedErr != nil {
				require.Error(t, reqErr, "empty encoded request stream must fail as it does when buffered")
				require.Error(t, respErr, "empty encoded response stream must fail as it does when buffered")
				assert.Nil(t, reqCtx.requestStreamDecomp)
				assert.Nil(t, respCtx.responseStreamDecomp)
				return
			}
			require.NoError(t, reqErr)
			require.NoError(t, respErr)
			assert.NotNil(t, reqCtx.requestStreamDecomp, "the decoder must be built and consulted, not skipped")
			assert.NotNil(t, respCtx.responseStreamDecomp)
		})
	}
}

// indexRecordingPolicy records the Index of every chunk the kernel delivers.
type indexRecordingPolicy struct {
	seen []uint64
}

func (p *indexRecordingPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{ResponseBodyMode: policy.BodyModeStream}
}

func (p *indexRecordingPolicy) OnResponseBody(_ context.Context, _ *policy.ResponseContext, _ map[string]interface{}) policy.ResponseAction {
	return policy.DownstreamResponseModifications{}
}

// NeedsMoreResponseData returns false so the kernel flushes every chunk, which
// is what a policy doing its own accumulation asks for.
func (p *indexRecordingPolicy) NeedsMoreResponseData(_ []byte) bool { return false }

func (p *indexRecordingPolicy) OnResponseBodyChunk(_ context.Context, _ *policy.ResponseStreamContext, chunk *policy.StreamBody, _ map[string]interface{}) policy.StreamingResponseAction {
	p.seen = append(p.seen, chunk.Index)
	return policy.ForwardResponseChunk{}
}

// A policy that stitches chunks together tells a new chunk from a redelivery by
// its Index, so the kernel must advance it on every delivery. When it does not,
// such a policy keeps only the first chunk and silently works off a partial
// body — which is how streaming LLM responses ended up priced at zero.
func TestStreamingResponse_ChunkIndexAdvancesPerDelivery(t *testing.T) {
	rec := &indexRecordingPolicy{}

	kernel := NewKernel()
	server := NewExternalProcessorServer(kernel, newTestExecutor(), config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)
	chain := &registry.PolicyChain{
		RequiresResponseBody:      true,
		SupportsResponseStreaming: true,
		Policies:                  []policy.Policy{rec},
		PolicySpecs:               []policy.PolicySpec{{Enabled: true}},
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	execCtx.buildRequestContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}}, RouteMetadata{})
	execCtx.buildResponseContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":status", RawValue: []byte("200")},
				{Key: "content-type", RawValue: []byte("text/event-stream")},
			},
		},
	})

	chunks := []string{
		"data: {\"model\":\"m\"}\n\n",
		"data: {\"choices\":[]}\n\n",
		"data: {\"usage\":{\"prompt_tokens\":6}}\n\n",
		"data: [DONE]\n\n",
	}
	for i, c := range chunks {
		_, err := execCtx.processStreamingResponseBody(context.Background(), &extprocv3.HttpBody{
			Body:        []byte(c),
			EndOfStream: i == len(chunks)-1,
		})
		require.NoError(t, err)
	}

	require.Len(t, rec.seen, len(chunks), "every chunk should reach the policy")
	for i := 1; i < len(rec.seen); i++ {
		assert.Greater(t, rec.seen[i], rec.seen[i-1],
			"chunk %d index %d must exceed chunk %d index %d; equal indices make accumulating policies discard the chunk",
			i, rec.seen[i], i-1, rec.seen[i-1])
	}
}
