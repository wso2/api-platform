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

package analytics

import (
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	v3 "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// Analytics for a request the engine refused over the A2A protocol version it stated
// (Section 8A).
//
// This is the one A2A event shape assembled entirely without the analytics system
// policy: header validation runs before a chain is bound, so the policy that normally
// emits the A2A request dimensions lives in a chain this request never reached. What
// the event can still say truthfully — and what it must not invent — is the whole
// subject here.

// a2aVersionRejectionEntry builds the ALS entry such a request produces: route-derived
// metadata stamped by the kernel, the resolver's own bounded attributes, and nothing
// from any policy.
func a2aVersionRejectionEntry(transport, version string) *v3.HTTPAccessLogEntry {
	metadata := map[string]string{
		APITypeKey:        string(policy.APIKindAgent),
		APIIDKey:          "agent-1",
		APINameKey:        "WeatherAgent",
		TerminalReasonKey: constants.TerminalReasonA2AVersionRejected,
	}
	if transport != "" {
		metadata[A2ATransportAttributeKey] = transport
	}
	if version != "" {
		metadata[A2AProtocolVersionAttributeKey] = version
	}

	entry := createLogEntryWithMetadata(metadata)
	entry.Response.ResponseCode = wrapperspb.UInt32(400)
	entry.Request.RequestMethod = corev3.RequestMethod_POST
	return entry
}

// The event is an attempted invocation that failed, and the client is answerable for
// it. Misfiling it as a card fetch would quietly remove every rejected request from
// the Agent's success rate — which is the reverse of what an operator watching a
// fleet of clients on the wrong protocol version needs to see.
func TestA2AAnalytics_VersionRejectionIsAFailedInvocationAttributedToTheClient(t *testing.T) {
	block := a2aBlock(t, a2aVersionRejectionEntry("JSONRPC", "1.0"))

	assert.Equal(t, A2ARequestTypeOperation, block.RequestType,
		"a refused operation request is an invocation attempt, not discovery")
	assert.Equal(t, A2AOutcomeFailure, block.Outcome)
	assert.Equal(t, A2AFailureOriginClient, block.FailureOrigin,
		"the agent never saw the request; the version it stated is the client's own")
}

// The operation stays "unknown" even when the route made the intended one obvious.
// No chain ran, so naming one would report policies that never executed — the exact
// divergence BoundResolution.Operation is derived from the chain key to prevent.
func TestA2AAnalytics_VersionRejectionReportsNoOperation(t *testing.T) {
	block := a2aBlock(t, a2aVersionRejectionEntry("HTTP+JSON", "1.0"))

	assert.Equal(t, A2AOperationUnknown, block.Operation)
}

// The two bounded protocol facts survive from the resolver's own attributes, so a
// rejection still says which binding and which configured version it was aimed at.
// Without this the event would identify the Agent and nothing else.
func TestA2AAnalytics_VersionRejectionCarriesTheRoutesTransportAndVersion(t *testing.T) {
	for _, transport := range []string{"JSONRPC", "HTTP+JSON"} {
		t.Run(transport, func(t *testing.T) {
			block := a2aBlock(t, a2aVersionRejectionEntry(transport, "1.0"))

			assert.Equal(t, transport, block.Transport)
			assert.Equal(t, "1.0", block.ProtocolVersion)
		})
	}
}

// The fallback is a fallback. On a request that resolved, the analytics system policy
// assembled the same two facts into the request-properties block, and that block is
// the authority — a resolver attribute must never overwrite it, or a later change to
// one of the two sources would silently win over the other.
func TestA2AAnalytics_ResolverAttributesNeverOverrideThePolicysProperties(t *testing.T) {
	entry := a2aLogEntry(a2aEntryOptions{
		operation: "SendMessage", transport: "JSONRPC", protocolVersion: "1.0",
	})
	// Both sources present and disagreeing, which only a bug could produce — the
	// point is which one wins.
	fields := entry.CommonProperties.Metadata.FilterMetadata[constants.ExtProcFilterName].
		Fields["analytics_data"].GetStructValue().Fields
	fields[A2ATransportAttributeKey] = structpb.NewStringValue("HTTP+JSON")
	fields[A2AProtocolVersionAttributeKey] = structpb.NewStringValue("9.9")

	block := a2aBlock(t, entry)
	assert.Equal(t, "JSONRPC", block.Transport)
	assert.Equal(t, "1.0", block.ProtocolVersion)
}

// A rejection whose route facts did not arrive reports neither dimension rather than
// an empty one, so a consumer can tell "not recorded" from "recorded as blank".
func TestA2AAnalytics_VersionRejectionOmitsAbsentRouteFacts(t *testing.T) {
	block := a2aBlock(t, a2aVersionRejectionEntry("", ""))

	assert.Empty(t, block.Transport)
	assert.Empty(t, block.ProtocolVersion)
	// The classification does not depend on them.
	assert.Equal(t, A2ARequestTypeOperation, block.RequestType)
	assert.Equal(t, A2AOutcomeFailure, block.Outcome)
}

// The attribute names are a wire contract with the a2a resolver, which lives in a
// package this one cannot import for the same reason the property keys above are
// mirrored literals. A rename on either side would silently drop both dimensions from
// every rejection event, so the spelling is pinned.
func TestA2AAnalytics_ResolverAttributeKeysMatchTheResolversSpelling(t *testing.T) {
	assert.Equal(t, "a2a.transport", A2ATransportAttributeKey)
	assert.Equal(t, "a2a.protocol.version", A2AProtocolVersionAttributeKey)
}
