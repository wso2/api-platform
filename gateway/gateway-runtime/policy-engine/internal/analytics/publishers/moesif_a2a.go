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

// Translation of the gateway's own Agent analytics envelope into Moesif's first-class
// `a2a` event block, which moesifapi-go v1.2.0 added to EventModel (v1.2.1 added the
// target agent's id and name to it).
//
// Until that version Moesif had no A2A schema, so the envelope travelled as a metadata
// key — free-form JSON, queryable only as custom metadata. The block is now part of the
// event schema proper, with its own field names, its own nesting and its own ingest-time
// validation, so this is where the two vocabularies are reconciled. They differ in three
// ways, all of them deliberate on the Moesif side:
//
//  1. Shape. Moesif nests the request and response dimensions under `request` and
//     `response`; the envelope keeps them flat and renames the two colliding response
//     identifiers. Nesting them back means responseTaskId is simply response.task_id,
//     and the rename disappears.
//  2. Spelling. Moesif's fields are snake_case (the SDK's struct tags carry that), and
//     its operation enum spells the catch-all `Unknown` where this codebase spells it
//     `unknown`.
//  3. Validation. Every field is bounded — enum membership, string length, numeric
//     range — and Moesif validates at ingest. A value outside its bound is dropped here
//     rather than sent: the alternative is the whole event being rejected for one field,
//     which would lose the generic dimensions (status, latency, consumer) too.
//
// Nothing is derived in this file. Every dimension is decided in the analytics package,
// where it is tested against the access-log entry it came from; a publisher that
// computed its own would make the same event mean different things in different sinks.

import (
	"strings"
	"unicode"

	"github.com/moesif/moesifapi-go/models"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
)

// Moesif's closed sets, transcribed from the A2A event schema. Kept as sets rather than
// trusted from upstream because these values cross an Envoy dynamic-metadata boundary as
// opaque strings: the analytics policy bounds them, and this bounds them again at the
// point where exceeding a bound costs the whole event.
var (
	moesifA2AOperations = map[string]struct{}{
		"SendMessage": {}, "SendStreamingMessage": {}, "GetTask": {}, "ListTasks": {},
		"CancelTask": {}, "SubscribeToTask": {}, "CreateTaskPushNotificationConfig": {},
		"GetTaskPushNotificationConfig": {}, "ListTaskPushNotificationConfigs": {},
		"DeleteTaskPushNotificationConfig": {}, "GetExtendedAgentCard": {},
		moesifA2AOperationUnknown: {},
	}

	moesifA2ATransports = map[string]struct{}{
		"JSONRPC": {}, "GRPC": {}, "HTTP+JSON": {},
	}

	moesifA2ARequestTypes = map[string]struct{}{
		"operation": {}, "agentCard": {}, "preflight": {},
	}

	moesifA2APayloadTypes = map[string]struct{}{
		"task": {}, "message": {}, "status_update": {}, "artifact_update": {},
		"task_list": {}, "push_notification_config": {}, "push_notification_config_list": {},
		"agent_card": {}, "empty": {}, "error": {}, "unknown": {},
	}

	moesifA2ATaskStates = map[string]struct{}{
		"TASK_STATE_UNSPECIFIED": {}, "TASK_STATE_SUBMITTED": {}, "TASK_STATE_WORKING": {},
		"TASK_STATE_INPUT_REQUIRED": {}, "TASK_STATE_COMPLETED": {}, "TASK_STATE_CANCELED": {},
		"TASK_STATE_FAILED": {}, "TASK_STATE_REJECTED": {}, "TASK_STATE_AUTH_REQUIRED": {},
	}

	moesifA2AOutcomes = map[string]struct{}{
		"SUCCESS": {}, "FAILURE": {}, "UNKNOWN": {},
	}

	moesifA2AFailureOrigins = map[string]struct{}{
		"CLIENT": {}, "POLICY": {}, "GATEWAY": {}, "UPSTREAM": {}, "UNKNOWN": {},
	}
)

