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
	"errors"
	"io"
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocconfigv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// =============================================================================
// Mock Stream for Testing (kept local - uses io.EOF specific behavior)
// =============================================================================

// mockExtProcStream implements extprocv3.ExternalProcessor_ProcessServer for testing
type mockExtProcStream struct {
	requests  []*extprocv3.ProcessingRequest
	responses []*extprocv3.ProcessingResponse
	recvIndex int
	recvErr   error
	sendErr   error
	ctx       context.Context
}

func newMockStream(requests []*extprocv3.ProcessingRequest) *mockExtProcStream {
	return &mockExtProcStream{
		requests:  requests,
		responses: make([]*extprocv3.ProcessingResponse, 0),
		ctx:       context.Background(),
	}
}

func (m *mockExtProcStream) Send(resp *extprocv3.ProcessingResponse) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.responses = append(m.responses, resp)
	return nil
}

func (m *mockExtProcStream) Recv() (*extprocv3.ProcessingRequest, error) {
	if m.recvErr != nil {
		return nil, m.recvErr
	}
	if m.recvIndex >= len(m.requests) {
		return nil, io.EOF
	}
	req := m.requests[m.recvIndex]
	m.recvIndex++
	return req, nil
}

func (m *mockExtProcStream) SetHeader(metadata.MD) error  { return nil }
func (m *mockExtProcStream) SendHeader(metadata.MD) error { return nil }
func (m *mockExtProcStream) SetTrailer(metadata.MD)       {}
func (m *mockExtProcStream) Context() context.Context     { return m.ctx }
func (m *mockExtProcStream) SendMsg(interface{}) error    { return nil }
func (m *mockExtProcStream) RecvMsg(interface{}) error    { return nil }

// =============================================================================
// NewExternalProcessorServer Tests
// =============================================================================

func TestNewExternalProcessorServer(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	tracingConfig := config.TracingConfig{}

	server := NewExternalProcessorServer(kernel, chainExecutor, tracingConfig, "test-service")

	require.NotNil(t, server)
	assert.Equal(t, kernel, server.kernel)
	assert.Equal(t, chainExecutor, server.executor)
	assert.NotNil(t, server.tracer)
}

func TestNewExternalProcessorServer_DefaultServiceName(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	tracingConfig := config.TracingConfig{}

	server := NewExternalProcessorServer(kernel, chainExecutor, tracingConfig, "")

	require.NotNil(t, server)
	// Tracer should be created with default name
	assert.NotNil(t, server.tracer)
}

// =============================================================================
// extractRouteMetadata Tests
// =============================================================================

func TestExtractRouteMetadata_NilAttributes(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	req := &extprocv3.ProcessingRequest{
		Attributes: nil,
	}

	metadata := server.extractRouteMetadata(req)

	// When no attributes, route name is empty (not "default" - that's set elsewhere)
	assert.Empty(t, metadata.RouteName)
}

func TestExtractRouteMetadata_EmptyExtProcAttrs(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	req := &extprocv3.ProcessingRequest{
		Attributes: map[string]*structpb.Struct{
			"other": {},
		},
	}

	metadata := server.extractRouteMetadata(req)

	// When ext_proc filter not present, route name is empty
	assert.Empty(t, metadata.RouteName)
}

func TestExtractRouteMetadata_WithRouteName(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	req := &extprocv3.ProcessingRequest{
		Attributes: map[string]*structpb.Struct{
			constants.ExtProcFilter: {
				Fields: map[string]*structpb.Value{
					"xds.route_name": structpb.NewStringValue("my-api-route"),
				},
			},
		},
	}

	metadata := server.extractRouteMetadata(req)

	assert.Equal(t, "my-api-route", metadata.RouteName)
}

func TestExtractRouteMetadata_WithRouteMetadata(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	// Create route metadata in prototext format
	routeMetadataStr := `filter_metadata {
		key: "wso2.route"
		value {
			fields {
				key: "api_id"
				value { string_value: "api-123" }
			}
			fields {
				key: "api_name"
				value { string_value: "PetStore" }
			}
			fields {
				key: "api_version"
				value { string_value: "v1.0.0" }
			}
			fields {
				key: "api_context"
				value { string_value: "/petstore" }
			}
			fields {
				key: "path"
				value { string_value: "/pets/{id}" }
			}
			fields {
				key: "vhost"
				value { string_value: "api.example.com" }
			}
			fields {
				key: "api_kind"
				value { string_value: "REST" }
			}
			fields {
				key: "template_handle"
				value { string_value: "gpt-4" }
			}
			fields {
				key: "provider_name"
				value { string_value: "openai" }
			}
			fields {
				key: "project_id"
				value { string_value: "proj-456" }
			}
		}
	}`

	req := &extprocv3.ProcessingRequest{
		Attributes: map[string]*structpb.Struct{
			constants.ExtProcFilter: {
				Fields: map[string]*structpb.Value{
					"xds.route_name":     structpb.NewStringValue("test-route"),
					"xds.route_metadata": structpb.NewStringValue(routeMetadataStr),
				},
			},
		},
	}

	metadata := server.extractRouteMetadata(req)

	assert.Equal(t, "test-route", metadata.RouteName)
	assert.Equal(t, "api-123", metadata.APIId)
	assert.Equal(t, "PetStore", metadata.APIName)
	assert.Equal(t, "v1.0.0", metadata.APIVersion)
	assert.Equal(t, "/petstore", metadata.Context)
	assert.Equal(t, "/pets/{id}", metadata.OperationPath)
	assert.Equal(t, "api.example.com", metadata.Vhost)
	assert.Equal(t, "REST", metadata.APIKind)
	assert.Equal(t, "gpt-4", metadata.TemplateHandle)
	assert.Equal(t, "openai", metadata.ProviderName)
	assert.Equal(t, "proj-456", metadata.ProjectID)
}

