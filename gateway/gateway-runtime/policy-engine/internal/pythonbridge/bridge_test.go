package pythonbridge

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/pythonbridge/proto"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

func TestBuildRequestHeadersRequestPreservesVhostAndStructuredAuth(t *testing.T) {
	bridge := &bridge{
		policyName:    "demo-policy",
		policyVersion: "v1.0.0",
		metadata:      policy.PolicyMetadata{RouteName: "route-a"},
		translator:    NewTranslator(),
		instanceID:    "instance-1",
	}

	reqCtx := &policy.RequestHeaderContext{
		SharedContext: &policy.SharedContext{
			ProjectID:     "project-1",
			RequestID:     "shared-request-1",
			Metadata:      map[string]interface{}{"flag": true},
			APIId:         "api-1",
			APIName:       "PetStore",
			APIVersion:    "v1",
			APIKind:       policy.APIKindRestApi,
			APIContext:    "/petstore",
			OperationPath: "/pets/{id}",
			AuthContext: &policy.AuthContext{
				Authenticated: true,
				Authorized:    true,
				AuthType:      "jwt",
				Subject:       "alice",
				Scopes:        map[string]bool{"read:pets": true},
				Previous: &policy.AuthContext{
					Authenticated: true,
					AuthType:      "apikey",
					Subject:       "legacy-client",
				},
			},
		},
		Headers:   policy.NewHeaders(map[string][]string{"X-Trace": {"one", "two"}}),
		Path:      "/petstore/v1/pets/123",
		Method:    "GET",
		Authority: "gateway.example.com",
		Scheme:    "https",
		Vhost:     "public.example.com",
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	req, err := bridge.buildRequestHeadersRequest(ctx, reqCtx, map[string]interface{}{"enabled": true})
	require.NoError(t, err)

	payload := req.GetRequestHeaders()
	require.NotNil(t, payload)
	assert.Equal(t, "public.example.com", payload.GetContext().GetVhost())
	assert.Equal(t, []string{"one", "two"}, payload.GetContext().GetHeaders().GetValues()["x-trace"].GetValues())
	assert.Equal(t, "jwt", req.GetSharedContext().GetAuthContext().GetAuthType())
	require.NotNil(t, req.GetSharedContext().GetAuthContext().GetPrevious())
	assert.Equal(t, "apikey", req.GetSharedContext().GetAuthContext().GetPrevious().GetAuthType())
	assert.Equal(t, proto.Phase_PHASE_REQUEST_HEADERS, req.GetExecutionMetadata().GetPhase())
	assert.Equal(t, "route-a", req.GetExecutionMetadata().GetRouteName())
	assert.True(t, req.GetParams().GetFields()["enabled"].GetBoolValue())
}

func TestBuildRequestChunkRequestPreservesChunkIndex(t *testing.T) {
	bridge := &bridge{
		policyName:    "demo-policy",
		policyVersion: "v1.0.0",
		metadata:      policy.PolicyMetadata{RouteName: "route-stream"},
		translator:    NewTranslator(),
		instanceID:    "instance-2",
	}

	reqCtx := &policy.RequestStreamContext{
		SharedContext: &policy.SharedContext{
			RequestID: "shared-request-2",
			Metadata:  map[string]interface{}{"count": "1"},
		},
		Headers:   policy.NewHeaders(map[string][]string{"content-type": {"application/json"}}),
		Path:      "/stream",
		Method:    "POST",
		Authority: "gateway.example.com",
		Scheme:    "https",
		Vhost:     "stream.example.com",
	}
	chunk := &policy.StreamBody{
		Chunk:       []byte("hello"),
		EndOfStream: true,
		Index:       7,
	}

	req, err := bridge.buildRequestChunkRequest(context.Background(), reqCtx, chunk, nil)
	require.NoError(t, err)

	payload := req.GetRequestChunk()
	require.NotNil(t, payload)
	assert.Equal(t, "stream.example.com", payload.GetContext().GetVhost())
	assert.Equal(t, uint64(7), payload.GetChunk().GetIndex())
	assert.Equal(t, []byte("hello"), payload.GetChunk().GetChunk())
	assert.True(t, payload.GetChunk().GetEndOfStream())
}

func TestMergeMetadataUpdatesSharedContext(t *testing.T) {
	bridge := &bridge{}
	shared := &policy.SharedContext{
		Metadata: map[string]interface{}{"existing": "value"},
	}

	updated, err := structpb.NewStruct(map[string]any{
		"existing": "updated",
		"fresh":    true,
	})
	require.NoError(t, err)

	bridge.mergeMetadata(shared, updated)

	assert.Equal(t, "updated", shared.Metadata["existing"])
	assert.Equal(t, true, shared.Metadata["fresh"])
}

func TestStreamingRequestErrorActionFailsOpen(t *testing.T) {
	action := (&bridge{}).streamingRequestErrorAction(errors.New("boom"))

	_, ok := action.(policy.ForwardRequestChunk)
	require.True(t, ok, "expected ForwardRequestChunk, got %T", action)
}

func TestStreamingResponseErrorActionTerminatesStream(t *testing.T) {
	action := (&bridge{}).streamingResponseErrorAction(errors.New("boom"))

	term, ok := action.(policy.TerminateResponseChunk)
	require.True(t, ok, "expected TerminateResponseChunk, got %T", action)
	assert.True(t, term.TerminateStream())
	assert.Nil(t, term.Body)
}

type fakePythonExecutorClient struct {
	destroyPolicyResp  *proto.DestroyPolicyResponse
	destroyPolicyErr   error
	destroyPolicyCalls int
}

func (f *fakePythonExecutorClient) ExecuteStream(context.Context, ...grpc.CallOption) (grpc.BidiStreamingClient[proto.StreamRequest, proto.StreamResponse], error) {
	return nil, errors.New("unexpected ExecuteStream call")
}

func (f *fakePythonExecutorClient) HealthCheck(context.Context, *proto.HealthCheckRequest, ...grpc.CallOption) (*proto.HealthCheckResponse, error) {
	return nil, errors.New("unexpected HealthCheck call")
}

func (f *fakePythonExecutorClient) InitPolicy(context.Context, *proto.InitPolicyRequest, ...grpc.CallOption) (*proto.InitPolicyResponse, error) {
	return nil, errors.New("unexpected InitPolicy call")
}

func (f *fakePythonExecutorClient) DestroyPolicy(context.Context, *proto.DestroyPolicyRequest, ...grpc.CallOption) (*proto.DestroyPolicyResponse, error) {
	f.destroyPolicyCalls++
	return f.destroyPolicyResp, f.destroyPolicyErr
}

func TestBridgeCloseReturnsExecutorDestroyFailure(t *testing.T) {
	harness := startTestPythonExecutorServer(t, nil)

	sm := NewStreamManager("bufconn")
	sm.dialContext = func(context.Context, string) (net.Conn, error) {
		return harness.listener.Dial()
	}
	require.NoError(t, sm.Connect(context.Background()))
	t.Cleanup(func() {
		_ = sm.Close()
	})

	fakeClient := &fakePythonExecutorClient{
		destroyPolicyResp: &proto.DestroyPolicyResponse{
			Success:      false,
			ErrorMessage: "executor refused destroy",
		},
	}
	sm.client = fakeClient

	bridge := &bridge{
		streamManager: sm,
		slogger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		instanceID:    "instance-1",
	}

	err := bridge.Close()
	require.EqualError(t, err, "destroy Python policy instance rejected by executor: executor refused destroy")

	err = bridge.Close()
	require.EqualError(t, err, "destroy Python policy instance rejected by executor: executor refused destroy")
	assert.Equal(t, 1, fakeClient.destroyPolicyCalls)
}
