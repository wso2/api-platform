package policyv1alpha

// Policy is the interface all policies must implement.
// It uses a monolithic style: Mode() declares processing requirements, and
// OnRequest/OnResponse are called by the kernel for the respective phases.
//
// New policies should be written against policyv2alpha, which uses a clean
// sub-interface model where capabilities are declared by implementing
// RequestHeaderPolicy, RequestPolicy, ResponsePolicy, etc.
type Policy interface {
	// Mode declares which phases this policy participates in and what body
	// access is required. The kernel inspects this once at chain-build time.
	Mode() ProcessingMode

	// OnRequest is called once the full request body is buffered (or immediately
	// if no body buffering is required). params is the merged map of init and
	// runtime parameters stored at chain-build time.
	OnRequest(ctx *RequestContext, params map[string]interface{}) RequestAction

	// OnResponse is called once the full response body is buffered (or immediately
	// if no body buffering is required). params is the same merged map.
	OnResponse(ctx *ResponseContext, params map[string]interface{}) ResponseAction
}

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

// PolicyFactory is the function signature for creating policy instances.
// Policy implementations must export a GetPolicy function with this signature:
//
//	func GetPolicy(metadata PolicyMetadata, params map[string]interface{}) (Policy, error)
type PolicyFactory func(metadata PolicyMetadata, params map[string]interface{}) (Policy, error)

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
	// Deprecated: Use policyv2alpha.StreamingRequestPolicy / StreamingResponsePolicy instead.
	BodyModeStream BodyProcessingMode = "STREAM"
)
