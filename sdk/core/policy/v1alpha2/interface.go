// Package policyv1alpha2 provides the policy interface for the WSO2 API Gateway.
//
// Policies declare capabilities by implementing phase-specific sub-interfaces.
// The kernel discovers which phases a policy participates in via type assertions
// at chain-build time — once at startup, with zero per-request overhead.
package policyv1alpha2

// Policy is the base interface all policies must implement.
// Mode declares the policy's processing requirements for each phase; the kernel
// uses this at chain-build time to configure buffering and ext_proc modes.
// Capabilities are declared by additionally implementing the phase-specific
// sub-interfaces below.
type Policy interface {
	Mode() ProcessingMode
}

// PolicyFactory is the function signature for creating policy instances.
// Policy implementations must export a GetPolicy function with this signature:
//
//	func GetPolicy(metadata PolicyMetadata, params map[string]interface{}) (Policy, error)
type PolicyFactory func(metadata PolicyMetadata, params map[string]interface{}) (Policy, error)

// PolicyMetadata contains metadata passed to GetPolicy for instance creation.
type PolicyMetadata struct {
	// RouteName is the unique identifier for the route this policy is attached to.
	RouteName string

	// APIId is the unique identifier of the API this policy belongs to.
	APIId string

	// APIName is the name of the API this policy belongs to.
	APIName string

	// APIVersion is the version of the API this policy belongs to.
	APIVersion string

	// AttachedTo indicates where the policy is attached (e.g., LevelAPI, LevelRoute).
	AttachedTo Level
}

// Level defines the attachment level of a policy.
type Level string

const (
	LevelAPI   Level = "api"
	LevelRoute Level = "route"
)

// ─── Processing mode ─────────────────────────────────────────────────────────

// ProcessingMode declares a policy's processing requirements for each phase.
// The kernel uses this at chain-build time to decide whether to buffer bodies
// and which Envoy ext_proc modes to request.
type ProcessingMode struct {
	RequestHeaderMode  HeaderProcessingMode
	RequestBodyMode    BodyProcessingMode
	ResponseHeaderMode HeaderProcessingMode
	ResponseBodyMode   BodyProcessingMode
}

// HeaderProcessingMode defines how a policy processes headers.
type HeaderProcessingMode string

const (
	// HeaderModeSkip — don't process headers; the phase method is not called.
	HeaderModeSkip HeaderProcessingMode = "SKIP"

	// HeaderModeProcess — process headers.
	HeaderModeProcess HeaderProcessingMode = "PROCESS"
)

// BodyProcessingMode defines how a policy processes body content.
type BodyProcessingMode string

const (
	// BodyModeSkip — don't process body; the phase method is not called.
	BodyModeSkip BodyProcessingMode = "SKIP"

	// BodyModeBuffer — buffer the complete body before invoking OnRequest/OnResponse.
	BodyModeBuffer BodyProcessingMode = "BUFFER"

	// BodyModeStream — process body in streaming chunks.
	BodyModeStream BodyProcessingMode = "STREAM"
)

// ─── Phase-specific sub-interfaces ───────────────────────────────────────────

// RequestHeaderPolicy processes request headers.
// Implement this to modify or inspect headers before the request body is read.
type RequestHeaderPolicy interface {
	OnRequestHeaders(ctx *RequestHeaderContext, params map[string]interface{}) RequestHeaderAction
}

// ResponseHeaderPolicy processes response headers.
// Implement this to modify or inspect response headers before the response body is read.
type ResponseHeaderPolicy interface {
	OnResponseHeaders(ctx *ResponseHeaderContext, params map[string]interface{}) ResponseHeaderAction
}

// RequestPolicy processes the complete buffered request body.
// If any policy in the chain implements this interface, the request body is
// buffered before any policy in the chain executes.
type RequestPolicy interface {
	OnRequestBody(ctx *RequestContext, params map[string]interface{}) RequestAction
}

// ResponsePolicy processes the complete buffered response body.
// If any policy in the chain implements only ResponsePolicy (not
// StreamingResponsePolicy), the entire chain uses BUFFERED mode.
type ResponsePolicy interface {
	OnResponseBody(ctx *ResponseContext, params map[string]interface{}) ResponseAction
}

// StreamingRequestPolicy processes the request body chunk-by-chunk.
// RequestPolicy is embedded as a buffered fallback — the kernel calls
// OnRequestBody when streaming is not possible (e.g. the chain has a
// non-streaming policy). NeedsMoreRequestData is called after each chunk;
// return true to accumulate before OnRequestBodyChunk is invoked.
//
// Error handling limitation: if an error occurs after one or more chunks have
// already been forwarded to upstream, the upstream connection is already in
// progress and the error cannot be cleanly surfaced — the stream will be
// aborted. Policies that hold per-stream resources should handle cleanup in
// their own error paths; there is currently no dedicated error-notification
// hook on this interface.
type StreamingRequestPolicy interface {
	RequestPolicy
	OnRequestBodyChunk(ctx *RequestStreamContext, chunk *StreamBody, params map[string]interface{}) RequestChunkAction
	NeedsMoreRequestData(accumulated []byte) bool
}

// StreamingResponsePolicy processes the response body chunk-by-chunk.
// ResponsePolicy is embedded as a buffered fallback — the kernel falls back to
// buffered mode when any policy in the chain does not implement this interface.
// The kernel upgrades to FULL_DUPLEX_STREAMED only when every response policy
// in the chain implements StreamingResponsePolicy.
//
// Error handling limitation: once the kernel has entered FULL_DUPLEX_STREAMED
// mode and flushed at least one chunk downstream, the response status and
// headers are committed to the client. A mid-stream error cannot be surfaced
// as a clean HTTP error response — Envoy will abort the HTTP/2 stream or reset
// the connection. ImmediateResponse is silently ignored in this context. There
// is currently no dedicated error-notification hook on this interface; a future
// OnStreamError method is planned to allow cleanup of per-stream resources
// (open connections, partial buffers, token counters for billing).
type StreamingResponsePolicy interface {
	ResponsePolicy
	OnResponseBodyChunk(ctx *ResponseStreamContext, chunk *StreamBody, params map[string]interface{}) ResponseChunkAction
	NeedsMoreResponseData(accumulated []byte) bool
}
