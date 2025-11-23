package policies

// Policy is the base interface that all policies must implement
type Policy interface {
	// Validate validates the policy configuration parameters
	// Called at configuration time, not request time
	// Returns error if configuration is invalid
	Validate(params map[string]interface{}) error

	// Mode returns the policy's processing mode for each phase
	// Used by the kernel to optimize execution (e.g., skip body buffering if not needed)
	Mode() ProcessingMode

	// OnRequest executes the policy during request phase
	// Called with request context including headers and body (if body mode is BUFFER)
	// Returns RequestAction with modifications or immediate response
	// Returns nil if policy has no action (pass-through)
	OnRequest(ctx *RequestContext, params map[string]interface{}) RequestAction

	// OnResponse executes the policy during response phase
	// Called with response context including headers and body (if body mode is BUFFER)
	// Returns ResponseAction with modifications
	// Returns nil if policy has no action (pass-through)
	OnResponse(ctx *ResponseContext, params map[string]interface{}) ResponseAction
}

// ProcessingMode declares a policy's processing requirements for each phase
// Used by the kernel to optimize execution (skip unnecessary phases, buffer strategically)
type ProcessingMode struct {
	// RequestHeaderMode specifies if/how the policy processes request headers
	RequestHeaderMode HeaderProcessingMode

	// RequestBodyMode specifies if/how the policy processes request body
	RequestBodyMode BodyProcessingMode

	// ResponseHeaderMode specifies if/how the policy processes response headers
	ResponseHeaderMode HeaderProcessingMode

	// ResponseBodyMode specifies if/how the policy processes response body
	ResponseBodyMode BodyProcessingMode
}

// HeaderProcessingMode defines how a policy processes headers
type HeaderProcessingMode string

const (
	// HeaderModeSkip - Don't process headers, skip method invocation
	HeaderModeSkip HeaderProcessingMode = "SKIP"

	// HeaderModeProcess - Process headers (headers are always available)
	HeaderModeProcess HeaderProcessingMode = "PROCESS"
)

// BodyProcessingMode defines how a policy processes body content
type BodyProcessingMode string

const (
	// BodyModeSkip - Don't process body, skip method invocation
	BodyModeSkip BodyProcessingMode = "SKIP"

	// BodyModeBuffer - Process body with full buffering
	// The kernel buffers complete body before invoking OnRequestBody/OnResponseBody
	BodyModeBuffer BodyProcessingMode = "BUFFER"

	// BodyModeStream - Process body in streaming chunks
	// The kernel invokes streaming methods for each chunk (requires StreamingPolicy interface)
	BodyModeStream BodyProcessingMode = "STREAM"
)
