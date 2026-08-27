/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package policyv1alpha2

import "time"

// APIKind identifies the kind of API being processed.
type APIKind string

const (
	APIKindRestApi     APIKind = "RestApi"
	APIKindLlmProvider APIKind = "LlmProvider"
	APIKindLlmProxy    APIKind = "LlmProxy"
	APIKindMCP         APIKind = "Mcp"
	APIKindWebSubApi   APIKind = "WebSubApi"
	APIKindGraphQL     APIKind = "GraphQLApi"
	APIKindAgent       APIKind = "Agent"
)

// UpstreamSlot identifies one of an API's built-in upstream slots.
type UpstreamSlot string

const (
	UpstreamSlotMain    UpstreamSlot = "main"
	UpstreamSlotSandbox UpstreamSlot = "sandbox"
)

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
