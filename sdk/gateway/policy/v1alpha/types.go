package policyv1alpha

import core "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"

// ParameterType defines the type of a policy parameter.
type ParameterType = core.ParameterType

const (
	ParameterTypeString      = core.ParameterTypeString
	ParameterTypeInt         = core.ParameterTypeInt
	ParameterTypeFloat       = core.ParameterTypeFloat
	ParameterTypeBool        = core.ParameterTypeBool
	ParameterTypeDuration    = core.ParameterTypeDuration
	ParameterTypeStringArray = core.ParameterTypeStringArray
	ParameterTypeIntArray    = core.ParameterTypeIntArray
	ParameterTypeMap         = core.ParameterTypeMap
	ParameterTypeURI         = core.ParameterTypeURI
	ParameterTypeEmail       = core.ParameterTypeEmail
	ParameterTypeHostname    = core.ParameterTypeHostname
	ParameterTypeIPv4        = core.ParameterTypeIPv4
	ParameterTypeIPv6        = core.ParameterTypeIPv6
	ParameterTypeUUID        = core.ParameterTypeUUID
)

// TypedValue represents a validated parameter value with type information.
type TypedValue = core.TypedValue

// ValidationRules contains type-specific validation constraints.
type ValidationRules = core.ValidationRules
