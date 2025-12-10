package policyv1alpha

// Policy is the base interface that all policies must implement
type Policy interface {

	// Mode returns the policy's processing mode for each phase
	// Used by the kernel to optimize execution (e.g., skip body buffering if not needed)
	Mode() ProcessingMode

	// // OnSetup initializes the policy with configuration parameters
	// // Called once by the policy engine during policy instantiation
	// // Returns error if configuration is invalid
	// OnSetup(params map[string]interface{}) error

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

	// // OnDestroy performs cleanup when the policy engine is shutting down
	// // Called once during graceful shutdown to release resources
	// // Policies should close connections, release locks, and cleanup state
	// OnDestroy()
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
