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

	// OnRequest executes the policy during request phase
	// Returns RequestAction with modifications or immediate response
	// Returns nil if policy has no action (pass-through)
	OnRequest(ctx *RequestContext, config map[string]interface{}) RequestAction
}

// ResponsePolicy defines the interface for policies that execute during response phase
type ResponsePolicy interface {
	Policy

	// OnResponse executes the policy during response phase
	// Returns ResponseAction with modifications
	// Returns nil if policy has no action (pass-through)
	OnResponse(ctx *ResponseContext, config map[string]interface{}) ResponseAction
}
