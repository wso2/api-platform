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

package publishers

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
)

func ptr[T any](v T) *T { return &v }

// Non-A2A traffic must ingest exactly as it did before the schema gained this field.
func TestA2AEventBlock_NilForNoDimensions(t *testing.T) {
	assert.Nil(t, a2aEventBlock(nil))
}

// The gateway's operation vocabulary is Moesif's, with one exception: the catch-all.
// Both name the same dimension and the difference is only how each enum spells it, so
// the translation happens here rather than leaving two spellings in the data.
func TestA2AEventBlock_OperationCatchAllIsSpelledMoesifsWay(t *testing.T) {
	block := a2aEventBlock(&dto.A2AAnalytics{RequestType: "operation", Operation: "unknown"})
	require.NotNil(t, block.Operation)
	assert.Equal(t, "Unknown", *block.Operation)
}

func TestA2AEventBlock_CanonicalOperationsPassThrough(t *testing.T) {
	for _, operation := range []string{
		"SendMessage", "SendStreamingMessage", "GetTask", "ListTasks", "CancelTask",
		"SubscribeToTask", "CreateTaskPushNotificationConfig",
		"GetTaskPushNotificationConfig", "ListTaskPushNotificationConfigs",
		"DeleteTaskPushNotificationConfig", "GetExtendedAgentCard",
	} {
		t.Run(operation, func(t *testing.T) {
			block := a2aEventBlock(&dto.A2AAnalytics{Operation: operation})
			require.NotNil(t, block.Operation)
			assert.Equal(t, operation, *block.Operation)
		})
	}
}

// Operation and transport are required by the schema. An event that determined neither
// — a card fetch, a preflight — carries the catch-alls rather than omitting the fields,
// because an event rejected at ingest for a missing required field loses its generic
// dimensions (status, latency, consumer) along with its A2A ones.
func TestA2AEventBlock_RequiredFieldsAlwaysPresent(t *testing.T) {
	block := a2aEventBlock(&dto.A2AAnalytics{RequestType: "preflight"})

	require.NotNil(t, block.Operation)
	assert.Equal(t, "Unknown", *block.Operation)
	require.NotNil(t, block.Transport)
	assert.Equal(t, "UNKNOWN", *block.Transport)
	require.NotNil(t, block.RequestType)
	assert.Equal(t, "preflight", *block.RequestType)
}

// Both bindings the gateway actually serves, plus the one the schema defines that it
// does not yet, so adding a gRPC binding later does not silently report as UNKNOWN.
func TestA2AEventBlock_TransportsPassThrough(t *testing.T) {
	for _, transport := range []string{"JSONRPC", "HTTP+JSON", "GRPC"} {
		t.Run(transport, func(t *testing.T) {
			block := a2aEventBlock(&dto.A2AAnalytics{Transport: transport})
			require.NotNil(t, block.Transport)
			assert.Equal(t, transport, *block.Transport)
		})
	}
}

// A value outside an enum means the two vocabularies have drifted. Where the enum has
// a catch-all that is what it reports; a dimension with no honest catch-all is dropped
// rather than assigned an invented member.
func TestA2AEventBlock_UnrecognisedEnumValuesCoerceOrDrop(t *testing.T) {
	block := a2aEventBlock(&dto.A2AAnalytics{
		Operation:     "TeleportTask",
		Transport:     "CARRIER_PIGEON",
		RequestType:   "somethingElse",
		Outcome:       "probably fine",
		FailureOrigin: "the weather",
		A2AResponseAnalytics: dto.A2AResponseAnalytics{
			TaskState: "TASK_STATE_ASCENDED",
		},
	})

	assert.Equal(t, "Unknown", *block.Operation)
	assert.Equal(t, "UNKNOWN", *block.Transport)
	assert.Equal(t, "unknown", *block.RequestType)
	assert.Equal(t, "UNKNOWN", *block.Outcome)
	assert.Equal(t, "UNKNOWN", *block.FailureOrigin)
	assert.Nil(t, block.Response, "an unrecognised task state is the only field it carried")
}