func TestExtractRouteMetadata_MalformedRouteMetadata(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	req := &extprocv3.ProcessingRequest{
		Attributes: map[string]*structpb.Struct{
			constants.ExtProcFilter: {
				Fields: map[string]*structpb.Value{
					"xds.route_name":     structpb.NewStringValue("test-route"),
					"xds.route_metadata": structpb.NewStringValue("not valid prototext {{{"),
				},
			},
		},
	}

	metadata := server.extractRouteMetadata(req)

	// Route name should still be extracted
	assert.Equal(t, "test-route", metadata.RouteName)
	// Other fields should be empty due to parse error
	assert.Empty(t, metadata.APIId)
}

// =============================================================================
// generateRequestID Tests
// =============================================================================

func TestGenerateRequestID(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	id1 := server.generateRequestID()
	id2 := server.generateRequestID()

	// Should generate valid UUIDs
	assert.Len(t, id1, 36) // UUID format: 8-4-4-4-12
	assert.Len(t, id2, 36)
	// Should be unique
	assert.NotEqual(t, id1, id2)
}

// =============================================================================
// skipAllProcessing Tests
// =============================================================================

func TestSkipAllProcessing(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	routeMetadata := RouteMetadata{
		RouteName:  "test-route",
		APIName:    "TestAPI",
		APIVersion: "v1.0",
	}

	resp := server.skipAllProcessing(routeMetadata)

	require.NotNil(t, resp)

	// Check response type
	reqHeaders := resp.GetRequestHeaders()
	require.NotNil(t, reqHeaders)

	// Check mode override
	require.NotNil(t, resp.ModeOverride)
	assert.Equal(t, extprocconfigv3.ProcessingMode_SKIP, resp.ModeOverride.ResponseHeaderMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_SKIP, resp.ModeOverride.RequestTrailerMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_SKIP, resp.ModeOverride.ResponseTrailerMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_NONE, resp.ModeOverride.RequestBodyMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_NONE, resp.ModeOverride.ResponseBodyMode)

	// Check dynamic metadata
	require.NotNil(t, resp.DynamicMetadata)
}

// =============================================================================
// Process Stream Tests
// =============================================================================

func TestProcess_EmptyStream(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	stream := newMockStream([]*extprocv3.ProcessingRequest{})

	err := server.Process(stream)

	assert.NoError(t, err)
	assert.Empty(t, stream.responses)
}

func TestProcess_RequestHeaders_NoPolicyChain(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	req := &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_RequestHeaders{
			RequestHeaders: &extprocv3.HttpHeaders{
				Headers: &corev3.HeaderMap{
					Headers: []*corev3.HeaderValue{
						{Key: ":path", RawValue: []byte("/api/v1/pets")},
						{Key: ":method", RawValue: []byte("GET")},
					},
				},
			},
		},
	}

	stream := newMockStream([]*extprocv3.ProcessingRequest{req})

	err := server.Process(stream)

	assert.NoError(t, err)
	require.Len(t, stream.responses, 1)

	// Should skip all processing when no chain found
	resp := stream.responses[0]
	require.NotNil(t, resp.ModeOverride)
}

func TestProcess_UnknownRequestType(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	// Create a request with nil Request field
	req := &extprocv3.ProcessingRequest{}

	stream := newMockStream([]*extprocv3.ProcessingRequest{req})

	err := server.Process(stream)

	assert.NoError(t, err)
	require.Len(t, stream.responses, 1)

	// Should return immediate response with 500 status
	resp := stream.responses[0]
	immResp := resp.GetImmediateResponse()
	require.NotNil(t, immResp)
	assert.Equal(t, uint32(500), uint32(immResp.Status.Code))
}

func TestProcess_RecvError(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	stream := newMockStream([]*extprocv3.ProcessingRequest{})
	stream.recvErr = errors.New("receive error")

	err := server.Process(stream)

	assert.Error(t, err)
}

func TestProcess_ContextCanceled(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	stream := newMockStream([]*extprocv3.ProcessingRequest{})
	stream.recvErr = context.Canceled

	err := server.Process(stream)

	// Context cancellation should not return error
	assert.NoError(t, err)
}

func TestProcess_SendError(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	req := &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_RequestHeaders{
			RequestHeaders: &extprocv3.HttpHeaders{},
		},
	}

	stream := newMockStream([]*extprocv3.ProcessingRequest{req})
	stream.sendErr = errors.New("send error")

	err := server.Process(stream)

	assert.Error(t, err)
}

