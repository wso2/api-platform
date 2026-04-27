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
	// ApplicationID identifies the mapped application for this API key when available.
	ApplicationID string `json:"applicationId,omitempty" yaml:"applicationId,omitempty"`
	// ApplicationName is the mapped application name for this API key when available.
	ApplicationName string `json:"applicationName,omitempty" yaml:"applicationName,omitempty"`
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
	// Issuer identifies the portal that created this key; nil means no restriction
	Issuer *string `json:"issuer,omitempty" yaml:"issuer,omitempty"`
	// AllowedTargets is a comma-separated list of allowed gateways; "ALL" or "" means unrestricted
	AllowedTargets string `json:"allowedTargets" yaml:"allowedTargets"`
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

func cloneAPIKey(apiKey *APIKey) *APIKey {
	if apiKey == nil {
		return nil
	}

	clonedAPIKey := *apiKey

	if apiKey.ExpiresAt != nil {
		expiresAt := *apiKey.ExpiresAt
		clonedAPIKey.ExpiresAt = &expiresAt
	}

	if apiKey.Issuer != nil {
		issuer := *apiKey.Issuer
		clonedAPIKey.Issuer = &issuer
	}

	return &clonedAPIKey
}

// ResolveValidatedAPIKey validates the provided API key and returns the matched API key object.
// It returns (nil, nil) when a key is found but does not satisfy validation constraints.
// issuer, when non-empty, restricts validation to keys from a specific portal.
func (aks *APIkeyStore) ResolveValidatedAPIKey(apiId, apiOperation, operationMethod, providedAPIKey string, issuer ...string) (*APIKey, error) {
	// Normalize the provided API key.
	providedAPIKey = strings.TrimSpace(providedAPIKey)
	if providedAPIKey == "" {
		return nil, fmt.Errorf("API key is empty")
	}

	// Compute hash for lookup (hash the full API key value as-is).
	hash := ComputeAPIKeyHash(providedAPIKey)
	if hash == "" {
		return nil, fmt.Errorf("failed to compute API key hash")
	}

	aks.mu.RLock()

	// Single unified O(1) lookup by hash.
	targetAPIKey, exists := aks.apiKeysByAPI[apiId][hash]
	if !exists {
		aks.mu.RUnlock()
		return nil, ErrNotFound
	}

	clonedAPIKey := cloneAPIKey(targetAPIKey)
	aks.mu.RUnlock()

	// Check if the API key belongs to the specified API.
	if clonedAPIKey.APIId != apiId {
		return nil, nil
	}

	// issuer check: only enforce when issuer is provided and non-empty.
	if len(issuer) > 0 && issuer[0] != "" {
		if clonedAPIKey.Issuer == nil || *clonedAPIKey.Issuer != issuer[0] {
			return nil, nil
		}
	}

	// Check if the API key is active.
	if clonedAPIKey.Status != Active {
		return nil, nil
	}

	// Check if the API key has expired.
	if clonedAPIKey.Status == Expired || (clonedAPIKey.ExpiresAt != nil && time.Now().After(*clonedAPIKey.ExpiresAt)) {
		return nil, nil
	}

	// TODO: Currently, API key creation happens only per API, not per operation, since it was decided to remove the operations field from API keys.
	// Therefore, this implementation should include some kind of mapping so that when a policy is attached to a resource, this method
	// can look up that mapping and perform the validation.
	return clonedAPIKey, nil
}

// GetAPIKeyByID retrieves an API key by API ID and key ID.
func (aks *APIkeyStore) GetAPIKeyByID(apiId, keyID string) (*APIKey, error) {
	if keyID == "" {
		return nil, ErrNotFound
	}

	aks.mu.RLock()

	apiKeysByHash, ok := aks.apiKeysByAPI[apiId]
	if !ok {
		aks.mu.RUnlock()
		return nil, ErrNotFound
	}

	for _, apiKey := range apiKeysByHash {
		if apiKey != nil && apiKey.ID == keyID {
			clonedAPIKey := cloneAPIKey(apiKey)
			aks.mu.RUnlock()
			return clonedAPIKey, nil
		}
	}
	aks.mu.RUnlock()

	return nil, ErrNotFound
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

// ReplaceAll atomically replaces all API keys in the store with the provided snapshot.
func (aks *APIkeyStore) ReplaceAll(newMap map[string]map[string]*APIKey) error {
	replacement := make(map[string]map[string]*APIKey, len(newMap))
	for apiId, apiKeys := range newMap {
		if len(apiKeys) == 0 {
			continue
		}

		clonedKeys := make(map[string]*APIKey, len(apiKeys))
		for hash, apiKey := range apiKeys {
			if apiKey == nil {
				return fmt.Errorf("API key cannot be nil")
			}

			normalizedHash := strings.TrimSpace(hash)
			clonedAPIKey := cloneAPIKey(apiKey)
			clonedAPIKey.APIKey = strings.TrimSpace(clonedAPIKey.APIKey)
			if clonedAPIKey.APIKey == "" {
				return fmt.Errorf("%w: API key hash cannot be empty", ErrInvalidInput)
			}
			if normalizedHash == "" {
				normalizedHash = clonedAPIKey.APIKey
			}
			if normalizedHash != clonedAPIKey.APIKey {
				return fmt.Errorf("%w: API key map hash does not match API key hash", ErrInvalidInput)
			}
			if _, exists := clonedKeys[normalizedHash]; exists {
				return ErrConflict
			}

			clonedKeys[normalizedHash] = clonedAPIKey
		}

		replacement[apiId] = clonedKeys
	}

	aks.mu.Lock()
	defer aks.mu.Unlock()
	aks.apiKeysByAPI = replacement

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
