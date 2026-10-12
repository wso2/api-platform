/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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

package policyv1alpha2

// DropHeaderAction controls which headers appear in the analytics event.
type DropHeaderAction struct {
	Action  string   // "allow" (allowlist) or "deny" (denylist)
	Headers []string // Header name list to drop or allow
}

// ─── Short-circuit ────────────────────────────────────────────────────────────

// ImmediateResponse terminates the policy chain and returns a response to the
// downstream client immediately. Returning nil from a method that returns a sealed
// action interface means "no action".
type ImmediateResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
	// IsFault states whether this response is a failure the fault flow should act on.
	//
	// One half of the gate: a response reaches the fault flow when EITHER its status is 400
	// or above OR a policy set this field. So a 4xx rejection need not declare.
	//
	// Set it when the response is a failure the status does not reveal — a 200 carrying the
	// failure in the body, which only the policy knows about.
	//
	// Leaving it false does NOT hold an error status out of the flow. A configured 404 or an
	// auth challenge still reaches the chain; narrow that with an executionCondition on the
	// fault entry.
	//
	// Setting Fault does not imply this: Fault says WHAT failed, not whether anything did.
	IsFault bool
	// Fault describes the failure for a renderer: code, class, direction, summary, detail.
	// Optional, and independent of IsFault — setting it does not make the response a fault.
	Fault                 *FaultDetails
	AnalyticsMetadata     map[string]any            // Custom analytics metadata
	DynamicMetadata       map[string]map[string]any // Dynamic metadata by namespace
	AnalyticsHeaderFilter DropHeaderAction          // Headers to exclude from analytics
}

// ─── Header phase actions (sealed oneof) ─────────────────────────────────────
//
// RequestHeaderAction and ResponseHeaderAction are sealed interfaces.
// Each has two concrete implementations — one for header mutations, one for
// short-circuiting. The kernel uses a type switch to dispatch.

// RequestHeaderAction is a sealed oneof returned by OnRequestHeaders.
// Implement either UpstreamRequestHeaderModifications or return ImmediateResponse.
type RequestHeaderAction interface {
	isRequestHeaderAction()
}

// UpstreamRequestHeaderModifications continues the request to upstream with the
// specified header and routing modifications. Returned when no short-circuit is needed.
type UpstreamRequestHeaderModifications struct {
	HeadersToSet    map[string]string   // overwrite header (last write wins)
	HeadersToAppend map[string][]string // append value(s), preserving any existing values
	HeadersToRemove []string            // remove by name (case-insensitive)

	// Routing mutations — applied before the request is forwarded to upstream.
	// These are valid at the header phase because routing decisions do not require
	// the request body to be available.
	UpstreamName            *string             // route to a named upstream definition (nil = no change)
	UpstreamSlot            *UpstreamSlot       // route to the API's main or sandbox upstream slot instead of the slot implied by the current vhost (nil = no change); requesting a slot the API doesn't have configured is a no-op (logged, not an error)
	Path                    *string             // rewrite the request path (nil = no change)
	Host                    *string             // rewrite the :authority header (nil = no change)
	Method                  *string             // rewrite the request method (nil = no change)
	QueryParametersToAdd    map[string][]string // add or replace query parameters
	QueryParametersToRemove []string            // remove query parameters by name

	AnalyticsMetadata     map[string]any            // custom analytics metadata
	DynamicMetadata       map[string]map[string]any // dynamic metadata by namespace
	AnalyticsHeaderFilter DropHeaderAction          // headers to exclude from analytics
}

func (UpstreamRequestHeaderModifications) isRequestHeaderAction() {}

// ImmediateResponse also implements RequestHeaderAction — returning it short-circuits
// the chain and sends the response directly to the downstream client.
func (ImmediateResponse) isRequestHeaderAction() {}

// ResponseHeaderAction is a sealed oneof returned by OnResponseHeaders.
// Implement either DownstreamResponseHeaderModifications or return ImmediateResponse.
type ResponseHeaderAction interface {
	isResponseHeaderAction()
}

