/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package api

import (
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// This file holds component types that are valid schemas in openapi.yaml but are
// dropped by oapi-codegen v2.5.1 on this OpenAPI 3.1 spec (it prints a 3.1
// warning during `make generate`). Keeping them here — outside the generated
// file — keeps `make generate` reproducible: regenerating drops them from
// generated.go, and these hand-maintained definitions keep the package
// compiling. Remove this file once the codegen fully supports OpenAPI 3.1.

// ApiIdentifierQ API Identifier query parameter.
type ApiIdentifierQ = string

// GatewayStatusResponse defines model for GatewayStatusResponse.
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

// GatewayStatusListResponse defines model for GatewayStatusListResponse.
type GatewayStatusListResponse struct {
	// Count Number of items in current response
	Count      int                     `binding:"required" json:"count" yaml:"count"`
	List       []GatewayStatusResponse `binding:"required" json:"list" yaml:"list"`
	Pagination Pagination              `json:"pagination" yaml:"pagination"`
}

// RESTAPIValidationResponse defines model for RESTAPIValidationResponse.
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

// UnpublishFromDevPortalRequest defines model for UnpublishFromDevPortalRequest.
type UnpublishFromDevPortalRequest struct {
	// DevPortalUuid UUID of the DevPortal to unpublish from
	DevPortalUuid openapi_types.UUID `binding:"required" json:"devPortalUuid" yaml:"devPortalUuid"`
}

// UnpublishRESTAPIFromDevPortalJSONRequestBody defines body for UnpublishRESTAPIFromDevPortal.
type UnpublishRESTAPIFromDevPortalJSONRequestBody = UnpublishFromDevPortalRequest

// UnpublishWebBrokerAPIFromDevPortalJSONRequestBody defines body for UnpublishWebBrokerAPIFromDevPortal.
type UnpublishWebBrokerAPIFromDevPortalJSONRequestBody = UnpublishFromDevPortalRequest

// UnpublishWebSubAPIFromDevPortalJSONRequestBody defines body for UnpublishWebSubAPIFromDevPortal.
type UnpublishWebSubAPIFromDevPortalJSONRequestBody = UnpublishFromDevPortalRequest

// ValidateRESTAPIParams defines parameters for ValidateRESTAPI.
type ValidateRESTAPIParams struct {
	// Identifier **API Identifier** to check for existence within the organization.
	Identifier *ApiIdentifierQ `form:"identifier,omitempty" json:"identifier,omitempty" yaml:"identifier,omitempty"`

	// Name **API Name** to check for existence within the organization.
	Name *ApiNameQ `form:"name,omitempty" json:"name,omitempty" yaml:"name,omitempty"`

	// Version **API Version** to check for existence within the organization.
	Version *ApiVersionQ `form:"version,omitempty" json:"version,omitempty" yaml:"version,omitempty"`
}
