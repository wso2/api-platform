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
