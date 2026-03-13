package policyv1alpha

// Policy is the marker interface that all policies must implement.
// Capabilities are declared by implementing phase-specific sub-interfaces.
// The kernel discovers capabilities at chain-build time using type assertions —
// once, at startup, with zero per-request overhead.
//
// Mode selection rules (evaluated at chain-build time):
//   - If ALL response-body policies implement StreamingResponsePolicy →
//     kernel upgrades Envoy to FULL_DUPLEX_STREAMED at response-headers phase
//     when streaming indicators are detected in the upstream response.
//   - If ANY response-body policy implements only ResponsePolicy →
//     entire chain is forced to BUFFERED mode, preserving the ability to
//     return ImmediateResponse before the client sees any bytes.
type Policy interface{}

// PolicyMetadata contains metadata passed to GetPolicy for instance creation.
// This will be passed to the GetPolicy factory function to provide context about the policy.
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

// PolicyFactory is the function signature for creating policy instances.
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

// ─── Sub-interfaces ──────────────────────────────────────────────────────────
// Policies implement whichever combination they need. The kernel infers the
// required Envoy processing mode from the set of sub-interfaces implemented.

// RequestHeaderPolicy processes request headers.
type RequestHeaderPolicy interface {
	OnRequestHeaders(ctx *RequestHeaderContext) RequestHeaderAction
}

// ResponseHeaderPolicy processes response headers.
type ResponseHeaderPolicy interface {
	OnResponseHeaders(ctx *ResponseHeaderContext) ResponseHeaderAction
}

// RequestPolicy processes the complete buffered request body.
// If any policy in the chain implements this interface the request body is
// forced to BUFFERED mode.
type RequestPolicy interface {
	OnRequestBody(ctx *RequestContext) RequestAction
}

// ResponsePolicy processes the complete buffered response body.
// If any policy in the chain implements only ResponsePolicy (not
// StreamingResponsePolicy), the entire chain is forced to BUFFERED mode.
type ResponsePolicy interface {
	OnResponseBody(ctx *ResponseContext) ResponseAction
}

// StreamingRequestPolicy processes the request body chunk-by-chunk.
// It must also implement RequestPolicy as a buffered fallback for when the
// chain is forced to BUFFERED mode.
// NeedsMoreRequestData is called after each chunk; when it returns true the chunk is
// held and OnRequestBodyChunk is NOT called until NeedsMoreRequestData returns false
// (or end-of-stream is reached). Return false to process each chunk independently.
type StreamingRequestPolicy interface {
	RequestPolicy
	OnRequestBodyChunk(ctx *RequestStreamContext, chunk *StreamBody) RequestChunkAction
	NeedsMoreRequestData(accumulated []byte) bool
}

// StreamingResponsePolicy processes the response body chunk-by-chunk.
// It must also implement ResponsePolicy as a buffered fallback.
// The kernel upgrades Envoy to FULL_DUPLEX_STREAMED only when every
// response policy in the chain implements this interface.
// NeedsMoreResponseData works symmetrically to the request-side method.
type StreamingResponsePolicy interface {
	ResponsePolicy
	OnResponseBodyChunk(ctx *ResponseStreamContext, chunk *StreamBody) ResponseChunkAction
	NeedsMoreResponseData(accumulated []byte) bool
}

// Level defines the attachment level of a policy
type Level string

const (
	// LevelAPI indicates the policy is attached at the API level
	LevelAPI Level = "api"

	// LevelRoute indicates the policy is attached at the route level
	LevelRoute Level = "route"
)
