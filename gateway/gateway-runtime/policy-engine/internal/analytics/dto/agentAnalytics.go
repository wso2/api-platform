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

package dto

// The published Agent analytics model — the external contract a downstream consumer
// (Moesif, the analytics pipeline, AI Workspace) reads off an Agent event.
//
// The envelope is keyed by domain (agentAnalytics.a2a) so a later Agent analytics
// domain can be added as a sibling of `a2a`. Inside that section every dimension is
// one flat level: there is no `request` or `response` object. Only the two response
// identifiers that would collide with a request field are renamed, to responseTaskId
// and responseContextId; a consumer derives effectiveTaskId = responseTaskId ?? taskId
// itself, so no effective-id field is published.
//
// The Go type keeps the two directions as embedded field groups, which JSON flattens.
// That is a decode concern: each group is exactly what one side of the pipeline
// serializes, so a policy-supplied block cannot reach a field the kernel owns.
//
// Every shape-dependent field is optional and omitted when it was not carried; there is
// deliberately no schema-version property. Generic facts — API and Agent identity,
// organization, project, environment, consumer and credential identity, correlation id,
// HTTP status, sizes, latencies — stay outside this envelope.

// AgentAnalytics is the published analytics envelope for one Agent event.
type AgentAnalytics struct {
	// A2A is a pointer so an Agent event that produced no A2A dimensions publishes
	// an empty envelope rather than an object full of zero values.
	A2A *A2AAnalytics `json:"a2a,omitempty"`
}

// A2AAnalytics is the A2A section of the envelope, published as one flat object.
//
// The dimensions safe to group by are drawn from closed sets: the operation from the
// protocol version's operation table (or `unknown`), the transport from a two-valued
// enum, the version from the registry, the outcome and failure origin from the
// vocabularies in this package, and payloadType/taskState from bounded enums. The
// identifiers are unbounded and are event and trace data, never metric labels.
type A2AAnalytics struct {
	// RequestType separates the three shapes of traffic an Agent serves on one
	// context: `operation`, `agentCard`, and `preflight`. A card fetch and a
	// preflight carry this and nothing else, so neither can be mistaken for an
	// invocation by a consumer that simply counts events.
	RequestType string `json:"requestType,omitempty"`

	// Operation is the canonical operation the request resolved to, or `unknown`
	// for an attempted invocation whose operation could not be determined.
	Operation string `json:"operation,omitempty"`

	// Transport and ProtocolVersion are the binding the request arrived over and
	// the protocol version the route exposes — not the version the caller stated,
	// which is unbounded and attacker-chosen.
	Transport       string `json:"transport,omitempty"`
	ProtocolVersion string `json:"protocolVersion,omitempty"`

	A2ARequestAnalytics
	A2AResponseAnalytics

	// Outcome is SUCCESS, FAILURE or UNKNOWN, derived from the A2A result rather
	// than the HTTP status. FailureOrigin names the answerable layer and is
	// present only for a failure.
	Outcome       string `json:"outcome,omitempty"`
	FailureOrigin string `json:"failureOrigin,omitempty"`
}

// A2ARequestAnalytics is what the caller asked for: three opaque caller-owned
// identifiers and three content-free measures of the request's shape.
//
// The measures are pointers because each zero value is a real answer — a message can
// carry no parts, a history length of zero asks for no history, and returnImmediately
// false is the protocol's own default rather than an absent field.
type A2ARequestAnalytics struct {
	MessageID string `json:"messageId,omitempty"`
	TaskID    string `json:"taskId,omitempty"`
	ContextID string `json:"contextId,omitempty"`

	InputPartCount    *int  `json:"inputPartCount,omitempty"`
	ReturnImmediately *bool `json:"returnImmediately,omitempty"`
	HistoryLength     *int  `json:"historyLength,omitempty"`
}

// A2AResponseAnalytics is what the gateway observed coming back.
//
// IsError is what makes an A2A success rate correct rather than plausible: a JSON-RPC
// error travels inside a 200, so a rate computed from the HTTP status counts failed
// invocations as successes. It is omitted — never defaulted to false — when no response
// body could be read, because false is a positive claim of success. The error *message*
// is deliberately absent throughout: it is agent-authored free text of unbounded length
// and unknown sensitivity, and the numeric code is what a dashboard groups by.
//
// The two timings are measured by the analytics policy from the gateway's own clock,
// because the access-log timepoints the generic latency fields come from cannot express
// them: a streaming response's last upstream byte arrives when the stream ends.
//
// ResponseTaskID and ResponseContextID carry the `response` prefix because a request
// taskId and contextId share this flat object. That is the point of publishing them: a
// caller that sends a bare message gets back a task id it never supplied, and without
// these that invocation correlates to nothing.
type A2AResponseAnalytics struct {
	IsError   *bool `json:"isError,omitempty"`
	ErrorCode *int  `json:"errorCode,omitempty"`

	IsStreaming        *bool  `json:"isStreaming,omitempty"`
	TimeToFirstEventMs *int64 `json:"timeToFirstEventMs,omitempty"`
	StreamDurationMs   *int64 `json:"streamDurationMs,omitempty"`

	PayloadType       string `json:"payloadType,omitempty"`
	ResponseTaskID    string `json:"responseTaskId,omitempty"`
	ResponseContextID string `json:"responseContextId,omitempty"`
	TaskState         string `json:"taskState,omitempty"`
}
