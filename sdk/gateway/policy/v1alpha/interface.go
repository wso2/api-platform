package policyv1alpha

import core "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"

// PolicyMetadata contains metadata passed to GetPolicy for instance creation
// This will be passed to the GetPolicy factory function to provide context about policy
type PolicyMetadata struct {
	// RouteName is the unique identifier for the route this policy is attached to
	RouteName string

	// APIId is the unique identifier of the API this policy belongs to
	APIId string

	// APIName is the name of the API this policy belongs to
	APIName string

	// APIVersion is the version of the API this policy belongs to
	APIVersion string

	// AttachedTo indicates where the policy is attached (e.g., LevelAPI, LevelRoute)
	AttachedTo Level
}

// Policy is the base interface that all policies must implement
type Policy interface {

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

// PolicyFactory is the function signature for creating policy instances
// Policy implementations must export a GetPolicy function with this signature:
//
//	func GetPolicy(
//	    metadata PolicyMetadata,
//	    params map[string]interface{},
//	) (Policy, error)
//
// Parameters:
//   - metadata: Contains route-level metadata (routeName, etc.)
//   - params: Merged parameters combining static config (from policy definition
//     with resolved ${config} references) and runtime parameters (from API
//     configuration). Runtime params override static config on key conflicts.
//
// Returns:
//   - Policy instance (can be singleton, cached, or per-route)
//   - Error if initialization/validation fails
//
// The policy should perform all initialization, validation, and preprocessing
// in GetPolicy. This includes parsing configuration, caching expensive operations,
// and setting up any required state.
type PolicyFactory func(metadata PolicyMetadata, params map[string]interface{}) (Policy, error)

// ProcessingMode, HeaderProcessingMode, and BodyProcessingMode are type aliases for
// the canonical definitions in sdk/core. This allows policies to implement Mode()
// once using the core types and satisfy both policyv1alpha.Policy and policyv1alpha2.Policy.
type ProcessingMode = core.ProcessingMode
type HeaderProcessingMode = core.HeaderProcessingMode
type BodyProcessingMode = core.BodyProcessingMode

const (
	HeaderModeSkip    HeaderProcessingMode = core.HeaderModeSkip
	HeaderModeProcess HeaderProcessingMode = core.HeaderModeProcess
)

const (
	BodyModeSkip   BodyProcessingMode = core.BodyModeSkip
	BodyModeBuffer BodyProcessingMode = core.BodyModeBuffer
	BodyModeStream BodyProcessingMode = core.BodyModeStream
)

// Level defines the attachment level of a policy
type Level string

const (
	// LevelAPI indicates the policy is attached at the API level
	LevelAPI Level = "api"

	// LevelRoute indicates the policy is attached at the route level
	LevelRoute Level = "route"
)
