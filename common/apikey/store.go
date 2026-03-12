/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package apikey

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type APIKey struct {
	// ID of the API Key
	ID string `json:"id" yaml:"id"`
	// Name of the API key (URL-safe identifier, auto-generated, immutable)
	Name string `json:"name" yaml:"name"`
	// DisplayName is the human-readable name (user-provided, mutable)
	DisplayName string `json:"displayName" yaml:"displayName"`
	// ApiKey API key with apip_ prefix
	APIKey string `json:"apiKey" yaml:"apiKey"`
	// APIId Unique identifier of the API that the key is associated with
	APIId string `json:"apiId" yaml:"apiId"`
	// Operations List of API operations the key will have access to
	Operations string `json:"operations" yaml:"operations"`
	// Status of the API key
	Status APIKeyStatus `json:"status" yaml:"status"`
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
	// ProvisionedBy identifies the portal that created this key; nil means no restriction
	ProvisionedBy *string `json:"provisionedBy,omitempty" yaml:"provisionedBy,omitempty"`
}

// APIKeyStatus Status of the API key
type APIKeyStatus string

// ParsedAPIKey represents a parsed API key with its components
type ParsedAPIKey struct {
	APIKey string
	ID     string
}

// Defines values for APIKeyStatus.
const (
	Active  APIKeyStatus = "active"
	Expired APIKeyStatus = "expired"
	Revoked APIKeyStatus = "revoked"
)


// Common storage errors - implementation agnostic
var (
	// ErrNotFound is returned when an API key is not found
	ErrNotFound = errors.New("API key not found")

	// ErrConflict is returned when an API Key with the same name/version or key value already exists
	ErrConflict = errors.New("API key already exists")

	// ErrInvalidInput is returned when input validation fails (e.g. external key missing IndexKey)
	ErrInvalidInput = errors.New("invalid input")
)

// Singleton instance
var (
	instance *APIkeyStore
	once     sync.Once
)

// APIkeyStore holds all API keys in memory for fast access
type APIkeyStore struct {
	mu sync.RWMutex // Protects concurrent access
	// API Keys storage indexed by hash
	// Key: "API ID" → Value: map[SHA256(plain key)]*APIKey
	// Both local and external keys use the same hash-based lookup
	apiKeysByAPI map[string]map[string]*APIKey
}

// NewAPIkeyStore creates a new in-memory API key store
func NewAPIkeyStore() *APIkeyStore {
	return &APIkeyStore{
		apiKeysByAPI: make(map[string]map[string]*APIKey),
	}
}

// GetAPIkeyStoreInstance provides a shared instance of APIkeyStore
func GetAPIkeyStoreInstance() *APIkeyStore {
	once.Do(func() {
		instance = NewAPIkeyStore()
	})
	return instance
}

// StoreAPIKey stores an API key in the in-memory cache, indexed by its hash
func (aks *APIkeyStore) StoreAPIKey(apiId string, apiKey *APIKey) error {
	if apiKey == nil {
		return fmt.Errorf("API key cannot be nil")
	}

	// Normalize the API key hash value before storing
	apiKey.APIKey = strings.TrimSpace(apiKey.APIKey)

	// Validate that APIKey (hash) is non-empty
	if apiKey.APIKey == "" {
		return fmt.Errorf("%w: API key hash cannot be empty", ErrInvalidInput)
	}

	aks.mu.Lock()
	defer aks.mu.Unlock()

	// Check if an API key with the same apiId and name already exists
	existingKeys, apiIdExists := aks.apiKeysByAPI[apiId]
	var existingHash = ""

	if apiIdExists {
		for existingKeyHash, existingKey := range existingKeys {
			if existingKey.Name == apiKey.Name {
				existingHash = existingKeyHash
				break
			}
		}
	}

	if existingHash != "" {
		// Remove old hash entry and add new one (for key regeneration)
		delete(aks.apiKeysByAPI[apiId], existingHash)
	}

	// Check if this hash already exists (conflict detection)
	if _, exists := aks.apiKeysByAPI[apiId][apiKey.APIKey]; exists {
		return ErrConflict
	}

	// Initialize the map for this API ID if it doesn't exist
	if aks.apiKeysByAPI[apiId] == nil {
		aks.apiKeysByAPI[apiId] = make(map[string]*APIKey)
	}

	// Store by hash (APIKey field contains the hash)
	aks.apiKeysByAPI[apiId][apiKey.APIKey] = apiKey

	return nil
}

// ValidationOptions provides optional parameters for ValidateAPIKey.
type ValidationOptions struct {
	// ProvisionedByFilter, when non-empty, is matched against the key's ProvisionedBy field.
	// The check is skipped when this field is empty or the key carries no portal label.
	ProvisionedByFilter string
}

