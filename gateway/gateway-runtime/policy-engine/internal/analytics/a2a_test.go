/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package analytics

import (
	"encoding/json"
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	v3 "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// ─── Test helpers ────────────────────────────────────────────────────────────

type a2aEntryOptions struct {
	operation         string
	transport         string
	protocolVersion   string
	contextID         string
	taskID            string
	messageID         string
	responseIsError   bool
	responseErrorCode *int
	terminalReason    string
	statusCode        uint32
	requestMethod     corev3.RequestMethod
	upstreamContacted bool
	omitRequestProps  bool
	omitResponseProps bool
}

// a2aLogEntry builds the ALS entry an Agent request produces, the way the engine and
// the analytics system policy would actually have populated it.
func a2aLogEntry(opts a2aEntryOptions) *v3.HTTPAccessLogEntry {
	metadata := map[string]string{
		APITypeKey: string(policy.APIKindAgent),
		APIIDKey:   "agent-1",
		APINameKey: "WeatherAgent",
	}
	if opts.operation != "" {
		metadata[ResolvedOperationKey] = opts.operation
	}
	if opts.terminalReason != "" {
		metadata[TerminalReasonKey] = opts.terminalReason
	}
	if !opts.omitRequestProps {
		reqProps := map[string]any{}
		if opts.transport != "" {
			reqProps["transport"] = opts.transport
		}
		if opts.protocolVersion != "" {
			reqProps["protocolVersion"] = opts.protocolVersion
		}
		if opts.contextID != "" {
			reqProps["contextId"] = opts.contextID
		}
		if opts.taskID != "" {
			reqProps["taskId"] = opts.taskID
		}
		if opts.messageID != "" {
			reqProps["messageId"] = opts.messageID
		}
		if len(reqProps) > 0 {
			encoded, err := json.Marshal(reqProps)
			if err != nil {
				panic(err)
			}
			metadata[A2ARequestPropertiesKey] = string(encoded)
		}
	}
	if !opts.omitResponseProps {
		respProps := map[string]any{"isError": opts.responseIsError}
		if opts.responseErrorCode != nil {
			respProps["errorCode"] = *opts.responseErrorCode
		}
		encoded, err := json.Marshal(respProps)
		if err != nil {
			panic(err)
		}
		metadata[A2AResponsePropertiesKey] = string(encoded)
	}

	entry := createLogEntryWithMetadata(metadata)
	status := opts.statusCode
	if status == 0 {
		status = 200
	}
	entry.Response.ResponseCode = wrapperspb.UInt32(status)
	if opts.requestMethod != corev3.RequestMethod_METHOD_UNSPECIFIED {
		entry.Request.RequestMethod = opts.requestMethod
	} else {
		entry.Request.RequestMethod = corev3.RequestMethod_POST
	}
	if opts.upstreamContacted {
		entry.CommonProperties.UpstreamRemoteAddress = &corev3.Address{
			Address: &corev3.Address_SocketAddress{
				SocketAddress: &corev3.SocketAddress{Address: "10.1.2.3"},
			},
		}
	}
	return entry
}

func a2aBlock(t *testing.T, entry *v3.HTTPAccessLogEntry) map[string]interface{} {
	t.Helper()
	event := NewAnalytics(&config.Config{}).prepareAnalyticEvent(entry)
	require.NotNil(t, event)
	block, ok := event.Properties[A2AAnalyticsProperty].(map[string]interface{})
	require.True(t, ok, "expected an %s block on an Agent event", A2AAnalyticsProperty)
	return block
}

// ─── Transport convergence ───────────────────────────────────────────────────

// The core Section 12 requirement: the two A2A bindings of one operation must
// aggregate to the same operation while staying distinguishable by transport. If the
// operation differed, both transports of the same call would land in separate buckets
// of every operation-usage rollup.
func TestA2AAnalytics_TransportsConvergeOnOneOperationWithDistinctTransports(t *testing.T) {
	jsonrpc := a2aBlock(t, a2aLogEntry(a2aEntryOptions{
		operation: "SendMessage", transport: "JSONRPC", protocolVersion: "1.0",
	}))
	httpjson := a2aBlock(t, a2aLogEntry(a2aEntryOptions{
		operation: "SendMessage", transport: "HTTP+JSON", protocolVersion: "1.0",
	}))

	assert.Equal(t, "SendMessage", jsonrpc["operation"])
	assert.Equal(t, jsonrpc["operation"], httpjson["operation"],
		"both transports must aggregate under one canonical operation")

	assert.Equal(t, "JSONRPC", jsonrpc["transport"])
	assert.Equal(t, "HTTP+JSON", httpjson["transport"])
	assert.NotEqual(t, jsonrpc["transport"], httpjson["transport"],
		"transport must stay a separate dimension, not be folded into the operation")

	// Both are invocations, both succeeded.
	for name, block := range map[string]map[string]interface{}{"jsonrpc": jsonrpc, "httpjson": httpjson} {
		assert.Equal(t, A2ARequestTypeOperation, block["requestType"], name)
		assert.Equal(t, A2AOutcomeSuccess, block["outcome"], name)
		assert.Equal(t, "1.0", block["protocolVersion"], name)
	}
}

// ─── Outcome derived from the A2A result, not the HTTP status ────────────────

// A JSON-RPC error travels inside a 200. A success rate computed from the HTTP status
// would count this as a success — the single most consequential way an A2A dashboard
// can be silently wrong.
func TestA2AAnalytics_JSONRPCErrorInsideA200IsAFailure(t *testing.T) {
	code := -32601
	block := a2aBlock(t, a2aLogEntry(a2aEntryOptions{
		operation: "SendMessage", transport: "JSONRPC", protocolVersion: "1.0",
		statusCode: 200, upstreamContacted: true,
		responseIsError: true, responseErrorCode: &code,
	}))

	assert.Equal(t, A2AOutcomeFailure, block["outcome"])
	assert.Equal(t, A2AFailureOriginUpstream, block["failureOrigin"],
		"a JSON-RPC error object is the agent's own failure")
	assert.Equal(t, true, block["isError"])
	assert.EqualValues(t, code, block["errorCode"])
}

func TestA2AAnalytics_CleanTwoHundredIsASuccessWithNoFailureOrigin(t *testing.T) {
	block := a2aBlock(t, a2aLogEntry(a2aEntryOptions{
		operation: "GetTask", transport: "HTTP+JSON", protocolVersion: "1.0",
		statusCode: 200, upstreamContacted: true,
	}))

	assert.Equal(t, A2AOutcomeSuccess, block["outcome"])
	assert.NotContains(t, block, "failureOrigin",
		"a successful invocation must carry no failure origin at all")
}

func TestA2AAnalytics_FailureOrigin(t *testing.T) {
	tests := []struct {
		name              string
		statusCode        uint32
		terminalReason    string
		upstreamContacted bool
		wantOrigin        string
	}{
		{
			name:              "policy short-circuit is attributed to the policy layer",
			statusCode:        429,
			terminalReason:    constants.TerminalReasonPolicyDenied,
			upstreamContacted: false,
			wantOrigin:        A2AFailureOriginPolicy,
		},
		{
			// Same status as the case above. Without the terminal reason these two are
			// indistinguishable, and the agent gets blamed for the gateway's throttling.
			name:              "the agent's own 429 is attributed to the agent",
			statusCode:        429,
			upstreamContacted: true,
			wantOrigin:        A2AFailureOriginUpstream,
		},
		{
			name:              "a 5xx from the agent is the agent's",
			statusCode:        502,
			upstreamContacted: true,
			wantOrigin:        A2AFailureOriginUpstream,
		},
		{
			name:              "a 5xx with no upstream contacted is the gateway's",
			statusCode:        500,
			upstreamContacted: false,
			wantOrigin:        A2AFailureOriginGateway,
		},
		{
			name:              "a 4xx the gateway raised before reaching the agent is the caller's",
			statusCode:        400,
			upstreamContacted: false,
			wantOrigin:        A2AFailureOriginClient,
		},
		{
			name:              "an oversized body rejected pre-upstream is the caller's",
			statusCode:        413,
			upstreamContacted: false,
			wantOrigin:        A2AFailureOriginClient,
		},
		{
			name:              "a 404 from the agent is the agent's",
			statusCode:        404,
			upstreamContacted: true,
			wantOrigin:        A2AFailureOriginUpstream,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			block := a2aBlock(t, a2aLogEntry(a2aEntryOptions{
				operation: "SendMessage", transport: "JSONRPC", protocolVersion: "1.0",
				statusCode: tc.statusCode, terminalReason: tc.terminalReason,
				upstreamContacted: tc.upstreamContacted,
			}))
			assert.Equal(t, A2AOutcomeFailure, block["outcome"])
			assert.Equal(t, tc.wantOrigin, block["failureOrigin"])
		})
	}
}