// DownstreamResponseHeaderModifications continues with the specified response header
// modifications applied before the response is forwarded to the client.
type DownstreamResponseHeaderModifications struct {
	HeadersToSet    map[string]string   // overwrite header (last write wins)
	HeadersToAppend map[string][]string // append value(s), preserving any existing values
	HeadersToRemove []string            // remove by name (case-insensitive)

	AnalyticsMetadata     map[string]any            // custom analytics metadata
	DynamicMetadata       map[string]map[string]any // dynamic metadata by namespace
	AnalyticsHeaderFilter DropHeaderAction          // headers to exclude from analytics
}

func (DownstreamResponseHeaderModifications) isResponseHeaderAction() {}

// ImmediateResponse also implements ResponseHeaderAction — returning it short-circuits
// the chain and sends the response directly to the downstream client.
func (ImmediateResponse) isResponseHeaderAction() {}

// ─── Buffered body actions (sealed oneof) ────────────────────────────────────
//
// RequestAction and ResponseAction are sealed interfaces. Each has two concrete
// implementations — one for mutations (continue to upstream/client), one for
// short-circuiting (ImmediateResponse). StopExecution() lets callers branch
// without a type assertion; a type switch is still needed to access fields.

// RequestAction is a sealed oneof returned by RequestPolicy.OnRequestBody.
// Implement either UpstreamRequestModifications or return ImmediateResponse.
type RequestAction interface {
	isRequestAction()
	// StopExecution returns true when the chain should be short-circuited and
	// the response returned directly to the downstream client.
	StopExecution() bool
}

// UpstreamRequestModifications forwards the request to upstream with the
// specified mutations. Returned when processing should continue normally.
// Because the request body is fully buffered, header and routing mutations
// applied here are still effective.
type UpstreamRequestModifications struct {
	Body []byte // nil = passthrough; []byte{} = clear body

	HeadersToSet    map[string]string   // overwrite header (last write wins)
	HeadersToAppend map[string][]string // append value(s), preserving any existing values
	HeadersToRemove []string            // remove by name (case-insensitive)

	// Routing mutations — applied before the request is forwarded to upstream.
	UpstreamName            *string             // route to a named upstream definition (nil = no change)
	UpstreamSlot            *UpstreamSlot       // route to the API's main or sandbox upstream slot instead of the slot implied by the current vhost (nil = no change); requesting a slot the API doesn't have configured is a no-op (logged, not an error)
	Path                    *string             // rewrite the request path (nil = no change)
	Host                    *string             // rewrite the :authority header (nil = no change)
	Method                  *string             // rewrite the request method (nil = no change)
	QueryParametersToAdd    map[string][]string // add or replace query parameters
	QueryParametersToRemove []string            // remove query parameters by name

	AnalyticsMetadata     map[string]any            // custom analytics metadata
	DynamicMetadata       map[string]map[string]any // dynamic metadata by namespace
	AnalyticsHeaderFilter DropHeaderAction          // headers to exclude from analytics
}

func (UpstreamRequestModifications) isRequestAction()    {}
func (UpstreamRequestModifications) StopExecution() bool { return false }

// ImmediateResponse also implements RequestAction — returning it short-circuits
// the chain and sends the response directly to the downstream client.
func (ImmediateResponse) isRequestAction()    {}
func (ImmediateResponse) StopExecution() bool { return true }

// ResponseAction is a sealed oneof returned by ResponsePolicy.OnResponseBody.
// Implement either DownstreamResponseModifications or return ImmediateResponse.
type ResponseAction interface {
	isResponseAction()
	// StopExecution returns true when the entire response should be replaced by
	// this ImmediateResponse rather than forwarding the upstream response body.
	StopExecution() bool
}

