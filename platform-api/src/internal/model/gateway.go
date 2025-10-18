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

package model

import (
	"time"
)

// Gateway represents a registered gateway instance within an organization
type Gateway struct {
	ID             string    `json:"id" db:"uuid"`
	OrganizationID string    `json:"organizationId" db:"organization_id"`
	Name           string    `json:"name" db:"name"`
	DisplayName    string    `json:"displayName" db:"display_name"`
	CreatedAt      time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt      time.Time `json:"updatedAt" db:"updated_at"`
}

// TableName returns the table name for the Gateway model
func (Gateway) TableName() string {
	return "gateways"
}

// GatewayToken represents an authentication token for a gateway
type GatewayToken struct {
	ID        string     `json:"id" db:"uuid"`
	GatewayID string     `json:"gatewayId" db:"gateway_uuid"`
	TokenHash string     `json:"-" db:"token_hash"` // Never expose in JSON responses
	Salt      string     `json:"-" db:"salt"`       // Never expose in JSON responses
	Status    string     `json:"status" db:"status"` // "active" or "revoked"
	CreatedAt time.Time  `json:"createdAt" db:"created_at"`
	RevokedAt *time.Time `json:"revokedAt,omitempty" db:"revoked_at"` // Pointer for NULL support
}

// TableName returns the table name for the GatewayToken model
func (GatewayToken) TableName() string {
	return "gateway_tokens"
}

// IsActive returns true if token status is active
func (t *GatewayToken) IsActive() bool {
	return t.Status == "active"
}

// Revoke marks the token as revoked with current timestamp
func (t *GatewayToken) Revoke() {
	now := time.Now()
	t.Status = "revoked"
	t.RevokedAt = &now
}