// A policy denial is authoritative even when the body happens to parse as a clean
// JSON-RPC result: the request never reached the agent, so whatever is in the body
// cannot be the agent's answer.
func TestA2AAnalytics_PolicyDenialOutranksTheResponseBody(t *testing.T) {
	block := a2aBlock(t, a2aLogEntry(a2aEntryOptions{
		operation: "SendMessage", transport: "JSONRPC", protocolVersion: "1.0",
		statusCode: 401, terminalReason: constants.TerminalReasonPolicyDenied,
		responseIsError: false,
	}))
	assert.Equal(t, A2AFailureOriginPolicy, block["failureOrigin"])
}

// A policy that *answers* is not a policy that refused.
//
// A managed protected Agent Card is served by the gateway's own A2A policy with a
// 200, which stamps the same policy-short-circuit terminal reason as an auth
// denial does. Reading that reason alone would report every locally served card
// as a policy failure — and the more often a card is fetched, the more broken the
// Agent would look.
//
// It is still an operation event: GetExtendedAgentCard ran, its policies ran, and
// a client received a result. Only the outcome derivation changes.
func TestA2AAnalytics_APolicyThatAnswersSuccessfullyIsNotAFailure(t *testing.T) {
	t.Run("HTTP+JSON, where a 2xx is the agent's statement of success", func(t *testing.T) {
		block := a2aBlock(t, a2aLogEntry(a2aEntryOptions{
			operation: "GetExtendedAgentCard", transport: "HTTP+JSON", protocolVersion: "1.0",
			statusCode: 200, terminalReason: constants.TerminalReasonPolicyDenied,
			upstreamContacted: false,
			omitResponseProps: true,
		}))

		assert.Equal(t, A2ARequestTypeOperation, block["requestType"],
			"an extended-card request is the operation, never discovery")
		assert.Equal(t, "GetExtendedAgentCard", block["operation"])
		assert.Equal(t, A2AOutcomeSuccess, block["outcome"])
		assert.NotContains(t, block, "failureOrigin")
	})

	// JSON-RPC answers 200 whether the call succeeded or failed, and an
	// ImmediateResponse from the request phase is never seen by a response-body
	// policy — so there is no readable result. Undetermined is the honest answer
	// and the pre-existing rule for that case; what matters here is that it is not
	// reported as a *failure*.
	t.Run("JSON-RPC, where a 2xx carries no outcome information", func(t *testing.T) {
		block := a2aBlock(t, a2aLogEntry(a2aEntryOptions{
			operation: "GetExtendedAgentCard", transport: "JSONRPC", protocolVersion: "1.0",
			statusCode: 200, terminalReason: constants.TerminalReasonPolicyDenied,
			upstreamContacted: false,
			// An ImmediateResponse from the request phase never reaches a
			// response-body policy, so no A2A result was observed at all.
			omitResponseProps: true,
		}))

		assert.Equal(t, A2ARequestTypeOperation, block["requestType"])
		assert.Equal(t, A2AOutcomeUnknown, block["outcome"])
		assert.NotContains(t, block, "failureOrigin")
	})

	// And the refusal is unchanged: a 401 from the same policy on the same
	// operation is still attributed to the policy layer, which is the distinction
	// the terminal reason exists to make.
	t.Run("the same policy refusing is still a policy failure", func(t *testing.T) {
		block := a2aBlock(t, a2aLogEntry(a2aEntryOptions{
			operation: "GetExtendedAgentCard", transport: "HTTP+JSON", protocolVersion: "1.0",
			statusCode: 401, terminalReason: constants.TerminalReasonPolicyDenied,
			upstreamContacted: false,
		}))

		assert.Equal(t, A2ARequestTypeOperation, block["requestType"])
		assert.Equal(t, A2AOutcomeFailure, block["outcome"])
		assert.Equal(t, A2AFailureOriginPolicy, block["failureOrigin"])
	})
}