// The schema matches payload types case-insensitively and coerces what it cannot
// classify, rather than dropping it: that a payload arrived and could not be classified
// is itself something a dashboard should be able to count.
func TestA2AEventBlock_PayloadTypeIsCaseInsensitiveAndCoerces(t *testing.T) {
	cases := map[string]string{
		"task":            "task",
		"STATUS_UPDATE":   "status_update",
		"Artifact_Update": "artifact_update",
		"whatever":        "unknown",
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			block := a2aEventBlock(&dto.A2AAnalytics{
				A2AResponseAnalytics: dto.A2AResponseAnalytics{PayloadType: input},
			})
			require.NotNil(t, block.Response)
			require.NotNil(t, block.Response.PayloadType)
			assert.Equal(t, want, *block.Response.PayloadType)
		})
	}
}

// Terminality is derived in the analytics package, where it is tested against the task
// state it came from. This publisher carries it, and carries its absence: false is a
// claim the task is still running, which an event that observed no state never made.
func TestA2AEventBlock_TerminalityIsCarriedIncludingItsAbsence(t *testing.T) {
	assert.Nil(t, a2aEventBlock(&dto.A2AAnalytics{}).Terminal)

	block := a2aEventBlock(&dto.A2AAnalytics{Terminal: ptr(false)})
	require.NotNil(t, block.Terminal)
	assert.False(t, *block.Terminal)
}

// The sub-blocks are omitted rather than sent empty, so a GetTask carrying nothing but
// a path parameter does not publish two empty objects on every event.
func TestA2AEventBlock_EmptySubBlocksAreOmitted(t *testing.T) {
	block := a2aEventBlock(&dto.A2AAnalytics{RequestType: "operation", Operation: "GetTask"})
	assert.Nil(t, block.Request)
	assert.Nil(t, block.Response)
}

// A zero is a real answer in all three of these — a message with no parts, a request
// for no history, and the protocol's own default for returnImmediately — so none of
// them may be mistaken for an absent field.
func TestA2AEventBlock_ZeroValuedRequestMeasuresSurvive(t *testing.T) {
	block := a2aEventBlock(&dto.A2AAnalytics{
		A2ARequestAnalytics: dto.A2ARequestAnalytics{
			InputPartCount:    ptr(0),
			HistoryLength:     ptr(0),
			ReturnImmediately: ptr(false),
		},
	})

	require.NotNil(t, block.Request)
	require.NotNil(t, block.Request.InputPartCount)
	assert.Equal(t, 0, *block.Request.InputPartCount)
	require.NotNil(t, block.Request.HistoryLength)
	assert.Equal(t, 0, *block.Request.HistoryLength)
	require.NotNil(t, block.Request.ReturnImmediately)
	assert.False(t, *block.Request.ReturnImmediately)
}

// Identifiers are dropped, never truncated, when they exceed the schema's ceiling: a
// truncated identifier is not a shorter identifier, it is a different one, and
// correlating on it would silently group unrelated requests.
func TestA2AEventBlock_OverLongIdentifiersAreDroppedNotTruncated(t *testing.T) {
	atCeiling := strings.Repeat("a", moesifA2AMaxIDLen)
	overCeiling := strings.Repeat("b", moesifA2AMaxIDLen+1)

	block := a2aEventBlock(&dto.A2AAnalytics{
		A2ARequestAnalytics: dto.A2ARequestAnalytics{MessageID: atCeiling, TaskID: overCeiling},
	})

	require.NotNil(t, block.Request)
	require.NotNil(t, block.Request.MessageId)
	assert.Equal(t, atCeiling, *block.Request.MessageId, "a value at the ceiling still travels")
	assert.Nil(t, block.Request.TaskId)
}

// An identifier is opaque but not arbitrary: a control character in one would corrupt
// the serialized event for every consumer downstream of Moesif, not just this field.
func TestA2AEventBlock_IdentifiersWithControlCharactersAreDropped(t *testing.T) {
	block := a2aEventBlock(&dto.A2AAnalytics{
		A2ARequestAnalytics: dto.A2ARequestAnalytics{
			MessageID: "msg\x00-1",
			TaskID:    "task\n-1",
			ContextID: "ctx-1",
		},
	})

	require.NotNil(t, block.Request)
	assert.Nil(t, block.Request.MessageId)
	assert.Nil(t, block.Request.TaskId)
	require.NotNil(t, block.Request.ContextId)
	assert.Equal(t, "ctx-1", *block.Request.ContextId)
}

