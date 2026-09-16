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
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// Agent response-body streaming, and why it has no APIKindMCP-style carve-out.
//
// A2A's streaming operations (SendStreamingMessage, SubscribeToTask) answer with SSE,
// so response-body streaming is the feature rather than an optimisation: buffering it
// would hold every event back until the task finished, which is the one thing those
// operations exist not to do.
//
// responseStreamingEnabled keeps MCP buffered because an upstream that frames a
// response as chunked and then sends zero body bytes makes Envoy never deliver the
// body phase in FULL_DUPLEX_STREAMED, leaving the request hanging. That condition was
// tested directly against an Agent route and it does reproduce — but only with an
// upstream manufactured to produce it. It is not reachable through a conformant A2A
// agent: a streaming operation must emit at least the initial Task event, so a
// zero-event response is already a protocol violation, and the reference SDK sends
// neither that nor a content-length alongside its SSE framing. Carving out the whole
// kind would therefore trade a working feature for protection against an upstream that
// is broken anyway; an Agent's idleTimeout bounds that case instead, turning a hang
// into a 504.
//
// These tests pin both halves of the resulting behaviour so neither can change without
// a failure naming the reason.

// An Agent route whose upstream answers with SSE streams: the kind is not carved out,
// and the ModeOverride sent to Envoy agrees with the handler the kernel will run.
func TestResponseStreamingEnabled_AgentSSEUpgrades(t *testing.T) {
	kernel := NewKernel()
	server := NewExternalProcessorServer(kernel, executor.NewChainExecutor(nil, nil, nil),
		config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

	chain := &registry.PolicyChain{
		RequiresResponseBody: true,
		// Mirrors the chain every Agent carries with no user policies attached: the
		// only response-body policy is the streaming-capable analytics system policy.
		SupportsResponseStreaming: true,
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	execCtx.buildRequestContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}},
		RouteMetadata{APIKind: string(policy.APIKindAgent)})
	execCtx.buildResponseContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":status", RawValue: []byte("200")},
				// The exact header shape a real A2A agent replies with, taken from
				// a2a-sdk 1.1.2 on both the JSON-RPC and HTTP+JSON bindings — which
				// notably send no content-length.
				{Key: "content-type", RawValue: []byte("text/event-stream; charset=utf-8")},
				{Key: "transfer-encoding", RawValue: []byte("chunked")},
			},
		},
		EndOfStream: false,
	})

	execCtx.isStreamingResponse = execCtx.responseStreamingEnabled(false)
	require.True(t, execCtx.isStreamingResponse,
		"Agent SSE responses must stream: buffering them would withhold every event until the task completed")

	execCtx.phase = phaseResponseHeaders
	mode := execCtx.getModeOverride()
	assert.Equal(t, extprocconfigv3.ProcessingMode_FULL_DUPLEX_STREAMED, mode.ResponseBodyMode)
}

// A non-success reply to a streaming operation is a normal buffered response, not an
// empty stream: both A2A bindings answer an error with a single JSON document, so the
// route that just streamed does not stream this one.
//
// Nothing tells the kernel which operations are streaming ones — the decision rests
// entirely on what the upstream framed — and this is why that is sufficient: an A2A
// error is never framed as an event stream.
func TestResponseStreamingEnabled_AgentBufferedErrorStaysBuffered(t *testing.T) {
	kernel := NewKernel()
	server := NewExternalProcessorServer(kernel, newTestExecutor(),
		config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

	// A response-body policy that mutates, so a mode/handler disagreement would surface
	// as a streamed-response mutation delivered into a buffered body callback — which is
	// what Envoy rejects as a content-length/mutated-body mismatch.
	rewritten := []byte(`{"jsonrpc":"2.0","id":3,"error":{"code":-32001}}`)
	mockPolicy := &testutils.ConfigurableMockPolicy{
		MockMode: policy.ProcessingMode{ResponseBodyMode: policy.BodyModeBuffer},
		OnRespFn: func(_ *policy.ResponseContext, _ map[string]interface{}) policy.ResponseAction {
			return policy.DownstreamResponseModifications{Body: rewritten}
		},
	}

	chain := &registry.PolicyChain{
		RequiresResponseBody:      true,
		SupportsResponseStreaming: true,
		Policies:                  []policy.Policy{mockPolicy},
		PolicySpecs:               []policy.PolicySpec{{Enabled: true}},
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	execCtx.buildRequestContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":path", RawValue: []byte("/alice/a2a/v1")},
				{Key: ":method", RawValue: []byte("POST")},
			},
		},
	}, RouteMetadata{APIKind: string(policy.APIKindAgent)})

	// SubscribeToTask against a task that does not exist: a JSON-RPC error inside a 200,
	// carrying a content-length, on the very endpoint that serves the streaming operations.
	upstreamBody := []byte(`{"jsonrpc":"2.0","id":3,"error":{"code":-32001,"message":"Task not found"}}`)
	headersResp, err := execCtx.processResponseHeaders(context.Background(), &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":status", RawValue: []byte("200")},
				{Key: "content-type", RawValue: []byte("application/json")},
				{Key: "content-length", RawValue: []byte("75")},
			},
		},
		EndOfStream: false,
	})
	require.NoError(t, err)
	require.False(t, execCtx.isStreamingResponse,
		"an A2A error response carries no event framing, so it must not be treated as a stream")
	require.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED, headersResp.ModeOverride.ResponseBodyMode)

	bodyResp, err := execCtx.processResponseBody(context.Background(), &extprocv3.HttpBody{
		Body:        upstreamBody,
		EndOfStream: true,
	})
	require.NoError(t, err)

	mutation := bodyResp.GetResponseBody().GetResponse().GetBodyMutation()
	require.NotNil(t, mutation)
	assert.Nil(t, mutation.GetStreamedResponse(),
		"a buffered body callback must never carry a streamed-response mutation")
	assert.Equal(t, rewritten, mutation.GetBody())
}
