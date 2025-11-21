package policies

// Policy is the base interface that all policies must implement
type Policy interface {
	// Name returns the policy name (must match PolicyDefinition.Name)
	Name() string

	// Validate validates the policy configuration parameters
	// Called at configuration time, not request time
	// Returns error if configuration is invalid
	Validate(config map[string]interface{}) error
}

// RequestPolicy defines the interface for policies that execute during request phase
type RequestPolicy interface {
	Policy

	// ExecuteRequest executes the policy during request phase
	// Returns RequestPolicyAction with modifications or immediate response
	// Returns nil if policy has no action (pass-through)
	ExecuteRequest(ctx *RequestContext, config map[string]interface{}) *RequestPolicyAction
}

// ResponsePolicy defines the interface for policies that execute during response phase
type ResponsePolicy interface {
	Policy

	// ExecuteResponse executes the policy during response phase
	// Returns ResponsePolicyAction with modifications
	// Returns nil if policy has no action (pass-through)
	ExecuteResponse(ctx *ResponseContext, config map[string]interface{}) *ResponsePolicyAction
}
