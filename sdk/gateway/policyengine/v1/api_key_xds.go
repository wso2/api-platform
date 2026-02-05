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

package policyenginev1

import (
	"time"
)

// APIKeyOperationType represents the type of API key operation
type APIKeyOperationType string

const (
	// APIKeyOperationStore represents storing/updating an API key
	APIKeyOperationStore APIKeyOperationType = "store"
	// APIKeyOperationRevoke represents revoking an API key
	APIKeyOperationRevoke APIKeyOperationType = "revoke"
	// APIKeyOperationRemoveByAPI represents removing all API keys for an API
	APIKeyOperationRemoveByAPI APIKeyOperationType = "remove_by_api"
)

// APIKeyOperation represents an API key operation to be sent via xDS
type APIKeyOperation struct {
	// Operation type: store, revoke, or remove_by_api
	Operation APIKeyOperationType `json:"operation" yaml:"operation"`

	// APIKey contains the API key data (for store operations)
	APIKey *APIKeyData `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`

	// APIId of the API associated with the operation
	APIId string `json:"apiId" yaml:"apiId"`

	// APIKeyValue for revoke operations (the actual key value to revoke)
	APIKeyValue string `json:"apiKeyValue,omitempty" yaml:"apiKeyValue,omitempty"`

	// Timestamp of the operation
	Timestamp time.Time `json:"timestamp" yaml:"timestamp"`

	// CorrelationID for tracking the operation
	CorrelationID string `json:"correlationId" yaml:"correlationId"`
}

// APIKeyData represents an API key for xDS transmission
type APIKeyData struct {
	// ID of the API Key
	ID string `json:"id" yaml:"id"`

	// Name of the API key
	Name string `json:"name" yaml:"name"`

	// APIKey value with apip_ prefix
	APIKey string `json:"apiKey" yaml:"apiKey"`

	// APIId of the API the key is associated with
	APIId string `json:"apiId" yaml:"apiId"`

	// Operations List of API operations the key will have access to (JSON array string)
	Operations string `json:"operations" yaml:"operations"`

	// Status of the API key
	Status string `json:"status" yaml:"status"`

	// CreatedAt Timestamp when the API key was generated
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`

	// CreatedBy User who created the API key
	CreatedBy string `json:"createdBy" yaml:"createdBy"`

	// UpdatedAt Timestamp when the API key was last updated
	UpdatedAt time.Time `json:"updatedAt" yaml:"updatedAt"`

	// ExpiresAt Expiration timestamp (null if no expiration)
	ExpiresAt *time.Time `json:"expiresAt" yaml:"expiresAt"`

	// Source tracking for external key support ("local" | "external")
	Source string `json:"source" yaml:"source"`

	// IndexKey Pre-computed hash for O(1) lookup (external plain text keys only)
	IndexKey string `json:"indexKey" yaml:"indexKey"`
}

// APIKeyOperationBatch represents a batch of API key operations
// This is the main resource type sent via xDS
type APIKeyOperationBatch struct {
	// Operations contains a list of API key operations
	Operations []APIKeyOperation `json:"operations" yaml:"operations"`

	// BatchID uniquely identifies this batch
	BatchID string `json:"batchId" yaml:"batchId"`

	// Version represents the version of this batch
	Version int64 `json:"version" yaml:"version"`

	// Timestamp when this batch was created
	Timestamp time.Time `json:"timestamp" yaml:"timestamp"`
}
