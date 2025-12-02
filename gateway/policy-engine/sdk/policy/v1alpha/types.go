package policyv1alpha

import "time"

// ParameterType defines the type of a policy parameter
type ParameterType string

const (
	ParameterTypeString      ParameterType = "string"
	ParameterTypeInt         ParameterType = "int"
	ParameterTypeFloat       ParameterType = "float"
	ParameterTypeBool        ParameterType = "bool"
	ParameterTypeDuration    ParameterType = "duration"
	ParameterTypeStringArray ParameterType = "string_array"
	ParameterTypeIntArray    ParameterType = "int_array"
	ParameterTypeMap         ParameterType = "map"
	ParameterTypeURI         ParameterType = "uri"
	ParameterTypeEmail       ParameterType = "email"
	ParameterTypeHostname    ParameterType = "hostname"
	ParameterTypeIPv4        ParameterType = "ipv4"
	ParameterTypeIPv6        ParameterType = "ipv6"
	ParameterTypeUUID        ParameterType = "uuid"
)

// TypedValue represents a validated parameter value with type information
type TypedValue struct {
	// Parameter type (string, int, float, bool, duration, array, map, uri, email, etc.)
	Type ParameterType

	// Actual value after validation and type conversion
	// Go native type matching ParameterType:
	//   string → string
	//   int → int64
	//   float → float64
	//   bool → bool
	//   duration → time.Duration
	//   string_array → []string
	//   int_array → []int64
	//   uri → string (validated as URI)
	//   email → string (validated as email)
	//   hostname → string (validated as hostname)
	//   ipv4 → string (validated as IPv4)
	//   ipv6 → string (validated as IPv6)
	//   uuid → string (validated as UUID)
	//   map → map[string]interface{}
	Value interface{}
}

// ValidationRules contains type-specific validation constraints
type ValidationRules struct {
	// String validation
	MinLength *int
	MaxLength *int
	Pattern   string // regex pattern
	Format    string // email, uri, hostname, ipv4, ipv6, uuid
	Enum      []string

	// Numeric validation (int, float)
	Min        *float64
	Max        *float64
	MultipleOf *float64

	// Array validation
	MinItems    *int
	MaxItems    *int
	UniqueItems bool

	// Duration validation
	MinDuration *time.Duration
	MaxDuration *time.Duration

	// Custom CEL validation expression
	// Expression context: value
	// Must return bool
	CustomValidation *string
}