// ValidateAPIKey validates the provided API key against the internal APIkey store.
// Supports both local and external keys using unified hash-based lookup.
// An optional ValidationOptions value may be passed to enforce portal and target restrictions.
func (aks *APIkeyStore) ValidateAPIKey(apiId, apiOperation, operationMethod, providedAPIKey string, opts ...ValidationOptions) (bool, error) {
	aks.mu.RLock()
	defer aks.mu.RUnlock()

	// Normalize the provided API key
	providedAPIKey = strings.TrimSpace(providedAPIKey)
	if providedAPIKey == "" {
		return false, fmt.Errorf("API key is empty")
	}

	// Compute hash for lookup (hash the full API key value as-is)
	hash := ComputeAPIKeyHash(providedAPIKey)
	if hash == "" {
		return false, fmt.Errorf("failed to compute API key hash")
	}

	// Single unified O(1) lookup by hash
	targetAPIKey, exists := aks.apiKeysByAPI[apiId][hash]
	if !exists {
		return false, ErrNotFound
	}

	// Check if the API key belongs to the specified API
	if targetAPIKey.APIId != apiId {
		return false, nil
	}

	if len(opts) > 0 {
		o := opts[0]

		// provisioned_by check: only enforce when the key carries a portal label
		if targetAPIKey.ProvisionedBy != nil && *targetAPIKey.ProvisionedBy != "" {
			if o.ProvisionedByFilter == "" || *targetAPIKey.ProvisionedBy != o.ProvisionedByFilter {
				return false, nil
			}
		}
	}

	// Check if the API key is active
	if targetAPIKey.Status != Active {
		return false, nil
	}

	// Check if the API key has expired
	if targetAPIKey.Status == Expired || (targetAPIKey.ExpiresAt != nil && time.Now().After(*targetAPIKey.ExpiresAt)) {
		targetAPIKey.Status = Expired
		return false, nil
	}

    // TODO: Currently, API key creation happens only per API, not per operation, since it was decided to remove the operations field from API keys. 
	// Therefore, this implementation should include some kind of mapping so that when a policy is attached to a resource, this method  
	// can look up that mapping and perform the validation.
	return true, nil
}

// RevokeAPIKey revokes a specific API key by plain text API key value
// Supports both local and external keys using unified hash-based lookup
func (aks *APIkeyStore) RevokeAPIKey(apiId, providedAPIKey string) error {
	aks.mu.Lock()
	defer aks.mu.Unlock()

	// Normalize the provided API key
	providedAPIKey = strings.TrimSpace(providedAPIKey)
	if providedAPIKey == "" {
		return nil // Idempotent - treat empty key as already revoked
	}

	// Compute hash for lookup (hash the full API key value as-is)
	hash := ComputeAPIKeyHash(providedAPIKey)
	if hash == "" {
		return nil // Idempotent - treat invalid key as already revoked
	}

	// Single unified O(1) lookup by hash
	matchedKey, exists := aks.apiKeysByAPI[apiId][hash]
	if !exists {
		return nil // Idempotent - key doesn't exist, treat as already revoked
	}

	// Set status to revoked
	matchedKey.Status = Revoked

	// Remove from mapping
	aks.removeFromAPIMapping(apiId, hash)

	return nil
}

// RemoveAPIKeysByAPI removes all API keys for a specific API
func (aks *APIkeyStore) RemoveAPIKeysByAPI(apiId string) error {
	aks.mu.Lock()
	defer aks.mu.Unlock()

	// Simply remove the entire API ID mapping
	delete(aks.apiKeysByAPI, apiId)

	return nil
}

// ClearAll removes all API keys from the store
func (aks *APIkeyStore) ClearAll() error {
	aks.mu.Lock()
	defer aks.mu.Unlock()

	// Clear the API-specific keys map
	aks.apiKeysByAPI = make(map[string]map[string]*APIKey)

	return nil
}


// ComputeAPIKeyHash computes a SHA-256 hash of the plain-text API key for storage and lookup
// Returns the hash as hex-encoded string (64 characters)
// Normalizes the key by trimming whitespace before hashing for consistency
func ComputeAPIKeyHash(plainAPIKey string) string {
	// Normalize the API key by trimming whitespace
	trimmedAPIKey := strings.TrimSpace(plainAPIKey)
	if trimmedAPIKey == "" {
		return ""
	}

	hasher := sha256.New()
	hasher.Write([]byte(trimmedAPIKey))
	hash := hasher.Sum(nil)
	return hex.EncodeToString(hash)
}

// removeFromAPIMapping removes an API key from the API mapping by hash
func (aks *APIkeyStore) removeFromAPIMapping(apiId, hash string) {
	apiKeys, apiIdExists := aks.apiKeysByAPI[apiId]
	if apiIdExists {
		delete(apiKeys, hash)
		// clean up empty maps
		if len(aks.apiKeysByAPI[apiId]) == 0 {
			delete(aks.apiKeysByAPI, apiId)
		}
	}
}
