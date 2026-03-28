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

	// BodyModeBuffer — buffer the complete body before invoking OnRequestBody/OnResponseBody.
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
// Error handling: if an error occurs after one or more chunks have already been
// forwarded to upstream, the stream will be aborted and the kernel will invoke
// RequestLifecyclePolicy.OnRequestComplete(CompletionError, ...) on all policies
// in the chain that implement RequestLifecyclePolicy. Implement that interface to
// release any per-stream resources (partial buffers, in-flight calls, open spans).
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
// Error handling: once the kernel has entered FULL_DUPLEX_STREAMED mode and
// flushed at least one chunk downstream, the response status and headers are
// committed to the client. A mid-stream error cannot be surfaced as a clean HTTP
// error response — Envoy will abort the HTTP/2 stream with a RESET_STREAM.
// The kernel will invoke RequestLifecyclePolicy.OnRequestComplete(CompletionError, ...)
// on all policies in the chain that implement RequestLifecyclePolicy. Implement
// that interface to release per-stream resources (open connections, partial
// buffers, token counters for billing).
type StreamingResponsePolicy interface {
	ResponsePolicy
	OnResponseBodyChunk(ctx *ResponseStreamContext, chunk *StreamBody, params map[string]interface{}) ResponseChunkAction
	NeedsMoreResponseData(accumulated []byte) bool
}

// ─── Lifecycle hooks ──────────────────────────────────────────────────────────

// CloseablePolicy is implemented by policies that hold instance-level resources
// that must be released when the policy instance is decommissioned.
//
// The kernel calls Close() when an API is updated or deleted via xDS and the
// policy chain is being replaced. Before calling Close(), the kernel drains all
// in-flight requests on the retiring chain — no On* phase methods will be
// invoked concurrently with or after Close(). Close() is called with a bounded
// timeout (default: 30s); the kernel logs an error if Close() exceeds that
// budget.
//
// Typical resources that require Close():
//   - HTTP client connection pools
//   - Database or cache connection pools
//   - Background goroutines started in the PolicyFactory
//   - File handles or OS resources
//   - Registered callbacks or subscribers on shared services
//
// Close() must be idempotent. Return an error only for failures that warrant a
// log entry; partial cleanup errors should be handled and suppressed internally.
type CloseablePolicy interface {
	Close() error
}

// RequestLifecyclePolicy is implemented by policies that hold per-request
// resources that need explicit notification when a request ends.
//
// OnRequestComplete is called exactly once per request after all phase methods
// have returned — or as soon as the kernel detects an abnormal request end
// (client disconnect, upstream error, mid-stream failure). It fires for both
// streaming and non-streaming request paths.
//
// The cause parameter indicates why the request ended:
//   - CompletionNormal: the request completed the full pipeline successfully.
//   - CompletionCancelled: the client disconnected before the response was delivered.
//   - CompletionError: an upstream or policy error ended the request early.
//
// The shared context carries the RequestID, API metadata, and the Metadata map
// populated during the request — useful for finalising observability spans,
// billing records, or accumulated state (e.g. token counts for partial LLM
// responses interrupted by a client disconnect).
//
// Important: the Go context passed to On* phase methods is already cancelled
// when OnRequestComplete runs for non-normal causes. Use a fresh context for
// any cleanup I/O (the kernel provides a short-lived background context with a
// bounded timeout).
type RequestLifecyclePolicy interface {
	OnRequestComplete(cause CompletionCause, shared *SharedContext)
}

// CompletionCause describes why a request ended, passed to
// RequestLifecyclePolicy.OnRequestComplete.
type CompletionCause uint8

const (
	// CompletionNormal indicates the request completed the full pipeline without error.
	CompletionNormal CompletionCause = iota

	// CompletionCancelled indicates the client disconnected before the response
	// was fully delivered (e.g., HTTP/2 RST_STREAM, TCP close mid-flight).
	CompletionCancelled

	// CompletionError indicates an upstream error, policy error, or mid-stream
	// processing failure ended the request before normal completion.
	CompletionError
)

// ─── Default implementations ─────────────────────────────────────────────────

// PolicyBase provides no-op default implementations for the optional lifecycle
// hooks — CloseablePolicy and RequestLifecyclePolicy. Embedding PolicyBase in
// a policy struct satisfies these interfaces with safe defaults; override only
// the hooks your policy actually needs.
//
// PolicyBase intentionally does NOT implement the base Policy interface — it
// does not provide Mode(). Policy authors must still implement Mode() themselves.
//
// Example:
//
//	type MyPolicy struct {
//	    policyv1alpha2.PolicyBase
//	    pool *ConnectionPool
//	}
//
//	func (p *MyPolicy) Mode() policyv1alpha2.ProcessingMode { ... }
//
//	// Only Close needs overriding — OnRequestComplete stays as no-op.
//	func (p *MyPolicy) Close() error { return p.pool.Close() }
type PolicyBase struct{}

func (PolicyBase) Close() error                                           { return nil }
func (PolicyBase) OnRequestComplete(CompletionCause, *SharedContext) {}
