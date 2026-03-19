package policyv1alpha

import core "github.com/wso2/api-platform/sdk/core/policy"

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
type PolicyMetadata = core.PolicyMetadata

// PolicyFactory is the function signature for creating policy instances.
// Policy implementations must export a GetPolicy function with this signature:
//
//	func GetPolicy(metadata PolicyMetadata, params map[string]interface{}) (Policy, error)
type PolicyFactory func(metadata PolicyMetadata, params map[string]interface{}) (Policy, error)

// Level defines the attachment level of a policy.
type Level = core.Level

const (
	LevelAPI   = core.LevelAPI
	LevelRoute = core.LevelRoute
)

// ─── Processing mode ─────────────────────────────────────────────────────────

// ProcessingMode declares a policy's processing requirements for each phase.
// The kernel uses this at chain-build time to decide whether to buffer bodies
// and which Envoy ext_proc modes to request.
type ProcessingMode = core.ProcessingMode

// HeaderProcessingMode defines how a policy processes headers.
type HeaderProcessingMode = core.HeaderProcessingMode

const (
	// HeaderModeSkip — don't process headers; the phase method is not called.
	HeaderModeSkip = core.HeaderModeSkip

	// HeaderModeProcess — process headers.
	HeaderModeProcess = core.HeaderModeProcess
)

// BodyProcessingMode defines how a policy processes body content.
type BodyProcessingMode = core.BodyProcessingMode

const (
	// BodyModeSkip — don't process body; the phase method is not called.
	BodyModeSkip = core.BodyModeSkip

	// BodyModeBuffer — buffer the complete body before invoking OnRequest/OnResponse.
	BodyModeBuffer = core.BodyModeBuffer

	// BodyModeStream — process body in streaming chunks.
	// Deprecated: Use policyv2alpha.StreamingRequestPolicy / StreamingResponsePolicy instead.
	BodyModeStream = core.BodyModeStream
)