// ─── Card and preflight traffic is not an invocation ────────────────────────

func TestA2AAnalytics_CardAndPreflightAreReportedSeparately(t *testing.T) {
	tests := []struct {
		name            string
		method          corev3.RequestMethod
		wantRequestType string
	}{
		{"a card fetch", corev3.RequestMethod_GET, A2ARequestTypeAgentCard},
		{"a CORS preflight", corev3.RequestMethod_OPTIONS, A2ARequestTypePreflight},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// No resolved operation: the card and preflight routes are resolved by
			// route identity, so the a2a resolver never runs on them.
			block := a2aBlock(t, a2aLogEntry(a2aEntryOptions{
				requestMethod: tc.method, omitRequestProps: true, omitResponseProps: true,
			}))

			assert.Equal(t, tc.wantRequestType, block["requestType"])
			// None of the invocation dimensions may be present, or a downstream
			// rollup could count card polling as agent traffic.
			for _, dimension := range []string{"operation", "transport", "outcome", "failureOrigin"} {
				assert.NotContains(t, block, dimension,
					"%s must not be shaped like an invocation", tc.name)
			}
		})
	}
}

// A request the resolver rejected is an attempted invocation that failed, not a card
// fetch. It has no operation — that is what failed — so it groups as unknown rather
// than echoing the caller-supplied value that did not resolve, which would make the
// operation dimension unbounded.
//
// Before this, such a request produced no analytics event at all: the rejection is the
// only ext_proc response it generates and it carried no dynamic metadata, so the
// access-log entry had nothing identifying the API. The failure was visible only in
// resolution_failures_total.
func TestA2AAnalytics_ResolutionFailureIsAClientSideInvocationFailure(t *testing.T) {
	block := a2aBlock(t, a2aLogEntry(a2aEntryOptions{
		// No resolved operation: resolution is what failed.
		terminalReason: constants.TerminalReasonResolutionFailed,
		statusCode:     400, upstreamContacted: false,
		omitRequestProps: true, omitResponseProps: true,
	}))

	assert.Equal(t, A2ARequestTypeOperation, block["requestType"],
		"a rejected operation request must not be misfiled as a card fetch")
	assert.Equal(t, A2AOperationUnknown, block["operation"])
	assert.Equal(t, A2AOutcomeFailure, block["outcome"])
	assert.Equal(t, A2AFailureOriginClient, block["failureOrigin"])
}

