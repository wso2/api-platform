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

// a2aBlock builds one Agent event and returns its A2A section.
//
// Typed, like the published model: these assertions are the contract with a downstream
// consumer, and asserting them against a map would pass just as happily on a field
// whose value quietly changed type.
func a2aBlock(t *testing.T, entry *v3.HTTPAccessLogEntry) *dto.A2AAnalytics {
	t.Helper()
	event := NewAnalytics(&config.Config{}).prepareAnalyticEvent(entry)
	require.NotNil(t, event)
	envelope, ok := event.Properties[AgentAnalyticsProperty].(*dto.AgentAnalytics)
	require.True(t, ok, "expected an %s envelope on an Agent event", AgentAnalyticsProperty)
	require.NotNil(t, envelope.A2A, "the envelope must carry an a2a section")
	return envelope.A2A
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

	assert.Equal(t, "SendMessage", jsonrpc.Operation)
	assert.Equal(t, jsonrpc.Operation, httpjson.Operation,
		"both transports must aggregate under one canonical operation")

	assert.Equal(t, "JSONRPC", jsonrpc.Transport)
	assert.Equal(t, "HTTP+JSON", httpjson.Transport)
	assert.NotEqual(t, jsonrpc.Transport, httpjson.Transport,
		"transport must stay a separate dimension, not be folded into the operation")

	// Both are invocations, both succeeded.
	for name, block := range map[string]*dto.A2AAnalytics{"jsonrpc": jsonrpc, "httpjson": httpjson} {
		assert.Equal(t, A2ARequestTypeOperation, block.RequestType, name)
		assert.Equal(t, A2AOutcomeSuccess, block.Outcome, name)
		assert.Equal(t, "1.0", block.ProtocolVersion, name)
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

	assert.Equal(t, A2AOutcomeFailure, block.Outcome)
	assert.Equal(t, A2AFailureOriginUpstream, block.FailureOrigin,
		"a JSON-RPC error object is the agent's own failure")
	require.NotNil(t, block.IsError)
	assert.True(t, *block.IsError)
	require.NotNil(t, block.ErrorCode)
	assert.Equal(t, code, *block.ErrorCode)
}

func TestA2AAnalytics_CleanTwoHundredIsASuccessWithNoFailureOrigin(t *testing.T) {
	block := a2aBlock(t, a2aLogEntry(a2aEntryOptions{
		operation: "GetTask", transport: "HTTP+JSON", protocolVersion: "1.0",
		statusCode: 200, upstreamContacted: true,
	}))

	assert.Equal(t, A2AOutcomeSuccess, block.Outcome)
	assert.Empty(t, block.FailureOrigin,
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
			assert.Equal(t, A2AOutcomeFailure, block.Outcome)
			assert.Equal(t, tc.wantOrigin, block.FailureOrigin)
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
	assert.Equal(t, A2AFailureOriginPolicy, block.FailureOrigin)
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

		assert.Equal(t, A2ARequestTypeOperation, block.RequestType,
			"an extended-card request is the operation, never discovery")
		assert.Equal(t, "GetExtendedAgentCard", block.Operation)
		assert.Equal(t, A2AOutcomeSuccess, block.Outcome)
		assert.Empty(t, block.FailureOrigin)
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

		assert.Equal(t, A2ARequestTypeOperation, block.RequestType)
		assert.Equal(t, A2AOutcomeUnknown, block.Outcome)
		assert.Empty(t, block.FailureOrigin)
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

		assert.Equal(t, A2ARequestTypeOperation, block.RequestType)
		assert.Equal(t, A2AOutcomeFailure, block.Outcome)
		assert.Equal(t, A2AFailureOriginPolicy, block.FailureOrigin)
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

			assert.Equal(t, tc.wantRequestType, block.RequestType)
			// None of the invocation dimensions may be present, or a downstream
			// rollup could count card polling as agent traffic. requestType is the
			// only field such an event carries.
			assert.Empty(t, block.Operation, tc.name)
			assert.Empty(t, block.Transport, tc.name)
			assert.Empty(t, block.ProtocolVersion, tc.name)
			assert.Empty(t, block.Outcome, tc.name)
			assert.Empty(t, block.FailureOrigin, tc.name)
			assert.Equal(t, dto.A2ARequestAnalytics{}, block.A2ARequestAnalytics, tc.name)
			assert.Equal(t, dto.A2AResponseAnalytics{}, block.A2AResponseAnalytics, tc.name)
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

	assert.Equal(t, A2ARequestTypeOperation, block.RequestType,
		"a rejected operation request must not be misfiled as a card fetch")
	assert.Equal(t, A2AOperationUnknown, block.Operation)
	assert.Equal(t, A2AOutcomeFailure, block.Outcome)
	assert.Equal(t, A2AFailureOriginClient, block.FailureOrigin)
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

	assert.Equal(t, A2ARequestTypeOperation, block.RequestType)
	assert.Equal(t, "CancelTask", block.Operation)
	assert.Equal(t, A2AOutcomeUnknown, block.Outcome)
	assert.Empty(t, block.FailureOrigin,
		"an undetermined outcome is not a failure and has no origin")
	assert.Empty(t, block.Transport)
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

	assert.Equal(t, A2AOutcomeUnknown, block.Outcome)
	assert.Empty(t, block.FailureOrigin)
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

	assert.Equal(t, A2AOutcomeSuccess, block.Outcome)
	assert.Empty(t, block.FailureOrigin)
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
	assert.Equal(t, A2AOutcomeFailure, failed.Outcome)
	assert.Equal(t, A2AFailureOriginUpstream, failed.FailureOrigin)

	succeeded := a2aBlock(t, a2aLogEntry(a2aEntryOptions{
		operation: "SendMessage", transport: "JSONRPC", protocolVersion: "1.0",
		statusCode: 200, upstreamContacted: true,
	}))
	assert.Equal(t, A2AOutcomeSuccess, succeeded.Outcome)
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

	assert.Equal(t, "ctx-abc", block.ContextID)
	assert.Equal(t, "task-def", block.TaskID)
	assert.Equal(t, "msg-ghi", block.MessageID)
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
	envelope, ok := event.Properties[AgentAnalyticsProperty].(*dto.AgentAnalytics)
	require.True(t, ok)
	require.NotNil(t, envelope.A2A)
	assert.Equal(t, "SendMessage", envelope.A2A.Operation,
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
			assert.NotContains(t, event.Properties, AgentAnalyticsProperty)
		})
	}
}