// DownstreamResponseModifications forwards the response to the client with the
// specified mutations. The request headers are already committed to upstream,
// but status, body, and response headers can still be changed.
type DownstreamResponseModifications struct {
	Body       []byte // nil = passthrough; []byte{} = clear body
	StatusCode *int   // nil = no change
	// IsFault states whether this modification represents a failure the fault flow should act
	// on. False — the zero value — means NO, exactly as on ImmediateResponse.
	//
	// This action carries every ordinary response mutation, not only rejections, so the
	// default matters more here than anywhere else: a set-headers policy on a healthy 200
	// must not be a fault, and neither must a policy relabelling the backend's error or
	// interceptor-service applying whatever status an external interceptor returned.
	//
	// The status is not consulted. An earlier design inferred a rejection from "the policy
	// changed the status", which misread all three of those cases; a response guardrail that
	// rejects says so with this field instead.
	//
	// Whether a policy SUPPLIED a status is a separate question, answered by StatusCode
	// directly, and is what the tracing outcome uses to distinguish an overridden status from
	// the upstream's own. That question never needed this field.
	IsFault bool
	// Fault describes the failure for a renderer. Optional, and independent of IsFault:
	// setting it does not make the modification a fault.
	Fault *FaultDetails

	HeadersToSet    map[string]string   // overwrite header (last write wins)
	HeadersToAppend map[string][]string // append value(s), preserving any existing values
	HeadersToRemove []string            // remove by name (case-insensitive)

	AnalyticsMetadata     map[string]any            // custom analytics metadata
	DynamicMetadata       map[string]map[string]any // dynamic metadata by namespace
	AnalyticsHeaderFilter DropHeaderAction          // headers to exclude from analytics
}

func (DownstreamResponseModifications) isResponseAction()   {}
func (DownstreamResponseModifications) StopExecution() bool { return false }

// ImmediateResponse also implements ResponseAction — returning it replaces the
// entire upstream response with the specified status, headers, and body.
func (ImmediateResponse) isResponseAction() {}

// ─── Fault phase action ──────────────────────────────────────────────────────

// FaultResponse is what a fault policy returns from FaultPolicy.OnFault.
//
// Not a member of the response phase's ResponseAction oneof: that oneof asks "forward or
// replace?", and on this path there is no upstream response to forward — every fault entry
// edits the same object.
//
// Two ImmediateResponse fields are omitted as misleading here:
//
//	IsFault  meaningless — the fault flow is already running
//	Headers  a whole-map replacement, where a fault entry wants to ADD a header
//
// nil is the ordinary return and means "I changed nothing". Every field is optional and
// merges: a zero FaultResponse changes nothing, and header operations apply on top of the
// headers the error already has.
//
// Nothing here stops the chain: every fault entry runs, in order, and a later entry sees
// what an earlier one did. An entry that should not run for some failures says so with its
// executionCondition.
type FaultResponse struct {
	// StatusCode overrides the error's status. nil — the common case — keeps it.
	//
	// Raising or lowering the status does not change whether this is a fault; that was
	// settled before the chain ran, and is not re-derived from anything here.
	StatusCode *int

	// Body replaces the error body.
	//
	// nil means "leave it alone" and []byte{} means "clear it", the same distinction the
	// rest of the SDK draws. Setting a body is also what tells the gateway not to render
	// its own: a fault entry writing a body has decided what the client receives, so the
	// protocol formatter stands down for this response. That is unconditional here, unlike
	// on the producing policy, where a body accompanying a described Fault is treated as a
	// fallback — a fault entry ran AFTER the description and with knowledge of it, so its
	// body is the later decision rather than a stand-in.
	Body []byte

	// Fault re-describes the failure, for a renderer or a later entry. nil keeps the
	// existing description.
	Fault *FaultDetails

	// Header operations, applied over the error's existing headers rather than replacing
	// them. HeadersToSet overwrites a named header, HeadersToAppend adds to it, and
	// HeadersToRemove drops it; removal is case-insensitive.
	HeadersToSet    map[string]string
	HeadersToAppend map[string][]string
	HeadersToRemove []string

	AnalyticsMetadata     map[string]any            // custom analytics metadata
	DynamicMetadata       map[string]map[string]any // dynamic metadata by namespace
	AnalyticsHeaderFilter DropHeaderAction          // headers to exclude from analytics
}