// An Agent event whose request/response property blocks are missing entirely — the
// analytics system policy contributed nothing — must still classify as an invocation,
// but must NOT claim a success nobody determined. With no transport either, there is
// not even a status convention to fall back on.
func TestA2AAnalytics_OperationWithoutPolicyPropertiesIsUndetermined(t *testing.T) {
	block := a2aBlock(t, a2aLogEntry(a2aEntryOptions{
		operation: "CancelTask", statusCode: 200, upstreamContacted: true,
		omitRequestProps: true, omitResponseProps: true,
	}))

	assert.Equal(t, A2ARequestTypeOperation, block["requestType"])
	assert.Equal(t, "CancelTask", block["operation"])
	assert.Equal(t, A2AOutcomeUnknown, block["outcome"])
	assert.NotContains(t, block, "failureOrigin",
		"an undetermined outcome is not a failure and has no origin")
	assert.NotContains(t, block, "transport")
}

// The load-bearing case for A2AOutcomeUnknown. On JSON-RPC the status is 200 whether
// the call succeeded or failed, so a 200 whose body could not be read says nothing at
// all — and the response policy correctly omits isError for it. Defaulting that to
// SUCCESS is the false-success the whole outcome derivation exists to prevent.
func TestA2AAnalytics_JSONRPCTwoHundredWithAnUnreadableBodyIsUndetermined(t *testing.T) {
	block := a2aBlock(t, a2aLogEntry(a2aEntryOptions{
		operation: "SendMessage", transport: "JSONRPC", protocolVersion: "1.0",
		statusCode: 200, upstreamContacted: true,
		// The response policy saw a body it could not parse, so it emitted the block
		// without isError.
		omitResponseProps: true,
	}))

	assert.Equal(t, A2AOutcomeUnknown, block["outcome"])
	assert.NotContains(t, block, "failureOrigin")
}