// A measure outside the range Moesif accepts is dropped rather than clamped, for the
// same reason an identifier is dropped rather than truncated: a clamped measure is
// indistinguishable from a real one at the boundary.
func TestA2AEventBlock_OutOfRangeMeasuresAreDroppedNotClamped(t *testing.T) {
	block := a2aEventBlock(&dto.A2AAnalytics{
		A2ARequestAnalytics: dto.A2ARequestAnalytics{
			MessageID:      "msg-1", // keeps the request sub-block alive
			InputPartCount: ptr(moesifA2AMaxInputPartCount + 1),
			HistoryLength:  ptr(-1),
		},
		A2AResponseAnalytics: dto.A2AResponseAnalytics{
			ErrorCode:          ptr(moesifA2AMinErrorCode - 1),
			TimeToFirstEventMs: ptr(moesifA2AMaxTimeToFirstEventMs + 1),
			StreamDurationMs:   ptr(int64(-1)),
			PayloadType:        "task", // keeps the response sub-block alive
		},
	})

	require.NotNil(t, block.Request)
	assert.Nil(t, block.Request.InputPartCount)
	assert.Nil(t, block.Request.HistoryLength)
	require.NotNil(t, block.Response)
	assert.Nil(t, block.Response.ErrorCode)
	assert.Nil(t, block.Response.TimeToFirstEventMs)
	assert.Nil(t, block.Response.StreamDurationMs)
}

// The codes A2A actually produces must all survive the range check — a JSON-RPC error,
// a gRPC status and an HTTP status are the three shapes this field carries.
func TestA2AEventBlock_RealErrorCodesSurvive(t *testing.T) {
	for _, code := range []int{-32700, -32603, -32601, 0, 7, 16, 404, 429, 503} {
		block := a2aEventBlock(&dto.A2AAnalytics{
			A2AResponseAnalytics: dto.A2AResponseAnalytics{ErrorCode: ptr(code)},
		})
		require.NotNil(t, block.Response)
		require.NotNil(t, block.Response.ErrorCode, "code %d must survive", code)
		assert.Equal(t, code, *block.Response.ErrorCode)
	}
}

// The protocol version is bounded upstream by the version registry, so this rejects
// nothing in practice. It is checked because the value reaches this publisher as an
// opaque string out of Envoy dynamic metadata.
func TestA2AEventBlock_ProtocolVersionCharsetAndLength(t *testing.T) {
	assert.Equal(t, "1.0", *a2aEventBlock(&dto.A2AAnalytics{ProtocolVersion: "1.0"}).ProtocolVersion)
	assert.Nil(t, a2aEventBlock(&dto.A2AAnalytics{ProtocolVersion: "1.0\n"}).ProtocolVersion)
	assert.Nil(t, a2aEventBlock(&dto.A2AAnalytics{
		ProtocolVersion: strings.Repeat("1", moesifA2AMaxProtocolVersionLen+1),
	}).ProtocolVersion)
}

// isError is what makes an A2A success rate correct rather than plausible — a JSON-RPC
// error rides a 200 — and its absence is a distinct state from false, meaning no
// response body could be read. Both must survive the mapping.
func TestA2AEventBlock_IsErrorCarriesBothValuesAndItsAbsence(t *testing.T) {
	assert.Nil(t, a2aEventBlock(&dto.A2AAnalytics{}).Response)

	for _, isError := range []bool{true, false} {
		block := a2aEventBlock(&dto.A2AAnalytics{
			A2AResponseAnalytics: dto.A2AResponseAnalytics{IsError: &isError},
		})
		require.NotNil(t, block.Response)
		require.NotNil(t, block.Response.IsError)
		assert.Equal(t, isError, *block.Response.IsError)
	}
}

// The failure vocabulary reaches Moesif intact: without it a dashboard cannot tell an
// agent that is erroring from a gateway that is rejecting, and both look like the
// agent's fault.
func TestA2AEventBlock_OutcomeAndFailureOriginPassThrough(t *testing.T) {
	for _, origin := range []string{"CLIENT", "POLICY", "GATEWAY", "UPSTREAM"} {
		block := a2aEventBlock(&dto.A2AAnalytics{Outcome: "FAILURE", FailureOrigin: origin})
		require.NotNil(t, block.Outcome)
		assert.Equal(t, "FAILURE", *block.Outcome)
		require.NotNil(t, block.FailureOrigin)
		assert.Equal(t, origin, *block.FailureOrigin)
	}

	// A success has no answerable layer, and the field is absent rather than UNKNOWN.
	block := a2aEventBlock(&dto.A2AAnalytics{Outcome: "SUCCESS"})
	assert.Equal(t, "SUCCESS", *block.Outcome)
	assert.Nil(t, block.FailureOrigin)
}