// Compile-time interface satisfaction checks.
// These ensure ImmediateResponse satisfies all sealed action interfaces and that
// the concrete modification types satisfy their respective action interfaces.
var (
	_ RequestHeaderAction     = UpstreamRequestHeaderModifications{}
	_ RequestHeaderAction     = ImmediateResponse{}
	_ ResponseHeaderAction    = DownstreamResponseHeaderModifications{}
	_ ResponseHeaderAction    = ImmediateResponse{}
	_ RequestAction           = UpstreamRequestModifications{}
	_ RequestAction           = ImmediateResponse{}
	_ ResponseAction          = DownstreamResponseModifications{}
	_ ResponseAction          = ImmediateResponse{}
	_ StreamingRequestAction  = ForwardRequestChunk{}
	_ StreamingResponseAction = ForwardResponseChunk{}
	_ StreamingResponseAction = TerminateResponseChunk{}
)

// ─── Streaming body actions ───────────────────────────────────────────────────
//
// Streaming hooks receive one chunk at a time. By the time chunks arrive, both
// request headers (sent upstream) and response headers (sent downstream) are
// already committed. Only the chunk content can be changed.
//
// ImmediateResponse is NOT available in streaming chunk actions:
//   - For request chunks: the upstream connection is already open; use
//     RequestHeaderPolicy or RequestPolicy to reject before the body starts.
//   - For response chunks: the client has already received the response headers
//     and status; injecting a new response mid-stream is physically impossible.
//
// Mid-stream error handling:
// If the kernel encounters an error while processing a streaming chunk it will
// call StreamingRequestPolicy.OnStreamError / StreamingResponsePolicy.OnStreamError
// on all enabled policies in the chain so they can release held resources.
// The kernel then closes the gRPC ext_proc stream, which causes Envoy to abort
// the HTTP/2 stream with a RESET_STREAM. The downstream client will see an
// abrupt connection close rather than a structured HTTP error response.
// There is no recovery path — once chunk processing has started, a clean
// error response is not possible.

// StreamingRequestAction is a sealed oneof returned by StreamingRequestPolicy.OnRequestBodyChunk.
// Implement ForwardRequestChunk to continue normally.
// ImmediateResponse is not available in chunk actions — request headers are already
// committed to upstream by the time chunks are processed.
type StreamingRequestAction interface {
	isStreamingRequestAction()
}

// ForwardRequestChunk forwards the chunk to upstream with an optional body replacement.
// Return this when the chunk should be passed through (Body == nil) or rewritten.
type ForwardRequestChunk struct {
	Body []byte // nil = passthrough; non-nil bytes replace the chunk

	// Analytics — accumulates incremental data across chunks (e.g. token counts).
	AnalyticsMetadata map[string]any
	DynamicMetadata   map[string]map[string]any
}

func (ForwardRequestChunk) isStreamingRequestAction() {}

// StreamingResponseAction is a sealed oneof returned by StreamingResponsePolicy.OnResponseBodyChunk.
// Implement ForwardResponseChunk to continue normally, or TerminateResponseChunk to close
// the stream after this chunk. ImmediateResponse is not available — response headers are
// already committed to the downstream client.
type StreamingResponseAction interface {
	isStreamingResponseAction()
	// TerminateStream returns true when the policy engine should stop executing remaining
	// policies in the chain and close the stream after delivering this chunk to the client.
	// This is the correct way to signal guardrail intervention mid-stream: set Body to a
	// final SSE event (e.g. an error frame or [DONE]) and return TerminateResponseChunk.
	// Because response headers are already committed, no HTTP-level error status can be
	// sent — the stream is closed cleanly after the final chunk is delivered.
	TerminateStream() bool
}

// ForwardResponseChunk forwards the chunk to the downstream client with an optional body
// replacement. Return this when processing should continue normally.
type ForwardResponseChunk struct {
	Body []byte // nil = passthrough; non-nil bytes replace the chunk

	// Analytics — accumulates incremental data across chunks (e.g. per-SSE-event token counts).
	AnalyticsMetadata map[string]any
	DynamicMetadata   map[string]map[string]any
}

