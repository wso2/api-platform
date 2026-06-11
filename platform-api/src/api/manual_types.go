/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

// Package api contains manually preserved types that are defined in the OpenAPI spec
// but are not referenced by any path — oapi-codegen v2 only emits types reachable from paths.
// These types must be kept here to avoid breaking service code that references them.
package api

import openapi_types "github.com/oapi-codegen/runtime/types"

// ApiIdentifierQ defines the type for the api-identifier query parameter.
// Removed from the spec; preserved here because ValidateRESTAPIParams still uses it.
type ApiIdentifierQ = string

// GatewayStatusResponse is defined in the OpenAPI spec but not referenced by any path.
// Kept here because service/gateway.go uses it for polling responses.
type GatewayStatusResponse struct {
	// Id Unique identifier for the gateway
	Id *openapi_types.UUID `json:"id,omitempty" yaml:"id,omitempty"`

	// IsActive Indicates if the gateway is currently connected to the platform via WebSocket
	IsActive *bool `json:"isActive,omitempty" yaml:"isActive,omitempty"`

	// IsCritical Whether the gateway is critical for production
	IsCritical *bool `json:"isCritical,omitempty" yaml:"isCritical,omitempty"`

	// Name URL-friendly gateway identifier
	Name *string `json:"name,omitempty" yaml:"name,omitempty"`
}

// GatewayStatusListResponse is defined in the OpenAPI spec but not referenced by any path.
// Kept here because service/gateway.go uses it for polling responses.
type GatewayStatusListResponse struct {
	// Count Number of items in current response
	Count      int                     `binding:"required" json:"count" yaml:"count"`
	List       []GatewayStatusResponse `binding:"required" json:"list" yaml:"list"`
	Pagination Pagination              `json:"pagination" yaml:"pagination"`
}

// RESTAPIValidationResponse is defined in the OpenAPI spec but not referenced by any path.
// Kept here because service/api.go uses it for identifier/name-version uniqueness checks.
type RESTAPIValidationResponse struct {
	// Error Error details if validation fails
	Error *struct {
		// Code Error code indicating the type of validation failure
		Code string `json:"code" yaml:"code"`

		// Message Human-readable error message
		Message string `json:"message" yaml:"message"`
	} `binding:"required" json:"error" yaml:"error"`

	// Valid Whether the API identifier or name-version combination is valid (not already in use) in the organization
	Valid bool `binding:"required" json:"valid" yaml:"valid"`
}

// ValidateRESTAPIParams defines query parameters used internally by the ValidateAPI service method.
// The corresponding /rest-apis/validate path was removed from the spec; this type is preserved
// because service/api.go still uses it for identifier/name/version validation.
type ValidateRESTAPIParams struct {
	// Identifier **API Identifier** to check for existence within the organization.
	Identifier *ApiIdentifierQ `form:"identifier,omitempty" json:"identifier,omitempty" yaml:"identifier,omitempty"`

	// Name **API Name** to check for existence within the organization.
	Name *ApiNameQ `form:"name,omitempty" json:"name,omitempty" yaml:"name,omitempty"`

	// Version **API Version** to check for existence within the organization.
	Version *ApiVersionQ `form:"version,omitempty" json:"version,omitempty" yaml:"version,omitempty"`
}
