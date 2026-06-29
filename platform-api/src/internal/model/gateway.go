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

	"platform-api/src/internal/constants"
)

// Gateway represents a registered gateway instance within an organization
type Gateway struct {
	ID                string                 `json:"id" db:"uuid"`
	OrganizationID    string                 `json:"organizationId" db:"organization_uuid"`
	Name              string                 `json:"name" db:"name"`
	Handle            string                 `json:"handle" db:"handle"`
	Description       string                 `json:"description" db:"description"`
	Properties        map[string]interface{} `json:"properties,omitempty" db:"properties"`
	Vhost             string                 `json:"vhost" db:"vhost"`
	IsCritical        bool                   `json:"isCritical" db:"is_critical"`
	FunctionalityType string                 `json:"functionalityType" db:"gateway_functionality_type"`
	Version           string                 `json:"version" db:"version"`
	IsActive          bool                   `json:"isActive" db:"is_active"`
	CreatedBy         string                 `json:"createdBy,omitempty" db:"created_by"`
	UpdatedBy         string                 `json:"updatedBy,omitempty" db:"updated_by"`
	CreatedAt         time.Time              `json:"createdAt" db:"created_at"`
	UpdatedAt         time.Time              `json:"updatedAt" db:"updated_at"`
}

// TableName returns the table name for the Gateway model
func (Gateway) TableName() string {
	return "gateways"
}

// GatewayToken represents an authentication token for a gateway
type GatewayToken struct {
	ID        string     `json:"id" db:"uuid"`
	GatewayID string     `json:"gatewayId" db:"gateway_uuid"`
	TokenHash string     `json:"-" db:"token_hash"`  // Never expose in JSON responses
	Salt      string     `json:"-" db:"salt"`        // Never expose in JSON responses
	Status    string     `json:"status" db:"status"` // "active" or "revoked"
	CreatedBy string     `json:"createdBy,omitempty" db:"created_by"`
	CreatedAt time.Time  `json:"createdAt" db:"created_at"`
	RevokedBy *string    `json:"revokedBy,omitempty" db:"revoked_by"`
	RevokedAt *time.Time `json:"revokedAt,omitempty" db:"revoked_at"`
}

// TableName returns the table name for the GatewayToken model
func (GatewayToken) TableName() string {
	return "gateway_tokens"
}

// IsActive returns true if token status is active
func (t *GatewayToken) IsActive() bool {
	return t.Status == constants.GatewayTokenStatusActive
}

// Revoke marks the token as revoked with current timestamp and actor
func (t *GatewayToken) Revoke(revokedBy string) {
	now := time.Now()
	t.Status = constants.GatewayTokenStatusRevoked
	t.RevokedAt = &now
	if revokedBy != "" {
		t.RevokedBy = &revokedBy
	}
}

// APIGatewayWithDetails represents a gateway with its association and deployment details for an API
type APIGatewayWithDetails struct {
	// Gateway information
	ID                string                 `json:"id" db:"id"`
	OrganizationID    string                 `json:"organizationId" db:"organization_id"`
	Name              string                 `json:"name" db:"name"`
	Handle            string                 `json:"handle" db:"handle"`
	Description       string                 `json:"description" db:"description"`
	Properties        map[string]interface{} `json:"properties,omitempty" db:"properties"`
	Vhost             string                 `json:"vhost" db:"vhost"`
	IsCritical        bool                   `json:"isCritical" db:"is_critical"`
	FunctionalityType string                 `json:"functionalityType" db:"functionality_type"`
	IsActive          bool                   `json:"isActive" db:"is_active"`
	CreatedAt         time.Time              `json:"createdAt" db:"created_at"`
	UpdatedAt         time.Time              `json:"updatedAt" db:"updated_at"`

	// Association information
	AssociatedAt         time.Time `json:"associatedAt" db:"associated_at"`
	AssociationUpdatedAt time.Time `json:"associationUpdatedAt" db:"association_updated_at"`

	IsDeployed bool `json:"isDeployed" db:"is_deployed"`
	// Deployment information (nullable if not deployed)
	DeploymentID *string    `json:"deploymentId,omitempty" db:"deployment_uuid"`
	DeployedAt   *time.Time `json:"deployedAt,omitempty" db:"deployed_at"`
}