func (ForwardResponseChunk) isStreamingResponseAction() {}
func (ForwardResponseChunk) TerminateStream() bool      { return false }

// TerminateResponseChunk delivers a final chunk to the downstream client and then closes
// the stream. Use this for mid-stream guardrail intervention: set Body to a final SSE
// event (e.g. an error frame or [DONE]) before returning this action.
type TerminateResponseChunk struct {
	Body []byte // nil = deliver an empty final chunk before closing

	// IsFault states whether closing the stream here is a FAILURE.
	//
	// This action is genuinely ambiguous without it: the same call ends a stream because a
	// guardrail intervened AND because a transformer saw the upstream's final event and closed
	// cleanly. Those look identical from outside, so the gateway cannot tell them apart.
	//
	// The buffered actions declare too, and here there is not even a status to fall back on —
	// it was sent with the headers, long before this chunk, and for a streamed response it is
	// almost always 200. So the zero value is treated as NOT a failure:
	// a clean [DONE] close is the more common case, and inventing a fault for it would notify
	// on every successful stream. A guardrail intervening mid-stream must set this to true.
	IsFault bool
	// Fault describes the failure, for a fault policy reporting it. Optional.
	//
	// Note it cannot change what the client receives: by the time a chunk is being terminated
	// the status, headers and earlier chunks are already committed. It is for observability.
	Fault *FaultDetails

	// Analytics — accumulates incremental data across chunks (e.g. per-SSE-event token counts).
	AnalyticsMetadata map[string]any
	DynamicMetadata   map[string]map[string]any
}

func (TerminateResponseChunk) isStreamingResponseAction() {}
func (TerminateResponseChunk) TerminateStream() bool      { return true }

// FaultDetails is the gateway's one description of a failure it produced: what failed, and
// in enough detail for a renderer to express it in the caller's protocol.
//
// One type for two audiences: a policy fills it in to describe the rejection it made, and a
// formatter reads it to build the body the client receives.
//
// It describes; it does not decide. Whether a response enters the fault flow is stated by the
// action's IsFault field, kept separate so a policy can say "this is not a failure" without
// having an error to describe.
//
// Deliberately absent: no status code (it belongs to the response, and a second copy could
// only drift) and no source (a policy-supplied error is gateway-produced by definition;
// provenance is reported on FaultContext).
type FaultDetails struct {
	// Code is a stable machine identifier for the failure. Empty when not classified.
	//
	// The values live in fault_codes.go: FaultCode* for the reused codes,
	// GuardrailCode* for a content rejection, and the range constants for allocating a new
	// one. Reach for a constant before a literal — nine of these numbers were re-typed
	// seventy-one times across the shipped policies before they had a shared home.
	Code string
	// Type is the failure class — see the FaultType constants in fault_codes.go for the
	// canonical values and what each one means. Empty when not classified.
	//
	// It is the one field a caller can branch on without a code table, so prefer a
	// constant over a new string: two policies describing the same class differently
	// makes the field unusable for exactly the consumers it exists for.
	Type string
	// Direction states which side of the exchange was rejected: DirectionRequest when the
	// content the caller sent was refused, DirectionResponse when the content the upstream
	// returned was. These are operationally very different events that share a status code,
	// and only the policy knows which it did.
	Direction string
	// Message is a short human-readable summary, safe to return to the caller.
	Message string
	// Description is longer detail, which a renderer MAY withhold. For a guardrail rejection
	// this is the content the guardrail blocked, so it must never be assumed safe to forward.
	Description string
	// Policy names the policy that produced this failure, and is empty when no policy did —
	// a router failure, for instance. Empty means "not caused by a policy", never "unknown".
	//
	// GATEWAY-OWNED: the gateway sets this from the chain it just executed, overwriting
	// whatever a policy put here. A policy naming itself adds nothing it could not get
	// wrong, and a wrong attribution is worse than none — but a formatter needs the name in
	// the same object as the rest of the error, so it lives here rather than only on
	// FaultContext.
	Policy string
	// Guardrail carries the assessment detail specific to a guardrail rejection, and is nil
	// for every other failure. A pointer rather than a value so "not a guardrail error" is
	// expressible at all: a zero-valued struct is indistinguishable from a guardrail that
	// reported nothing.
	//
	// Its contents are NOT safe to forward by default — see GuardrailDetails.
	Guardrail *GuardrailDetails
	// JSONRPC carries wire detail for a caller whose protocol is JSON-RPC — MCP today, A2A
	// next — and is nil for every other caller.
	//
	// It is a different KIND of field from Guardrail, and the distinction is worth keeping
	// straight. Guardrail is domain detail: what happened, renderable in any shape, so every
	// renderer emits it. This is protocol detail: how the failure must be spelled on one
	// particular wire. Only the JSON-RPC renderer consumes it; the JSON and XML renderers
	// ignore it, correctly — neither has anywhere to put a JSON-RPC code.
	//
	// So it is the first of a per-protocol pattern rather than a special case for MCP. A SOAP
	// fault needs the same treatment, for the same reason: faultcode/faultsubcode are no more
	// derivable from an HTTP status than a JSON-RPC code is.
	JSONRPC *JSONRPCError
}