// ─── The published envelope shape ───────────────────────────────────────────

// The whole published model in one assertion: the envelope's domain key, the A2A
// section's four scalars, and the two flat objects with every field the policy can
// contribute. This is the contract a downstream dashboard is written against, so it is
// pinned as a shape rather than field by field — a field silently moved between the
// A2A level and one of the objects would still satisfy per-field assertions.
func TestAgentAnalytics_PublishedEnvelopeShape(t *testing.T) {
	requestProps, err := json.Marshal(map[string]any{
		"transport":         "JSONRPC",
		"protocolVersion":   "1.0",
		"messageId":         "msg-1",
		"taskId":            "task-existing",
		"contextId":         "ctx-1",
		"inputPartCount":    3,
		"returnImmediately": false,
		"historyLength":     5,
	})
	require.NoError(t, err)
	responseProps, err := json.Marshal(map[string]any{
		"isError":            false,
		"isStreaming":        true,
		"timeToFirstEventMs": 120,
		"streamDurationMs":   850,
		"payloadType":        "task",
		"responseTaskId":     "task-456",
		"responseContextId":  "ctx-1",
		"taskState":          "TASK_STATE_COMPLETED",
	})
	require.NoError(t, err)

	entry := createLogEntryWithMetadata(map[string]string{
		APITypeKey:               string(policy.APIKindAgent),
		APIIDKey:                 "agent-1",
		APINameKey:               "WeatherAgent",
		ResolvedOperationKey:     "SendMessage",
		A2ARequestPropertiesKey:  string(requestProps),
		A2AResponsePropertiesKey: string(responseProps),
	})

	event := NewAnalytics(&config.Config{}).prepareAnalyticEvent(entry)
	require.NotNil(t, event)
	envelope, ok := event.Properties[AgentAnalyticsProperty].(*dto.AgentAnalytics)
	require.True(t, ok, "the Agent envelope must be published as a typed value, not a map")
	require.NotNil(t, envelope.A2A)

	// The four A2A-level scalars plus the derived outcome.
	assert.Equal(t, A2ARequestTypeOperation, envelope.A2A.RequestType)
	assert.Equal(t, "SendMessage", envelope.A2A.Operation)
	assert.Equal(t, "JSONRPC", envelope.A2A.Transport)
	assert.Equal(t, "1.0", envelope.A2A.ProtocolVersion)
	assert.Equal(t, A2AOutcomeSuccess, envelope.A2A.Outcome)
	assert.Empty(t, envelope.A2A.FailureOrigin)

	a2a := envelope.A2A
	assert.Equal(t, "msg-1", a2a.MessageID)
	assert.Equal(t, "task-existing", a2a.TaskID)
	assert.Equal(t, "ctx-1", a2a.ContextID)
	require.NotNil(t, a2a.InputPartCount)
	assert.Equal(t, 3, *a2a.InputPartCount)
	require.NotNil(t, a2a.ReturnImmediately)
	assert.False(t, *a2a.ReturnImmediately)
	require.NotNil(t, a2a.HistoryLength)
	assert.Equal(t, 5, *a2a.HistoryLength)

	require.NotNil(t, a2a.IsError)
	assert.False(t, *a2a.IsError)
	require.NotNil(t, a2a.IsStreaming)
	assert.True(t, *a2a.IsStreaming)
	require.NotNil(t, a2a.TimeToFirstEventMs)
	assert.EqualValues(t, 120, *a2a.TimeToFirstEventMs)
	require.NotNil(t, a2a.StreamDurationMs)
	assert.EqualValues(t, 850, *a2a.StreamDurationMs)
	assert.Equal(t, "task", a2a.PayloadType)
	assert.Equal(t, "TASK_STATE_COMPLETED", a2a.TaskState)

	// The identifiers the caller sent and the ones the agent answered with are
	// carried under distinct names on the one flat object, so a disagreement
	// between the two stays diagnosable.
	assert.Equal(t, "task-456", a2a.ResponseTaskID)
	assert.Equal(t, "ctx-1", a2a.ResponseContextID)
	assert.NotEqual(t, a2a.TaskID, a2a.ResponseTaskID)
}