const (
	// moesifA2AOperationUnknown and moesifA2ATransportUnknown are the catch-alls for
	// the schema's two required fields. Moesif spells the operation one `Unknown`,
	// where the gateway's own vocabulary spells it `unknown`; both are the same
	// dimension and the difference is only the enum each side published.
	moesifA2AOperationUnknown = "Unknown"
	moesifA2ATransportUnknown = "UNKNOWN"
	// moesifA2ARequestTypeUnknown is the schema's fourth request type. The gateway
	// never produces it — its classification is total over the three shapes an Agent
	// serves — so it is only ever reached by a value this publisher did not recognise.
	moesifA2ARequestTypeUnknown = "unknown"
	// moesifA2APayloadTypeUnknown is where an unrecognised payload type coerces to,
	// as the schema specifies, rather than being dropped: that a payload arrived and
	// could not be classified is itself the observation.
	moesifA2APayloadTypeUnknown = "unknown"

	// Bounds Moesif applies at ingest. Opaque identifiers are capped at 500 bytes; the
	// two stream timings at one hour and one day respectively; the JSON-RPC/gRPC/HTTP
	// error code to a signed 16-bit range.
	moesifA2AMaxIDLen              = 500
	moesifA2AMaxProtocolVersionLen = 100
	moesifA2AMaxInputPartCount     = 1000
	moesifA2AMaxHistoryLength      = 100000
	moesifA2AMinErrorCode          = -32768
	moesifA2AMaxErrorCode          = 32767
	moesifA2AMaxTimeToFirstEventMs = int64(3600000)
	moesifA2AMaxStreamDurationMs   = int64(86400000)
)

// a2aEventBlock builds the event's `a2a` block from the A2A dimensions the collector
// assembled, or returns nil when the event carried none.
//
// Returning nil is the whole contract for non-A2A traffic: the SDK omits the block, and
// a REST or LLM event ingests exactly as it did before this field existed.
func a2aEventBlock(a2a *dto.A2AAnalytics) *models.A2aModel {
	if a2a == nil {
		return nil
	}

	// Operation and transport are the schema's two required fields, so they are always
	// set — a card fetch and a preflight genuinely have neither, and they carry the
	// catch-alls rather than being sent incomplete. An event rejected at ingest for a
	// missing required field would lose its generic dimensions too. That does not blur
	// the three shapes together: requestType is what tells them apart.
	operation := moesifA2AOperationUnknown
	if _, ok := moesifA2AOperations[a2a.Operation]; ok {
		operation = a2a.Operation
	}
	transport := moesifA2ATransportUnknown
	if _, ok := moesifA2ATransports[a2a.Transport]; ok {
		transport = a2a.Transport
	}
	requestType := moesifA2ARequestTypeUnknown
	if _, ok := moesifA2ARequestTypes[a2a.RequestType]; ok {
		requestType = a2a.RequestType
	}

	return &models.A2aModel{
		Operation:       &operation,
		Transport:       &transport,
		RequestType:     &requestType,
		ProtocolVersion: moesifA2AProtocolVersion(a2a.ProtocolVersion),
		Request:         moesifA2ARequest(a2a.A2ARequestAnalytics),
		Response:        moesifA2AResponse(a2a.A2AResponseAnalytics),
		Terminal:        a2a.Terminal,
		Outcome:         moesifEnum(a2a.Outcome, moesifA2AOutcomes, "UNKNOWN"),
		FailureOrigin:   moesifEnum(a2a.FailureOrigin, moesifA2AFailureOrigins, "UNKNOWN"),
	}
}

// setA2AAgentIdentity fills the block's agent id and name from the event's API identity.
//
// They are not A2A dimensions — an Agent's id and name are the event's API id and name,
// which the collector carries on every event regardless of kind — so they are not
// duplicated onto dto.A2AAnalytics. Moesif carries them inside the block anyway, so an
// A2A dashboard can group by the callee agent without joining against metadata. Both are
// bounded like any other opaque identifier: dropped, never truncated.
func setA2AAgentIdentity(block *models.A2aModel, api *dto.ExtendedAPI) {
	if block == nil || api == nil {
		return
	}
	block.AgentId = moesifA2AOpaqueID(api.APIID)
	block.AgentName = moesifA2AOpaqueID(api.APIName)
}

// moesifA2ARequest maps the caller-supplied side. Nil when the caller supplied none of
// it, so a GetTask with nothing but a path parameter does not publish an empty object.
func moesifA2ARequest(req dto.A2ARequestAnalytics) *models.A2aRequestModel {
	out := &models.A2aRequestModel{
		MessageId:         moesifA2AOpaqueID(req.MessageID),
		TaskId:            moesifA2AOpaqueID(req.TaskID),
		ContextId:         moesifA2AOpaqueID(req.ContextID),
		InputPartCount:    moesifBoundedInt(req.InputPartCount, 0, moesifA2AMaxInputPartCount),
		ReturnImmediately: req.ReturnImmediately,
		HistoryLength:     moesifBoundedInt(req.HistoryLength, 0, moesifA2AMaxHistoryLength),
	}
	if out.MessageId == nil && out.TaskId == nil && out.ContextId == nil &&
		out.InputPartCount == nil && out.ReturnImmediately == nil && out.HistoryLength == nil {
		return nil
	}
	return out
}