// Every task state the analytics policy can report must survive, or the states it
// validated upstream would be silently dropped here instead.
func TestA2AEventBlock_TaskStatesPassThrough(t *testing.T) {
	for _, state := range []string{
		"TASK_STATE_UNSPECIFIED", "TASK_STATE_SUBMITTED", "TASK_STATE_WORKING",
		"TASK_STATE_INPUT_REQUIRED", "TASK_STATE_COMPLETED", "TASK_STATE_CANCELED",
		"TASK_STATE_FAILED", "TASK_STATE_REJECTED", "TASK_STATE_AUTH_REQUIRED",
	} {
		t.Run(state, func(t *testing.T) {
			block := a2aEventBlock(&dto.A2AAnalytics{
				A2AResponseAnalytics: dto.A2AResponseAnalytics{TaskState: state},
			})
			require.NotNil(t, block.Response)
			require.NotNil(t, block.Response.TaskState)
			assert.Equal(t, state, *block.Response.TaskState)
		})
	}
}

// The streaming timings are the two dimensions the generic latency fields cannot
// express, so a streaming operation that reports them must not lose them here.
func TestA2AEventBlock_StreamingTimingsAreCarried(t *testing.T) {
	block := a2aEventBlock(&dto.A2AAnalytics{
		Operation: "SendStreamingMessage",
		A2AResponseAnalytics: dto.A2AResponseAnalytics{
			IsStreaming:        ptr(true),
			TimeToFirstEventMs: ptr(int64(120)),
			StreamDurationMs:   ptr(int64(8500)),
		},
	})

	require.NotNil(t, block.Response)
	require.NotNil(t, block.Response.IsStreaming)
	assert.True(t, *block.Response.IsStreaming)
	assert.EqualValues(t, 120, *block.Response.TimeToFirstEventMs)
	assert.EqualValues(t, 8500, *block.Response.StreamDurationMs)
}

// The agent id and name come from the event's API identity, not the A2A dimensions:
// for an Agent event the API is the callee agent.
func TestSetA2AAgentIdentity_FromEventAPI(t *testing.T) {
	block := a2aEventBlock(&dto.A2AAnalytics{RequestType: "operation", Operation: "SendMessage"})
	setA2AAgentIdentity(block, &dto.ExtendedAPI{API: dto.API{APIID: "agent-42", APIName: "Weather Agent"}})

	require.NotNil(t, block.AgentId)
	require.NotNil(t, block.AgentName)
	assert.Equal(t, "agent-42", *block.AgentId)
	assert.Equal(t, "Weather Agent", *block.AgentName)
}

// An absent or out-of-bound identity is dropped rather than sent, like any other opaque
// identifier, so one bad field cannot get the whole event rejected at ingest.
func TestSetA2AAgentIdentity_DropsAbsentAndOutOfBoundValues(t *testing.T) {
	block := a2aEventBlock(&dto.A2AAnalytics{RequestType: "operation"})
	setA2AAgentIdentity(block, &dto.ExtendedAPI{API: dto.API{
		APIID:   strings.Repeat("a", moesifA2AMaxIDLen+1),
		APIName: "bad\nname",
	}})
	assert.Nil(t, block.AgentId)
	assert.Nil(t, block.AgentName)

	block = a2aEventBlock(&dto.A2AAnalytics{RequestType: "operation"})
	setA2AAgentIdentity(block, &dto.ExtendedAPI{})
	assert.Nil(t, block.AgentId)
	assert.Nil(t, block.AgentName)
}

// Neither a missing block nor a missing API may panic the publisher.
func TestSetA2AAgentIdentity_NilSafe(t *testing.T) {
	setA2AAgentIdentity(nil, &dto.ExtendedAPI{})
	block := a2aEventBlock(&dto.A2AAnalytics{})
	setA2AAgentIdentity(block, nil)
	assert.Nil(t, block.AgentId)
}