// HTTP+JSON is REST-shaped: an error is a real error status, never a 200. So a 2xx is
// itself the agent's statement of success and a bodiless one — the 204 from
// DeleteTaskPushNotificationConfig — is determined, not missing.
func TestA2AAnalytics_HTTPJSONTwoHundredWithNoBodyIsASuccess(t *testing.T) {
	block := a2aBlock(t, a2aLogEntry(a2aEntryOptions{
		operation: "DeleteTaskPushNotificationConfig", transport: "HTTP+JSON",
		protocolVersion: "1.0", statusCode: 204, upstreamContacted: true,
		omitResponseProps: true,
	}))

	assert.Equal(t, A2AOutcomeSuccess, block["outcome"])
	assert.NotContains(t, block, "failureOrigin")
}

// A determined outcome always wins over the transport convention, in both directions:
// the body is the authority whenever it was actually read.
func TestA2AAnalytics_AReadBodyOutranksTheTransportConvention(t *testing.T) {
	code := -32000
	failed := a2aBlock(t, a2aLogEntry(a2aEntryOptions{
		operation: "SendMessage", transport: "HTTP+JSON", protocolVersion: "1.0",
		statusCode: 200, upstreamContacted: true,
		responseIsError: true, responseErrorCode: &code,
	}))
	assert.Equal(t, A2AOutcomeFailure, failed["outcome"])
	assert.Equal(t, A2AFailureOriginUpstream, failed["failureOrigin"])

	succeeded := a2aBlock(t, a2aLogEntry(a2aEntryOptions{
		operation: "SendMessage", transport: "JSONRPC", protocolVersion: "1.0",
		statusCode: 200, upstreamContacted: true,
	}))
	assert.Equal(t, A2AOutcomeSuccess, succeeded["outcome"])
}

// ─── Cardinality ─────────────────────────────────────────────────────────────

// The caller-controlled identifiers belong on the event, where they correlate one
// request. They are asserted present here so the "active consumers"/correlation
// dimensions are known to exist; the companion assertion that they are not metric
// labels lives in pkg/metrics (TestAgentsTotalCarriesOnlyBoundedLabels).
func TestA2AAnalytics_CarriesTheCorrelationIdentifiers(t *testing.T) {
	block := a2aBlock(t, a2aLogEntry(a2aEntryOptions{
		operation: "SendMessage", transport: "JSONRPC", protocolVersion: "1.0",
		contextID: "ctx-abc", taskID: "task-def", messageID: "msg-ghi",
	}))

	assert.Equal(t, "ctx-abc", block["contextId"])
	assert.Equal(t, "task-def", block["taskId"])
	assert.Equal(t, "msg-ghi", block["messageId"])
}

// ─── Consumer identity ──────────────────────────────────────────────────────

