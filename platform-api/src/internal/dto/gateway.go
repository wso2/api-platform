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
	OrganizationID string `json:"organizationId" binding:"required"`
	Name           string `json:"name" binding:"required"`
	DisplayName    string `json:"displayName" binding:"required"`
}

// GatewayResponse represents a gateway in API responses
type GatewayResponse struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// GatewayListResponse represents a paginated list of gateways (constitution-compliant)
type GatewayListResponse struct {
	Count      int               `json:"count"`      // Number of items in current response
	List       []GatewayResponse `json:"list"`       // Array of gateway objects
	Pagination PaginationInfo    `json:"pagination"` // Pagination metadata
}

// PaginationInfo contains pagination metadata for list responses
type PaginationInfo struct {
	Total  int `json:"total"`  // Total number of items available across all pages
	Offset int `json:"offset"` // Zero-based index of first item in current response
	Limit  int `json:"limit"`  // Maximum number of items returned per page
}

// TokenRotationResponse represents the response when rotating a gateway token
type TokenRotationResponse struct {
	TokenID   string    `json:"tokenId"`   // ID of the newly generated token
	Token     string    `json:"token"`     // Plain-text new authentication token
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