// The published JSON is the actual contract — a consumer reads names, not Go fields —
// so the serialized document is asserted directly. It pins that the a2a section is one
// flat level, that the two colliding response identifiers are the only prefixed names,
// and the two omissions the model depends on: an absent optional is missing rather than
// null or zero, and there is no schema-version property.
func TestAgentAnalytics_SerializesToTheDocumentedJSON(t *testing.T) {
	requestProps, err := json.Marshal(map[string]any{
		"transport": "JSONRPC", "protocolVersion": "1.0", "messageId": "msg-1",
		"taskId": "task-existing",
	})
	require.NoError(t, err)
	responseProps, err := json.Marshal(map[string]any{
		"isError": false, "payloadType": "task", "taskState": "TASK_STATE_COMPLETED",
		"responseTaskId": "task-456",
	})
	require.NoError(t, err)

	event := NewAnalytics(&config.Config{}).prepareAnalyticEvent(
		createLogEntryWithMetadata(map[string]string{
			APITypeKey:               string(policy.APIKindAgent),
			APIIDKey:                 "agent-1",
			APINameKey:               "WeatherAgent",
			ResolvedOperationKey:     "SendMessage",
			A2ARequestPropertiesKey:  string(requestProps),
			A2AResponsePropertiesKey: string(responseProps),
		}))
	require.NotNil(t, event)

	encoded, err := json.Marshal(event.Properties[AgentAnalyticsProperty])
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"a2a": {
			"requestType": "operation",
			"operation": "SendMessage",
			"transport": "JSONRPC",
			"protocolVersion": "1.0",
			"messageId": "msg-1",
			"taskId": "task-existing",
			"isError": false,
			"payloadType": "task",
			"taskState": "TASK_STATE_COMPLETED",
			"responseTaskId": "task-456",
			"outcome": "SUCCESS"
		}
	}`, string(encoded))
}

// A card fetch and a preflight use the same envelope and carry only requestType, so a
// consumer cannot mistake either for an invocation — and so neither needs a separate
// shape to be recognised by.
func TestAgentAnalytics_CardAndPreflightSerializeToRequestTypeAlone(t *testing.T) {
	for name, method := range map[string]corev3.RequestMethod{
		"agentCard": corev3.RequestMethod_GET,
		"preflight": corev3.RequestMethod_OPTIONS,
	} {
		t.Run(name, func(t *testing.T) {
			event := NewAnalytics(&config.Config{}).prepareAnalyticEvent(
				a2aLogEntry(a2aEntryOptions{
					requestMethod: method, omitRequestProps: true, omitResponseProps: true,
				}))
			require.NotNil(t, event)

			encoded, err := json.Marshal(event.Properties[AgentAnalyticsProperty])
			require.NoError(t, err)
			assert.JSONEq(t, `{"a2a": {"requestType": "`+name+`"}}`, string(encoded))
		})
	}
}

// The field names inside the two blocks are a wire contract with the analytics system
// policy, which is a separate Go module: it serializes these documents and this package
// unmarshals them into the published DTOs. A field renamed on one side alone decodes
// into nothing — the dimension disappears with no error anywhere — so both sides pin
// the names. The matching assertion is TestA2ABlockFieldNamesArePinned in that module.
func TestAgentAnalyticsWireFieldNamesArePinned(t *testing.T) {
	request := decodeA2ARequestBlock(`{
		"transport": "JSONRPC", "protocolVersion": "1.0",
		"messageId": "m-1", "contextId": "ctx-1", "taskId": "task-1",
		"inputPartCount": 2, "returnImmediately": false, "historyLength": 5
	}`)
	assert.Equal(t, "JSONRPC", request.Transport)
	assert.Equal(t, "1.0", request.ProtocolVersion)
	assert.Equal(t, "m-1", request.MessageID)
	assert.Equal(t, "ctx-1", request.ContextID)
	assert.Equal(t, "task-1", request.TaskID)
	require.NotNil(t, request.InputPartCount)
	assert.Equal(t, 2, *request.InputPartCount)
	require.NotNil(t, request.ReturnImmediately)
	require.NotNil(t, request.HistoryLength)
	assert.Equal(t, 5, *request.HistoryLength)

	response := decodeA2AResponseBlock(`{
		"isError": false, "errorCode": -32601,
		"isStreaming": true, "timeToFirstEventMs": 120, "streamDurationMs": 850,
		"payloadType": "task", "responseTaskId": "task-9", "responseContextId": "ctx-9",
		"taskState": "TASK_STATE_COMPLETED"
	}`)
	require.NotNil(t, response.IsError)
	require.NotNil(t, response.ErrorCode)
	assert.Equal(t, -32601, *response.ErrorCode)
	require.NotNil(t, response.IsStreaming)
	require.NotNil(t, response.TimeToFirstEventMs)
	require.NotNil(t, response.StreamDurationMs)
	assert.Equal(t, "task", response.PayloadType)
	assert.Equal(t, "task-9", response.ResponseTaskID)
	assert.Equal(t, "ctx-9", response.ResponseContextID)
	assert.Equal(t, "TASK_STATE_COMPLETED", response.TaskState)
}

// ─── Robustness of the block decode ─────────────────────────────────────────

// A block that will not parse costs its own section and nothing else. The earlier flat
// map kept the raw string on the event under its own key; a typed envelope has nowhere
// honest to put it, and a consumer reading a named field sees it as absent either way.
func TestDecodeA2ABlocks_UnparseableYieldsAnEmptySectionAndDoesNotBreakTheEvent(t *testing.T) {
	entry := createLogEntryWithMetadata(map[string]string{
		APITypeKey:               string(policy.APIKindAgent),
		APIIDKey:                 "agent-1",
		APINameKey:               "WeatherAgent",
		ResolvedOperationKey:     "SendMessage",
		A2ARequestPropertiesKey:  "{not json",
		A2AResponsePropertiesKey: "{not json",
	})

	event := NewAnalytics(&config.Config{}).prepareAnalyticEvent(entry)
	require.NotNil(t, event)
	envelope, ok := event.Properties[AgentAnalyticsProperty].(*dto.AgentAnalytics)
	require.True(t, ok)
	require.NotNil(t, envelope.A2A)

	assert.Equal(t, "SendMessage", envelope.A2A.Operation,
		"the operation is kernel-stamped and must survive an unreadable policy block")
	assert.Equal(t, dto.A2ARequestAnalytics{}, envelope.A2A.A2ARequestAnalytics)
	assert.Equal(t, dto.A2AResponseAnalytics{}, envelope.A2A.A2AResponseAnalytics)
}

func TestDecodeA2ABlocks_EmptyYieldsNoDimensions(t *testing.T) {
	assert.Equal(t, a2aRequestBlock{}, decodeA2ARequestBlock(""))
	assert.Equal(t, dto.A2AResponseAnalytics{}, decodeA2AResponseBlock(""))
}

// A policy-supplied block cannot set a dimension the kernel owns: the wire types it
// decodes into have no field for one, so a block naming it decodes into nothing.
func TestDecodeA2ABlocks_CannotOverrideKernelStampedDimensions(t *testing.T) {
	event := NewAnalytics(&config.Config{}).prepareAnalyticEvent(
		createLogEntryWithMetadata(map[string]string{
			APITypeKey:           string(policy.APIKindAgent),
			APIIDKey:             "agent-1",
			APINameKey:           "WeatherAgent",
			ResolvedOperationKey: "SendMessage",
			A2ARequestPropertiesKey: `{"operation":"CancelTask","requestType":"agentCard",` +
				`"outcome":"SUCCESS","failureOrigin":"client","messageId":"m-1"}`,
		}))
	require.NotNil(t, event)
	envelope, ok := event.Properties[AgentAnalyticsProperty].(*dto.AgentAnalytics)
	require.True(t, ok)
	require.NotNil(t, envelope.A2A)

	assert.Equal(t, "SendMessage", envelope.A2A.Operation)
	assert.Equal(t, A2ARequestTypeOperation, envelope.A2A.RequestType)
	assert.Equal(t, A2AOutcomeUnknown, envelope.A2A.Outcome)
	assert.Empty(t, envelope.A2A.FailureOrigin)
	assert.Equal(t, "m-1", envelope.A2A.MessageID, "the fields it does own still arrive")
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
	assert.Equal(t, "agentAnalytics", AgentAnalyticsProperty)

	// The transport value arrives as an opaque string out of Envoy dynamic metadata,
	// having been spelled by common/agentproto at the other end of the pipeline. This
	// package compares it (to decide whether a 2xx is itself an outcome), so a
	// divergence would silently turn every HTTP+JSON success into UNKNOWN.
	assert.Equal(t, "HTTP+JSON", a2aTransportHTTPJSON)
}