// The "active consumers" family needs a stable consumer identifier per event. The
// section flags it as possibly unavailable and to be reported as deferred if so — it
// is available: the analytics system policy stamps AuthContext.CredentialID
// generically for any authenticated request, whatever the auth type, and it survives
// onto the event alongside the A2A dimensions. Asserted here specifically for an Agent
// event, so the two are known to travel together.
func TestA2AAnalytics_CarriesTheConsumerIdentifierAlongsideTheA2ADimensions(t *testing.T) {
	entry := a2aLogEntry(a2aEntryOptions{
		operation: "SendMessage", transport: "JSONRPC", protocolVersion: "1.0",
	})
	// Added the way the analytics system policy adds it, into the same analytics_data
	// struct the A2A dimensions travel in.
	addAnalyticsMetadata(entry, map[string]string{
		dto.PropKeyAuthCredentialID: "client-123",
		dto.PropKeyAuthType:         "jwt",
	})

	event := NewAnalytics(&config.Config{}).prepareAnalyticEvent(entry)
	require.NotNil(t, event)

	assert.Equal(t, "client-123", event.Properties[dto.PropKeyAuthCredentialID],
		"a stable consumer identifier must reach the event for the active-consumers family")
	block, ok := event.Properties[A2AAnalyticsProperty].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "SendMessage", block["operation"],
		"consumer identity and the A2A dimensions must be on the same event, "+
			"or volume-per-consumer cannot be broken down by operation")
}

// addAnalyticsMetadata folds extra keys into an entry's existing analytics_data
// struct, the way a second contributing policy would.
func addAnalyticsMetadata(entry *v3.HTTPAccessLogEntry, extra map[string]string) {
	fields := entry.CommonProperties.Metadata.
		FilterMetadata[constants.ExtProcFilterName].
		Fields["analytics_data"].GetStructValue().Fields
	for key, value := range extra {
		fields[key] = structpb.NewStringValue(value)
	}
}

// ─── Non-Agent kinds are untouched ───────────────────────────────────────────

func TestA2AAnalytics_NotEmittedForOtherKinds(t *testing.T) {
	for _, kind := range []policy.APIKind{
		policy.APIKindRestApi, policy.APIKindMCP, policy.APIKindLlmProxy,
	} {
		t.Run(string(kind), func(t *testing.T) {
			entry := createLogEntryWithMetadata(map[string]string{
				APITypeKey:           string(kind),
				ResolvedOperationKey: "SendMessage", // even if something stamped one
			})
			event := NewAnalytics(&config.Config{}).prepareAnalyticEvent(entry)
			require.NotNil(t, event)
			assert.NotContains(t, event.Properties, A2AAnalyticsProperty)
		})
	}
}

// ─── Robustness of the property merge ───────────────────────────────────────

// An unparseable block is kept as a raw string rather than dropped: the event stays
// diagnosable and a consumer reading a named dimension sees it as absent, never as a
// wrong value.
func TestMergeJSONProperties_UnparseableIsKeptRawAndNamedDimensionsStayAbsent(t *testing.T) {
	dest := map[string]interface{}{}
	mergeJSONProperties(dest, "{not json", A2ARequestPropertiesKey)

	assert.Equal(t, "{not json", dest[A2ARequestPropertiesKey])
	assert.NotContains(t, dest, "transport")
}

func TestMergeJSONProperties_EmptyIsANoOp(t *testing.T) {
	dest := map[string]interface{}{}
	mergeJSONProperties(dest, "", A2ARequestPropertiesKey)
	assert.Empty(t, dest)
}

// ─── Cross-module and cross-package key spellings ───────────────────────────

// These keys are spelled twice: here, and in the analytics system policy, which is a
// separate Go module and cannot share a constant. A rename on one side alone is
// silent — the dimension simply stops appearing — so the wire spellings are pinned.
// The matching assertion lives in that module's own test.
func TestA2AMetadataKeySpellingsArePinned(t *testing.T) {
	assert.Equal(t, "a2a_request_properties", A2ARequestPropertiesKey)
	assert.Equal(t, "a2a_response_properties", A2AResponsePropertiesKey)
	assert.Equal(t, "x-wso2-resolved-operation", ResolvedOperationKey)
	assert.Equal(t, "x-wso2-terminal-reason", TerminalReasonKey)
	assert.Equal(t, "a2aAnalytics", A2AAnalyticsProperty)

	// The transport value arrives as an opaque string out of Envoy dynamic metadata,
	// having been spelled by common/agentproto at the other end of the pipeline. This
	// package compares it (to decide whether a 2xx is itself an outcome), so a
	// divergence would silently turn every HTTP+JSON success into UNKNOWN.
	assert.Equal(t, "HTTP+JSON", a2aTransportHTTPJSON)
}