// =============================================================================
// handleProcessingPhase Tests - RequestBody
// =============================================================================

func TestHandleProcessingPhase_RequestBody_NoContext(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	req := &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_RequestBody{
			RequestBody: &extprocv3.HttpBody{
				Body:        []byte("test body"),
				EndOfStream: true,
			},
		},
	}

	var execCtx *PolicyExecutionContext = nil

	tracer := otel.Tracer("test")
	_, parentSpan := tracer.Start(context.Background(), "test")
	resp, err := server.handleProcessingPhase(context.Background(), req, &execCtx, parentSpan)

	assert.NoError(t, err)
	require.NotNil(t, resp)

	// Should return empty body response
	bodyResp := resp.GetRequestBody()
	require.NotNil(t, bodyResp)
}

// =============================================================================
// handleProcessingPhase Tests - ResponseHeaders
// =============================================================================

func TestHandleProcessingPhase_ResponseHeaders_NoContext(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	req := &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_ResponseHeaders{
			ResponseHeaders: &extprocv3.HttpHeaders{
				Headers: &corev3.HeaderMap{
					Headers: []*corev3.HeaderValue{
						{Key: ":status", RawValue: []byte("200")},
					},
				},
			},
		},
	}

	var execCtx *PolicyExecutionContext = nil

	tracer := otel.Tracer("test")
	_, parentSpan := tracer.Start(context.Background(), "test")
	resp, err := server.handleProcessingPhase(context.Background(), req, &execCtx, parentSpan)

	assert.NoError(t, err)
	require.NotNil(t, resp)

	// Should return empty response headers
	headersResp := resp.GetResponseHeaders()
	require.NotNil(t, headersResp)
}

// =============================================================================
// handleProcessingPhase Tests - ResponseBody
// =============================================================================

func TestHandleProcessingPhase_ResponseBody_NoContext(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	req := &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_ResponseBody{
			ResponseBody: &extprocv3.HttpBody{
				Body:        []byte("response body"),
				EndOfStream: true,
			},
		},
	}

	var execCtx *PolicyExecutionContext = nil

	tracer := otel.Tracer("test")
	_, parentSpan := tracer.Start(context.Background(), "test")
	resp, err := server.handleProcessingPhase(context.Background(), req, &execCtx, parentSpan)

	assert.NoError(t, err)
	require.NotNil(t, resp)

	// Should return empty body response
	bodyResp := resp.GetResponseBody()
	require.NotNil(t, bodyResp)
}

// =============================================================================
// initializeExecutionContext Tests
// =============================================================================

func TestInitializeExecutionContext_NoPolicyChain(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	req := &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_RequestHeaders{
			RequestHeaders: &extprocv3.HttpHeaders{
				Headers: &corev3.HeaderMap{
					Headers: []*corev3.HeaderValue{
						{Key: ":path", RawValue: []byte("/api/v1/pets")},
					},
				},
			},
		},
		Attributes: map[string]*structpb.Struct{
			constants.ExtProcFilter: {
				Fields: map[string]*structpb.Value{
					"xds.route_name": structpb.NewStringValue("nonexistent-route"),
				},
			},
		},
	}

	var execCtx *PolicyExecutionContext

	routeMeta := server.initializeExecutionContext(context.Background(), req, &execCtx)

	assert.Nil(t, execCtx)
	assert.Equal(t, "nonexistent-route", routeMeta.RouteName)
}

func TestInitializeExecutionContext_WithPolicyChain(t *testing.T) {
	kernel := NewKernel()

	chain := &registry.PolicyChain{
		Policies:    []policy.Policy{},
		PolicySpecs: []policy.PolicySpec{},
	}
	kernel.RegisterRoute("test-route", chain)

	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	req := &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_RequestHeaders{
			RequestHeaders: &extprocv3.HttpHeaders{
				Headers: &corev3.HeaderMap{
					Headers: []*corev3.HeaderValue{
						{Key: ":path", RawValue: []byte("/api/v1/pets")},
						{Key: ":method", RawValue: []byte("GET")},
						{Key: ":authority", RawValue: []byte("api.example.com")},
						{Key: ":scheme", RawValue: []byte("https")},
						{Key: "x-request-id", RawValue: []byte("req-123")},
					},
				},
			},
		},
		Attributes: map[string]*structpb.Struct{
			constants.ExtProcFilter: {
				Fields: map[string]*structpb.Value{
					"xds.route_name": structpb.NewStringValue("test-route"),
				},
			},
		},
	}

	var execCtx *PolicyExecutionContext

	routeMeta := server.initializeExecutionContext(context.Background(), req, &execCtx)

	require.NotNil(t, execCtx)
	assert.Equal(t, "test-route", routeMeta.RouteName)
	assert.Equal(t, "test-route", execCtx.routeKey)
	assert.Equal(t, "req-123", execCtx.requestID)
	assert.NotNil(t, execCtx.requestContext)
	assert.Equal(t, "/api/v1/pets", execCtx.requestContext.Path)
	assert.Equal(t, "GET", execCtx.requestContext.Method)
}
