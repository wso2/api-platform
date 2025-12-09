package policyv1alpha

// ParameterSchema defines validation rules for a policy parameter
type ParameterSchema struct {
	// Parameter name (e.g., "jwksUrl", "maxRequests", "allowedOrigins")
	Name string `yaml:"name" json:"name"`

	// Parameter type (string, int, float, duration, array, uri, etc.)
	Type ParameterType `yaml:"type" json:"type"`

	// Human-readable description for documentation
	Description string `yaml:"description" json:"description"`

	// true if parameter must be provided in configuration
	Required bool `yaml:"required" json:"required"`

	// Default value if not provided (must match Type)
	// nil if no default (required parameters should have no default)
	Default interface{} `yaml:"default,omitempty" json:"default,omitempty"`

	// Validation rules based on type
	// Contains type-specific constraints (min/max, pattern, enum, etc.)
	Validation ValidationRules `yaml:"validation,omitempty" json:"validation,omitempty"`
}

// PolicyParameters holds policy configuration with type-safe validated values
type PolicyParameters struct {
	// Raw parameter values as received from xDS config (JSON/YAML)
	Raw map[string]interface{}

	// Validated parameters matching the policy's schema
	// Validated at configuration time, not execution time
	// Key: parameter name, Value: typed validated value
	Validated map[string]TypedValue
}

// PolicyDefinition describes a specific version of a policy
type PolicyDefinition struct {
	// Policy name (e.g., "jwtValidation", "rateLimiting")
	Name string `yaml:"name" json:"name"`

	// Semantic version of THIS definition (e.g., "v1.0.0", "v2.0.0")
	// Each version gets its own PolicyDefinition
	Version string `yaml:"version" json:"version"`

	// Human-readable description of what this policy version does
	Description string `yaml:"description" json:"description"`

	// Parameters for THIS version
	// Each schema defines name, type, validation rules
	Parameters []ParameterSchema `yaml:"parameters" json:"parameters"`
}

// PolicySpec is a configuration instance specifying how to use a policy
type PolicySpec struct {
	// Policy identifier (e.g., "jwtValidation", "rateLimiting")
	// Must match a registered policy name in PolicyRegistry
	Name string `yaml:"name" json:"name"`

	// Semantic version of policy implementation (e.g., "v1.0.0", "v2.1.0")
	// Must match a registered policy version in PolicyRegistry
	Version string `yaml:"version" json:"version"`

	// Static enable/disable toggle
	// If false, policy never executes regardless of ExecutionCondition
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Typed and validated configuration parameters
	// Validated against PolicyDefinition.ParameterSchemas at config time
	Parameters PolicyParameters `yaml:"parameters" json:"parameters"`

	// Optional CEL expression for dynamic conditional execution
	// nil = always execute (when Enabled=true)
	// non-nil = only execute when expression evaluates to true
	// Expression context: RequestContext (request phase) or ResponseContext (response phase)
	ExecutionCondition *string `yaml:"executionCondition,omitempty" json:"executionCondition,omitempty"`
}