// JSONRPCError is the JSON-RPC detail only the policy that read the request can supply.
//
// Both fields exist because the engine cannot obtain them. It derives a code from the HTTP
// status (4xx/5xx onto -32600/-32603), which is right for a gateway failure and wrong for a
// policy that parsed the request and knows the call was -32602; and it has no request id at
// all, because most errors are produced in the header phase before any body was read.
//
// A policy that DID read the body knows both. This is the channel for saying so.
type JSONRPCError struct {
	// Code is the JSON-RPC error code, e.g. -32602 for invalid params.
	//
	// A pointer so "unset" is expressible: nil means "derive from the status", which is what
	// every failure the engine itself produces wants. Zero is not a valid JSON-RPC code, but
	// relying on that would make the field's meaning depend on a fact about the protocol
	// rather than on the type.
	Code *int
	// ID is the request id, echoed back so a client with several calls in flight can tell
	// which one failed. Nil renders as JSON-RPC's null, which the spec requires for a request
	// whose id could not be determined.
	//
	// Typed as any because JSON-RPC permits a string, a number, or null, and a policy reading
	// it out of the request body gets whichever the client sent. The same reasoning as
	// GuardrailDetails.Assessments, which is a map for the same "whatever was there" reason.
	ID any
}

// GuardrailDetails is the detail a guardrail reports about an intervention.
//
// The field set mirrors the envelope the eleven shipped guardrails already publish in their
// documentation, so adopting this type is a re-homing of an existing contract rather than a
// new one customers must learn. That envelope's vocabulary is AWS Bedrock's ApplyGuardrail
// naming, generalised across guardrails that have no AWS involvement.
//
// Direction is deliberately not repeated here: it is on FaultDetails, which every guardrail
// error already carries.
type GuardrailDetails struct {
	// InterveningGuardrail is the guardrail that acted, e.g. "url-guardrail".
	InterveningGuardrail string
	// Action is what the guardrail did, e.g. GuardrailActionIntervened.
	Action string
	// ActionReason is a short explanation of why it acted.
	ActionReason string
	// Assessments is the per-guardrail evidence for the intervention.
	//
	// For a RESPONSE guardrail this frequently contains the very content the guardrail
	// existed to stop from leaving, so a formatter must treat it as withheld unless the
	// operator opted in — the same reasoning that gates the shipped guardrails' own
	// showAssessment parameter, and the same reasoning behind Description.
	Assessments map[string]any
}

// Guardrail action values for GuardrailDetails.Action.
const (
	// GuardrailActionIntervened is the value the shipped guardrail envelope uses.
	GuardrailActionIntervened = "GUARDRAIL_INTERVENED"
)

// Direction values for FaultDetails.Direction.
//
// The published guardrail error contract spells these upper-case ("REQUEST"/"RESPONSE"); a
// renderer targeting that shape converts rather than storing a second spelling here.
const (
	DirectionRequest  = "Request"
	DirectionResponse = "Response"
)
