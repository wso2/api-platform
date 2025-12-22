package policyenginev1

// PolicyChain represents the configuration for a policy chain on a route.
// This is the core configuration structure used for xDS communication between
// gateway-controller and policy-engine.
type PolicyChain struct {
	// RouteKey uniquely identifies the route (format: Method|Path|Vhost)
	// Example: "GET|/weather/us/seattle|localhost"
	// Must match the route name in Envoy configuration
	RouteKey string `json:"route_key" yaml:"route_key"`

	// Policies to execute for this route (in order)
	Policies []PolicyInstance `json:"policies" yaml:"policies"`
}

// PolicyInstance represents a single policy instance in a chain.
// This defines how a specific policy should be configured and executed.
type PolicyInstance struct {
	// Name of the policy (e.g., "jwtValidation", "rateLimiting")
	Name string `json:"name" yaml:"name"`

	// Version of the policy (e.g., "v1.0.0")
	Version string `json:"version" yaml:"version"`

	// Enabled controls whether this policy is active
	// If false, policy is skipped regardless of ExecutionCondition
	Enabled bool `json:"enabled" yaml:"enabled"`

	// ExecutionCondition is an optional CEL expression for conditional execution
	// If nil or empty, policy always executes (when Enabled=true)
	// If non-nil, policy only executes when expression evaluates to true
	ExecutionCondition *string `json:"executionCondition,omitempty" yaml:"executionCondition,omitempty"`

	// Parameters contains configuration parameters for the policy
	// Structure depends on the specific policy's schema
	Parameters map[string]interface{} `json:"parameters" yaml:"parameters"`
}

// Configuration represents a collection of policy chains with metadata.
// This is used for xDS communication to group multiple PolicyChain configurations.
type Configuration struct {
	// Routes contains policy chain configurations for multiple routes
	Routes []PolicyChain `json:"routes" yaml:"routes"`

	// Metadata contains contextual information about this configuration
	Metadata Metadata `json:"metadata" yaml:"metadata"`
}

// Metadata contains metadata about a policy configuration.
// Used for tracking configuration versions and associating with APIs.
type Metadata struct {
	// CreatedAt is the Unix timestamp of when the configuration was created
	CreatedAt int64 `json:"created_at" yaml:"created_at"`

	// UpdatedAt is the Unix timestamp of when the configuration was last updated
	UpdatedAt int64 `json:"updated_at" yaml:"updated_at"`

	// ResourceVersion is used for optimistic concurrency control
	ResourceVersion int64 `json:"resource_version" yaml:"resource_version"`

	// APIId is the unique identifier of the API this policy configuration belongs to
	APIId string `json:"api_id" yaml:"api_id"`

	// APIName is the name of the API this policy configuration belongs to
	APIName string `json:"api_name" yaml:"api_name"`

	// Version is the version of the API this policy configuration belongs to
	Version string `json:"version" yaml:"version"`

	// Context is the base path of the API
	Context string `json:"context" yaml:"context"`
}