// moesifA2AResponse maps what the gateway observed coming back. The two identifiers
// lose the `response` prefix they carry in the flat model: the nesting supplies the
// distinction the prefix existed to make.
func moesifA2AResponse(resp dto.A2AResponseAnalytics) *models.A2aResponseModel {
	out := &models.A2aResponseModel{
		IsError:            resp.IsError,
		ErrorCode:          moesifBoundedInt(resp.ErrorCode, moesifA2AMinErrorCode, moesifA2AMaxErrorCode),
		IsStreaming:        resp.IsStreaming,
		TimeToFirstEventMs: moesifBoundedInt64(resp.TimeToFirstEventMs, 0, moesifA2AMaxTimeToFirstEventMs),
		StreamDurationMs:   moesifBoundedInt64(resp.StreamDurationMs, 0, moesifA2AMaxStreamDurationMs),
		TaskId:             moesifA2AOpaqueID(resp.ResponseTaskID),
		ContextId:          moesifA2AOpaqueID(resp.ResponseContextID),
		TaskState:          moesifEnum(resp.TaskState, moesifA2ATaskStates, ""),
	}
	// Payload type is matched case-insensitively and coerced rather than dropped, per
	// the schema: it is derived from an agent-authored document, and an unclassifiable
	// payload is a fact a dashboard should be able to count.
	if resp.PayloadType != "" {
		payloadType := moesifA2APayloadTypeUnknown
		if lowered := strings.ToLower(resp.PayloadType); moesifA2AIsPayloadType(lowered) {
			payloadType = lowered
		}
		out.PayloadType = &payloadType
	}
	if out.IsError == nil && out.ErrorCode == nil && out.IsStreaming == nil &&
		out.TimeToFirstEventMs == nil && out.StreamDurationMs == nil && out.PayloadType == nil &&
		out.TaskId == nil && out.ContextId == nil && out.TaskState == nil {
		return nil
	}
	return out
}

func moesifA2AIsPayloadType(value string) bool {
	_, ok := moesifA2APayloadTypes[value]
	return ok
}

// moesifEnum returns value when it is a member of allowed, fallback when it is not, and
// nil when it is empty.
//
// An empty value means the gateway never determined the dimension, which the schema
// expresses by the field's absence. A non-empty value outside the set means the two
// vocabularies have drifted, which is worth reporting as the enum's own catch-all where
// it has one — and dropping where it does not, since inventing a member is worse.
func moesifEnum(value string, allowed map[string]struct{}, fallback string) *string {
	if value == "" {
		return nil
	}
	if _, ok := allowed[value]; ok {
		return &value
	}
	if fallback == "" {
		return nil
	}
	return &fallback
}

// moesifA2AOpaqueID passes an identifier through when it is within Moesif's bound and
// free of control characters, and drops it otherwise.
//
// Dropped, never truncated: a truncated identifier is not a shorter identifier, it is a
// different one, and correlating on it would silently group unrelated requests. This is
// the same treatment the analytics policy gives an over-long observed value.
func moesifA2AOpaqueID(value string) *string {
	if value == "" || len(value) > moesifA2AMaxIDLen {
		return nil
	}
	if strings.ContainsFunc(value, func(r rune) bool { return unicode.IsControl(r) }) {
		return nil
	}
	return &value
}

// moesifA2AProtocolVersion applies the schema's own charset and length rule to the
// protocol version. The value is fixed at route ingest and drawn from the version
// registry, so this rejects nothing in practice; it is here because the value reaches
// this publisher as an opaque string out of Envoy dynamic metadata, and a field whose
// contents are only guaranteed by a convention elsewhere is worth checking where it is
// finally serialized.
func moesifA2AProtocolVersion(value string) *string {
	if value == "" || len(value) > moesifA2AMaxProtocolVersionLen {
		return nil
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.', r == '/', r == ':', r == '@', r == ' ':
		default:
			return nil
		}
	}
	return &value
}

// moesifBoundedInt and moesifBoundedInt64 pass a measure through only when it falls
// inside the range Moesif accepts, and drop it otherwise.
//
// Dropped rather than clamped, for the same reason an identifier is dropped rather than
// truncated: a clamped measure is indistinguishable from a real one at the boundary, so
// a stream that genuinely ran for a day and one whose timing was computed wrongly would
// report the same number.
func moesifBoundedInt(value *int, minimum, maximum int) *int {
	if value == nil || *value < minimum || *value > maximum {
		return nil
	}
	return value
}

func moesifBoundedInt64(value *int64, minimum, maximum int64) *int64 {
	if value == nil || *value < minimum || *value > maximum {
		return nil
	}
	return value
}
