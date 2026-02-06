/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package dto

import (
	"time"
)

// CreateGatewayRequest represents the request body for registering a new gateway
type CreateGatewayRequest struct {
	Name              string                 `json:"name" binding:"required"`
	DisplayName       string                 `json:"displayName" binding:"required"`
	Description       string                 `json:"description,omitempty"`
	Vhost             string                 `json:"vhost" binding:"required"`
	IsCritical        bool                   `json:"isCritical,omitempty"`
	FunctionalityType string                 `json:"functionalityType" binding:"required"`
	Properties        map[string]interface{} `json:"properties,omitempty"`
}

// GatewayResponse represents a gateway in API responses
type GatewayResponse struct {
	ID                string                 `json:"id"`
	OrganizationID    string                 `json:"organizationId"`
	Name              string                 `json:"name"`
	DisplayName       string                 `json:"displayName"`
	Description       string                 `json:"description,omitempty"`
	Properties        map[string]interface{} `json:"properties,omitempty"`
	Vhost             string                 `json:"vhost"`
	IsCritical        bool      `json:"isCritical"`
	FunctionalityType string    `json:"functionalityType"`
	IsActive          bool      `json:"isActive"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// GatewayListResponse represents a paginated list of gateways (constitution-compliant)
type GatewayListResponse struct {
	Count      int               `json:"count"`      // Number of items in current response
	List       []GatewayResponse `json:"list"`       // Array of gateway objects
	Pagination Pagination        `json:"pagination"` // Pagination metadata
}

// UpdateGatewayRequest represents the request body for updating gateway details
type UpdateGatewayRequest struct {
	DisplayName *string                 `json:"displayName,omitempty"`
	Description *string                 `json:"description,omitempty"`
	IsCritical  *bool                   `json:"isCritical,omitempty"`
	Properties  *map[string]interface{} `json:"properties,omitempty"`
}

// TokenRotationResponse represents the response when rotating a gateway token
type TokenRotationResponse struct {
	ID        string    `json:"id"`    // ID of the newly generated token
	Token     string    `json:"token"` // Plain-text new authentication token
	CreatedAt time.Time `json:"createdAt"`
	Message   string    `json:"message"` // e.g., "New token generated. Old token remains active."
}

// TokenInfoResponse represents token metadata for audit purposes
type TokenInfoResponse struct {
	ID        string     `json:"id"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"createdAt"`
	RevokedAt *time.Time `json:"revokedAt,omitempty"`
}

// GatewayStatusResponse represents a lightweight gateway status for polling
type GatewayStatusResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IsActive   bool   `json:"isActive"`
	IsCritical bool   `json:"isCritical"`
}

// GatewayStatusListResponse represents a list of gateway statuses for polling
type GatewayStatusListResponse struct {
	Count      int                     `json:"count"`
	List       []GatewayStatusResponse `json:"list"`
	Pagination Pagination              `json:"pagination"`
}

// AddGatewayToAPIRequest represents the request to associate a gateway with an API
type AddGatewayToAPIRequest struct {
	GatewayID string `json:"gatewayId" binding:"required"`
}

// APIDeploymentDetails represents deployment details for an API on a gateway
type APIDeploymentDetails struct {
	DeploymentID string    `json:"deploymentId"`
	DeployedAt   time.Time `json:"deployedAt"`
}

// APIGatewayResponse represents a gateway with API association and deployment details
// This extends GatewayResponse with additional association and deployment fields
type APIGatewayResponse struct {
	GatewayResponse                       // Embedded gateway details
	AssociatedAt    time.Time             `json:"associatedAt"`
	IsDeployed      bool                  `json:"isDeployed"`
	Deployment      *APIDeploymentDetails `json:"deployment,omitempty"` // Only present when isDeployed is true
}

// APIGatewayListResponse represents a paginated list of gateways with API association and deployment details
type APIGatewayListResponse struct {
	Count      int                  `json:"count"`      // Number of items in current response
	List       []APIGatewayResponse `json:"list"`       // Array of gateway objects with deployment details
	Pagination Pagination           `json:"pagination"` // Pagination metadata
}

// GatewayArtifact represents an artifact (API, MCP, API Product) deployed to a gateway
type GatewayArtifact struct {
	ID        string    `json:"id"`
	Name      string    `json:"name,omitempty"`
	Type      string    `json:"type"`              // "API", "MCP", "API_PRODUCT"
	SubType   string    `json:"subType,omitempty"` // For APIs: "REST", "ASYNC", "GQL"
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// GatewayArtifactListResponse represents a paginated list of artifacts deployed to a gateway
type GatewayArtifactListResponse struct {
	Count      int               `json:"count"`
	List       []GatewayArtifact `json:"list"`
	Pagination Pagination        `json:"pagination"`
}
