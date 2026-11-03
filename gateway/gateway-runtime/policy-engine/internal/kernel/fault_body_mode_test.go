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
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocconfigv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// responseBodyModeFor returns the response body mode sent to Envoy at the response-headers
// phase of a REST request answered with status and content type.
func responseBodyModeFor(t *testing.T, chain *registry.PolicyChain, status, contentType string) extprocconfigv3.ProcessingMode_BodySendMode {
	t.Helper()
	server := NewExternalProcessorServer(NewKernel(), executor.NewChainExecutor(nil, nil, nil),
		config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes, nil)
	ec := newPolicyExecutionContext(server, "test-route", chain)
	ec.buildRequestContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}},
		RouteMetadata{APIKind: string(policy.APIKindRestApi)})
	ec.buildResponseContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
		{Key: ":status", RawValue: []byte(status)},
		{Key: "content-type", RawValue: []byte(contentType)},
	}}})
	ec.isStreamingResponse = ec.responseStreamingEnabled(false)
	ec.phase = phaseResponseHeaders
	return ec.getModeOverride().ResponseBodyMode
}

// When only the fault policies need the response body (the collector's fault-path policy on
// a REST API with response-body capture off), a successful response never passes through
// the engine -- a streamed one reaches the client as it arrives -- and an error response is
// buffered for the fault policies.
func TestFaultOnlyChain_RequestsResponseBodyOnlyForErrors(t *testing.T) {
	newChain := func() *registry.PolicyChain {
		// What BuildPolicyChain produces with no response-body policy, then the fault chain.
		chain := &registry.PolicyChain{HasFaultPolicies: true}
		ApplyFaultPoliciesBodyRequirement(chain)
		return chain
	}
	cases := []struct {
		name, status, contentType string
		want                      extprocconfigv3.ProcessingMode_BodySendMode
	}{
		{"SSE 200 streams through Envoy untouched", "200", "text/event-stream", extprocconfigv3.ProcessingMode_NONE},
		{"JSON 200 skips the engine", "200", "application/json", extprocconfigv3.ProcessingMode_NONE},
		{"upstream 503 is buffered for the fault policies", "503", "application/json", extprocconfigv3.ProcessingMode_BUFFERED},
		{"client error 404 is buffered for the fault policies", "404", "application/json", extprocconfigv3.ProcessingMode_BUFFERED},
		{"an error framed as SSE is still buffered", "500", "text/event-stream", extprocconfigv3.ProcessingMode_BUFFERED},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, responseBodyModeFor(t, newChain(), tc.status, tc.contentType))
		})
	}
}

// Before the response headers arrive there is no status, so a fault-only chain asks Envoy
// for no response body; the response-headers phase decides per response.
func TestFaultOnlyChain_NoResponseBodyBeforeResponseHeaders(t *testing.T) {
	chain := &registry.PolicyChain{HasFaultPolicies: true}
	ApplyFaultPoliciesBodyRequirement(chain)
	server := NewExternalProcessorServer(NewKernel(), executor.NewChainExecutor(nil, nil, nil),
		config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes, nil)
	ec := newPolicyExecutionContext(server, "test-route", chain)
	ec.buildRequestContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}},
		RouteMetadata{APIKind: string(policy.APIKindRestApi)})
	ec.phase = phaseRequestHeaders
	assert.Equal(t, extprocconfigv3.ProcessingMode_NONE, ec.getModeOverride().ResponseBodyMode)
}

// A normal policy that streams the response body is unaffected by the fault chain: a
// streamed success still streams through the engine, and an error is buffered as before.
func TestFaultChainWithStreamingBodyPolicy_Unchanged(t *testing.T) {
	newChain := func() *registry.PolicyChain {
		chain := &registry.PolicyChain{HasFaultPolicies: true, RequiresResponseBody: true, SupportsResponseStreaming: true}
		ApplyFaultPoliciesBodyRequirement(chain)
		return chain
	}
	assert.Equal(t, extprocconfigv3.ProcessingMode_FULL_DUPLEX_STREAMED,
		responseBodyModeFor(t, newChain(), "200", "text/event-stream"))
	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED,
		responseBodyModeFor(t, newChain(), "200", "application/json"))
}
