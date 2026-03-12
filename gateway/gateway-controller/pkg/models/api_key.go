/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
 */

package models

import (
	"time"
)

// APIKeyStatus represents the status of an API key
type APIKeyStatus string

const (
	APIKeyStatusActive  APIKeyStatus = "active"
	APIKeyStatusRevoked APIKeyStatus = "revoked"
	APIKeyStatusExpired APIKeyStatus = "expired"
)

// APIKey represents an API key for an API
type APIKey struct {
	ID           string       `json:"id" db:"id"`
	Name         string       `json:"name" db:"name"`                   // URL-safe identifier (auto-generated, immutable)
	APIKey       string       `json:"apiKey" db:"api_key"`              // Stores hashed API key
	MaskedAPIKey string       `json:"maskedApiKey" db:"masked_api_key"` // Stores masked API key for display
	PlainAPIKey  string       `json:"-" db:"-"`                         // Temporary field for plain API key (not persisted)
	APIId        string       `json:"apiId" db:"apiId"`
	Status       APIKeyStatus `json:"status" db:"status"`
	CreatedAt    time.Time    `json:"createdAt" db:"created_at"`
	CreatedBy    string       `json:"createdBy" db:"created_by"`
	UpdatedAt    time.Time    `json:"updatedAt" db:"updated_at"`
	ExpiresAt    *time.Time   `json:"expiresAt" db:"expires_at"`

	// Source tracking for external key support
	Source        string  `json:"source" db:"source"`                 // "local" | "external"
	ExternalRefId *string `json:"externalRefId" db:"external_ref_id"` // Cloud APIM key ID or other external reference

	// APIKeyUUID is the UUID v7 from platform API, used to correlate keys across systems.
	// Populated from the apikey.created event; generated locally if not provided.
	APIKeyUUID *string `json:"apiKeyUuid" db:"api_key_uuid"`

	// ProvisionedBy identifies the developer portal that provisioned this key; nil if not provided
	ProvisionedBy *string `json:"provisionedBy,omitempty" db:"provisioned_by"`

	// AllowedTargets is a comma-separated list of allowed gateways; defaults to 'ALL'
	AllowedTargets string `json:"allowedTargets" db:"allowed_targets"`
}

// IsValid checks if the API key is valid (active and not expired)
func (ak *APIKey) IsValid() bool {
	if ak.Status != APIKeyStatusActive {
		return false
	}

	if ak.ExpiresAt != nil && time.Now().After(*ak.ExpiresAt) {
		return false
	}

	return true
}

// IsExpired checks if the API key has expired
func (ak *APIKey) IsExpired() bool {
	return ak.ExpiresAt != nil && time.Now().After(*ak.ExpiresAt)
}
