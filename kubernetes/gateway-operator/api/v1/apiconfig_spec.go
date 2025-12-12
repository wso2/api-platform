/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

// APIConfig defines model for APIConfig from gateway controller API.
// This structure represents the API configuration that will be deployed to gateways.
type APIConfig struct {
	// Kind API type
	// +kubebuilder:validation:Enum=http/rest
	Kind APIConfigurationKind `json:"kind"`

	// Spec contains the API configuration data
	Spec APIConfigData `json:"spec"`

	// Version API specification version
	// +kubebuilder:validation:Enum=api-platform.wso2.com/v1
	Version APIConfigurationVersionType `json:"version"`
}

// APIConfigurationKind API type
type APIConfigurationKind string

const (
	// HTTPRest represents HTTP REST API type
	HTTPRest APIConfigurationKind = "http/rest"
)

// APIConfigurationVersionType API specification version
type APIConfigurationVersionType string

const (
	// APIVersion represents the API specification version
	APIVersion APIConfigurationVersionType = "api-platform.wso2.com/v1"
)

// APIConfigData defines model for APIConfigData.
type APIConfigData struct {
	// Context Base path for all API routes (must start with /, no trailing slash)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^/[a-zA-Z0-9\-._~!$&'()*+,;=:@%/]*[^/]$`
	Context string `json:"context"`

	// Name Human-readable API name (must be URL-friendly - only letters, numbers, spaces, hyphens, underscores, and dots allowed)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9\s\-_.]+$`
	Name string `json:"name"`

	// Operations List of HTTP operations/routes
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Operations []Operation `json:"operations"`

	// Policies List of API-level policies applied to all operations unless overridden
	// +optional
	Policies []Policy `json:"policies,omitempty"`

	// Upstream API-level upstream configuration
	// +kubebuilder:validation:Required
	Upstream UpstreamConfig `json:"upstream"`

	// Version Semantic version of the API
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^v?([0-9]+)(\.[0-9]+)?(\.[0-9]+)?(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`
	Version string `json:"version"`

	// Vhosts Custom virtual hosts/domains for the API
	// +optional
	Vhosts *VhostConfig `json:"vhosts,omitempty"`
}

// UpstreamConfig defines the upstream backend configuration for the API
type UpstreamConfig struct {
	// Main Upstream backend configuration for production traffic
	// +kubebuilder:validation:Required
	Main Upstream `json:"main"`

	// Sandbox Upstream backend configuration for sandbox/testing traffic
	// +optional
	Sandbox *Upstream `json:"sandbox,omitempty"`
}

// VhostConfig defines custom virtual hosts/domains for the API
type VhostConfig struct {
	// Main Custom virtual host/domain for production traffic
	// +kubebuilder:validation:Required
	Main string `json:"main"`

	// Sandbox Custom virtual host/domain for sandbox traffic
	// +optional
	Sandbox *string `json:"sandbox,omitempty"`
}

// Operation defines model for Operation.
type Operation struct {
	// Method HTTP method
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=GET;POST;PUT;PATCH;DELETE;HEAD;OPTIONS
	Method OperationMethod `json:"method"`

	// Path Route path with optional {param} placeholders
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^/[a-zA-Z0-9\-._~!$&'()*+,;=:@%/{}\[\]]*$`
	Path string `json:"path"`

	// Policies List of policies applied only to this operation (overrides or adds to API-level policies)
	// +optional
	Policies []Policy `json:"policies,omitempty"`
}

// OperationMethod HTTP method
type OperationMethod string

const (
	// OperationMethodGET represents HTTP GET method
	OperationMethodGET OperationMethod = "GET"
	// OperationMethodPOST represents HTTP POST method
	OperationMethodPOST OperationMethod = "POST"
	// OperationMethodPUT represents HTTP PUT method
	OperationMethodPUT OperationMethod = "PUT"
	// OperationMethodPATCH represents HTTP PATCH method
	OperationMethodPATCH OperationMethod = "PATCH"
	// OperationMethodDELETE represents HTTP DELETE method
	OperationMethodDELETE OperationMethod = "DELETE"
	// OperationMethodHEAD represents HTTP HEAD method
	OperationMethodHEAD OperationMethod = "HEAD"
	// OperationMethodOPTIONS represents HTTP OPTIONS method
	OperationMethodOPTIONS OperationMethod = "OPTIONS"
)

// Policy defines model for Policy.
type Policy struct {
	// ExecutionCondition Expression controlling conditional execution of the policy
	// +optional
	ExecutionCondition *string `json:"executionCondition,omitempty"`

	// Name Name of the policy
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Params Arbitrary parameters for the policy (free-form key/value structure)
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Params *runtime.RawExtension `json:"params,omitempty"`

	// Version Semantic version of the policy
	// +kubebuilder:validation:Required
	Version string `json:"version"`
}

// Upstream defines model for Upstream.
type Upstream struct {
	// Url Backend service URL (may include path prefix like /api/v2)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^https?://[a-zA-Z0-9\-._~:/?#\[\]@!$&'()*+,;=%]+$`
	Url string `json:"url"`
}
