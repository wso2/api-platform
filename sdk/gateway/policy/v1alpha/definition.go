package policyv1alpha

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

	// Parameters for THIS version
	// Each schema defines name, type, validation rules
	Parameters map[string]interface{} `yaml:"parameters" json:"parameters"`

	// SystemParameters for THIS version
	SystemParameters map[string]interface{} `yaml:"systemParameters" json:"systemParameters"`
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
