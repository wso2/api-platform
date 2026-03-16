package policyv1alpha

// ProcessingMode declares which HTTP phases a LegacyPolicy handles.
// The kernel uses Mode() at chain-build time to decide which executor
// paths to wire up for the policy.
type ProcessingMode int

const (
	// ModeRequest — only the request phase (OnRequest) is invoked.
	ModeRequest ProcessingMode = iota
	// ModeResponse — only the response phase (OnResponse) is invoked.
	ModeResponse
	// ModeRequestResponse — both OnRequest and OnResponse are invoked.
	ModeRequestResponse
)

// LegacyPolicy is the backward-compatible monolithic policy interface.
// It is provided for policies written before the phase-specific sub-interface
// model was introduced. The kernel detects this interface at chain-build time
// and routes request/response phases to OnRequest/OnResponse accordingly.
//
// Unlike the current model where params are consumed once inside GetPolicy,
// LegacyPolicy receives the merged params map on every OnRequest and OnResponse
// call. The kernel stores the params at chain-build time and forwards them on
// each invocation.
//
// New policies should implement the phase-specific sub-interfaces (RequestPolicy,
// ResponsePolicy, etc.) directly. LegacyPolicy is a compatibility shim and
// may be removed in a future major version.
type LegacyPolicy interface {
	Policy
	// Mode declares which phases this policy participates in.
	Mode() ProcessingMode
	// OnRequest is called once the full request body is buffered.
	// params is the same merged map that was stored at chain-build time.
	OnRequest(ctx *RequestContext, params map[string]interface{}) RequestAction
	// OnResponse is called once the full response body is buffered.
	// params is the same merged map that was stored at chain-build time.
	OnResponse(ctx *ResponseContext, params map[string]interface{}) ResponseAction
}
