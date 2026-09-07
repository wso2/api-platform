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

// The published Agent analytics model.
//
// This is the external contract: what a downstream consumer — Moesif, the analytics
// pipeline, AI Workspace — reads off an Agent event. It is a typed structure rather
// than the collector's scratch map because it is a contract, and a map is only ever a
// contract by convention: a renamed key or a value that quietly changes type is
// invisible until a dashboard goes blank.
//
// Two shaping rules, both deliberate:
//
// The envelope is keyed by domain (agentAnalytics.a2a), not flat. An Agent may later
// carry analytics for something other than A2A, and a sibling section can be added
// without its fields having to be distinguishable from A2A's by name alone.
//
// The request and response objects are flat inside that. Nesting already records which
// direction produced a value, so a response identifier is response.taskId — it does not
// also need a name that says "observed". The two are kept apart rather than merged so a
// disagreement between what a caller asked for and what an agent answered with stays
// diagnosable; a consumer wanting one value derives
// effectiveTaskId = response.taskId ?? request.taskId itself, which is why no such
// field is published.
//
// There is deliberately no schema-version property. Every shape-dependent field is
// optional and omitted when the request or response did not carry it, so a consumer
// reads what is present rather than branching on a version; and a version number that
// nothing enforces is a claim, not a guarantee.
//
// Generic facts stay outside this envelope: API and Agent identity, organization,
// project, environment, consumer and credential identity, correlation id, HTTP status,
// sizes and the common latency fields are all published as they are for every other API
// kind. Only what is specific to A2A lives here.

// AgentAnalytics is the published analytics envelope for one Agent event.
type AgentAnalytics struct {
	// A2A carries the A2A protocol's own dimensions. A pointer so an Agent event
	// that somehow produced none publishes an empty envelope rather than an
	// object full of zero values.
	A2A *A2AAnalytics `json:"a2a,omitempty"`
}

// A2AAnalytics is the A2A section of the envelope.
//
// The four scalars above the two objects are the dimensions a dashboard groups by, and
// every one of them is drawn from a closed set: the operation from the protocol
// version's own operation table (or `unknown`), the transport from a two-valued enum,
// the version from the registry, the outcome and failure origin from the vocabularies
// in this package. The identifiers that are not bounded live inside Request and
// Response, where they are event and trace data and never a metric label.
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

	// Request is what the caller asked for; Response is what the gateway observed
	// coming back. Both are pointers and both are omitted entirely when nothing
	// was extracted, so an absent object means "not applicable to this operation"
	// rather than "every field was empty".
	Request  *A2ARequestAnalytics  `json:"request,omitempty"`
	Response *A2AResponseAnalytics `json:"response,omitempty"`

	// Outcome is SUCCESS, FAILURE or UNKNOWN, derived from the A2A result rather
	// than the HTTP status. FailureOrigin names the answerable layer and is
	// present only for a failure.
	Outcome       string `json:"outcome,omitempty"`
	FailureOrigin string `json:"failureOrigin,omitempty"`
}

// A2ARequestAnalytics is what the caller asked for.
//
// The three identifiers are the caller's own and opaque; the three summaries are
// content-free measures of the request's shape. None of them belongs on a metric label
// — the identifiers because they are unbounded, the summaries because they are
// measures, which is what a histogram takes rather than what a dimension names.
//
// The summaries are pointers because each of their zero values is a real answer: a
// message can carry no parts, a history length of zero asks for no history at all, and
// returnImmediately false is the protocol's own default rather than an absent field.
type A2ARequestAnalytics struct {
	MessageID string `json:"messageId,omitempty"`
	TaskID    string `json:"taskId,omitempty"`
	ContextID string `json:"contextId,omitempty"`

	InputPartCount    *int  `json:"inputPartCount,omitempty"`
	ReturnImmediately *bool `json:"returnImmediately,omitempty"`
	HistoryLength     *int  `json:"historyLength,omitempty"`
}

// IsEmpty reports whether nothing was extracted, so the object can be omitted rather
// than published as `{}`.
func (r *A2ARequestAnalytics) IsEmpty() bool {
	if r == nil {
		return true
	}
	return r.MessageID == "" && r.TaskID == "" && r.ContextID == "" &&
		r.InputPartCount == nil && r.ReturnImmediately == nil && r.HistoryLength == nil
}

// A2AResponseAnalytics is what the gateway observed coming back.
//
// IsError is what makes an A2A success rate correct rather than plausible: a JSON-RPC
// error travels inside a 200, so a rate computed from the HTTP status counts failed
// invocations as successes. It is omitted — never defaulted to false — when no response
// body could be read, because false is a positive claim of success and an undetermined
// outcome is not one.
//
// The error *message* is deliberately absent throughout: it is agent-authored free text
// of unbounded length and unknown sensitivity. The numeric code is what a dashboard
// groups by after mapping it into a bounded category.
//
// The two timings are measured by the analytics policy from the gateway's own clock,
// because the access-log timepoints the generic latency fields come from cannot express
// them: a streaming response's last upstream byte arrives when the stream ends, so its
// backend latency is its whole duration and says nothing about when the first event
// reached the client.
type A2AResponseAnalytics struct {
	IsError   *bool `json:"isError,omitempty"`
	ErrorCode *int  `json:"errorCode,omitempty"`

	IsStreaming        *bool  `json:"isStreaming,omitempty"`
	TimeToFirstEventMs *int64 `json:"timeToFirstEventMs,omitempty"`
	StreamDurationMs   *int64 `json:"streamDurationMs,omitempty"`

	// PayloadType and TaskState are bounded enums and safe to group by. TaskID and
	// ContextID are the agent's own identifiers — which is the point of publishing
	// them: a caller that sends a bare message gets back a task id it never
	// supplied, and without these that invocation correlates to nothing.
	PayloadType string `json:"payloadType,omitempty"`
	TaskID      string `json:"taskId,omitempty"`
	ContextID   string `json:"contextId,omitempty"`
	TaskState   string `json:"taskState,omitempty"`
}

// IsEmpty reports whether nothing was observed, so the object can be omitted rather
// than published as `{}`.
func (r *A2AResponseAnalytics) IsEmpty() bool {
	if r == nil {
		return true
	}
	return r.IsError == nil && r.ErrorCode == nil &&
		r.IsStreaming == nil && r.TimeToFirstEventMs == nil && r.StreamDurationMs == nil &&
		r.PayloadType == "" && r.TaskID == "" && r.ContextID == "" && r.TaskState == ""
}
