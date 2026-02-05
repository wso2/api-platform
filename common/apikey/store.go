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
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
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
	// IndexKey Pre-computed hash for O(1) lookup (external plain text keys only)
	IndexKey string `json:"indexKey" yaml:"indexKey"`
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

const APIKeySeparator = "_"

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
	// API Keys storage
	apiKeysByAPI map[string]map[string]*APIKey // Key: "API ID" → Value: map[API key ID]*APIKey
	// Fast lookup index for external keys: Key: "API ID:SHA256(plain key)" → Value: API key ID
	// This avoids O(n) iteration through all keys for external key validation
	externalKeyIndex map[string]map[string]*string
}

// NewAPIkeyStore creates a new in-memory API key store
func NewAPIkeyStore() *APIkeyStore {
	return &APIkeyStore{
		apiKeysByAPI:     make(map[string]map[string]*APIKey),
		externalKeyIndex: make(map[string]map[string]*string),
	}
}

// GetAPIkeyStoreInstance provides a shared instance of APIkeyStore
func GetAPIkeyStoreInstance() *APIkeyStore {
	once.Do(func() {
		instance = NewAPIkeyStore()
	})
	return instance
}

// StoreAPIKey stores an API key in the in-memory cache
func (aks *APIkeyStore) StoreAPIKey(apiId string, apiKey *APIKey) error {
	if apiKey == nil {
		return fmt.Errorf("API key cannot be nil")
	}

	// Require non-empty IndexKey for external keys before any writes (no replacement from hashed APIKey)
	if apiKey.Source == "external" && strings.TrimSpace(apiKey.IndexKey) == "" {
		return fmt.Errorf("%w: external API key requires non-empty IndexKey", ErrInvalidInput)
	}

	aks.mu.Lock()
	defer aks.mu.Unlock()

	// Check if an API key with the same apiId and name already exists
	existingKeys, apiIdExists := aks.apiKeysByAPI[apiId]
	var existingKeyID = ""

	if apiIdExists {
		for id, existingKey := range existingKeys {
			if existingKey.Name == apiKey.Name {
				existingKeyID = id
				break
			}
		}
	}

	if existingKeyID != "" {
		// Remove old external key index entry if it exists (cleanup only; use IndexKey or compute from old key)
		oldKey := aks.apiKeysByAPI[apiId][existingKeyID]
		if oldKey != nil && oldKey.Source == "external" && aks.externalKeyIndex[apiId] != nil {
			oldIndexKey := oldKey.IndexKey
			if oldIndexKey == "" {
				oldIndexKey = computeExternalKeyIndexKey(oldKey.APIKey)
			}
			if oldIndexKey != "" {
				delete(aks.externalKeyIndex[apiId], oldIndexKey)
			}
		}

		// Update the existing entry in apiKeysByAPI
		aks.apiKeysByAPI[apiId][existingKeyID] = apiKey
	} else {
		// Insert new API key
		// Check if API key ID already exists
		if _, exists := aks.apiKeysByAPI[apiId][apiKey.ID]; exists {
			return ErrConflict
		}

		// Initialize the map for this API ID if it doesn't exist
		if aks.apiKeysByAPI[apiId] == nil {
			aks.apiKeysByAPI[apiId] = make(map[string]*APIKey)
		}

		// Store by API key value
		aks.apiKeysByAPI[apiId][apiKey.ID] = apiKey
	}

	// For external keys with non-empty IndexKey, add to fast lookup index (never insert empty index entry)
	if apiKey.Source == "external" && apiKey.IndexKey != "" {
		if aks.externalKeyIndex[apiId] == nil {
			aks.externalKeyIndex[apiId] = make(map[string]*string)
		}
		aks.externalKeyIndex[apiId][apiKey.IndexKey] = &apiKey.ID
	}

	return nil
}

// ValidateAPIKey validates the provided API key against the internal APIkey store
// Supports both local keys (with format: key_id) and external keys (any format)
func (aks *APIkeyStore) ValidateAPIKey(apiId, apiOperation, operationMethod, providedAPIKey string) (bool, error) {
	aks.mu.Lock()
	defer aks.mu.Unlock()

	var targetAPIKey *APIKey

	// Try to parse as local key (format: key_id)
	parsedAPIkey, ok := parseAPIKey(providedAPIKey)
	if ok {
		// Optimized O(1) lookup for local keys using ID
		apiKey, exists := aks.apiKeysByAPI[apiId][parsedAPIkey.ID]
		if exists && apiKey.Source == "local" && compareAPIKeys(parsedAPIkey.APIKey, apiKey.APIKey) {
			targetAPIKey = apiKey
		}
	}

	// If not found via local key lookup, try external key index for O(1) lookup
	if targetAPIKey == nil {
		// Compute the index key for external key lookup
		indexKey := computeExternalKeyIndexKey(providedAPIKey)
		if indexKey == "" {
			return false, fmt.Errorf("API key is empty")
		}
		trimmedAPIKey := strings.TrimSpace(providedAPIKey)
		keyID, exists := aks.externalKeyIndex[apiId][indexKey]
		if exists {
			// Found in index, retrieve the key
			if apiKey, ok := aks.apiKeysByAPI[apiId][*keyID]; ok {
				if apiKey.Source == "external" && compareAPIKeys(trimmedAPIKey, apiKey.APIKey) {
					targetAPIKey = apiKey
				}
			}
		}
	}

	if targetAPIKey == nil {
		return false, ErrNotFound
	}

	// Check if the API key belongs to the specified API
	if targetAPIKey.APIId != apiId {
		return false, nil
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

	// Check if the API key has access to the requested operation
	// Operations is a JSON string array of allowed operations in format "METHOD path"
	// Example: ["GET /{country_code}/{city}", "POST /data"], ["*"] for allow all operations
	var operations []string
	if err := json.Unmarshal([]byte(targetAPIKey.Operations), &operations); err != nil {
		return false, fmt.Errorf("invalid operations format: %w", err)
	}

	// Check if wildcard is present
	for _, op := range operations {
		if strings.TrimSpace(op) == "*" {
			return true, nil
		}
	}

	// Check if the requested operation is in the allowed operations list
	requestedOperation := fmt.Sprintf("%s %s", operationMethod, apiOperation)
	for _, op := range operations {
		if strings.TrimSpace(op) == requestedOperation {
			return true, nil
		}
	}

	// Operation not found in allowed list
	return false, nil
}

// RevokeAPIKey revokes a specific API key by plain text API key value
// Supports both local keys (with format: key_id) and external keys (any format)
func (aks *APIkeyStore) RevokeAPIKey(apiId, providedAPIKey string) error {
	aks.mu.Lock()
	defer aks.mu.Unlock()

	var matchedKey *APIKey


	// Try to parse as local key (format: key_id); empty Source treated as "local"
	parsedAPIkey, ok := parseAPIKey(providedAPIKey)
	if ok {
		apiKey, exists := aks.apiKeysByAPI[apiId][parsedAPIkey.ID]
		if exists && apiKey.Source == "local" && compareAPIKeys(parsedAPIkey.APIKey, apiKey.APIKey) {
			matchedKey = apiKey
		}
	}

	// If not found via local key lookup, try external key index for O(1) lookup
	if matchedKey == nil {
		indexKey := computeExternalKeyIndexKey(providedAPIKey)
		if keyID, exists := aks.externalKeyIndex[apiId][indexKey]; exists {
			if apiKey, ok := aks.apiKeysByAPI[apiId][*keyID]; ok {
				if apiKey.Source == "external" && compareAPIKeys(providedAPIKey, apiKey.APIKey) {
					matchedKey = apiKey
				}
			}
		}
	}

	// If the API key doesn't exist, treat revocation as successful (idempotent operation)
	if matchedKey == nil {
		return nil
	}

	// Set status to revoked
	matchedKey.Status = Revoked

	aks.removeFromAPIMapping(matchedKey)

	return nil
}

// RemoveAPIKeysByAPI removes all API keys for a specific API
func (aks *APIkeyStore) RemoveAPIKeysByAPI(apiId string) error {
	aks.mu.Lock()
	defer aks.mu.Unlock()

	apiKeys, exists := aks.apiKeysByAPI[apiId]
	if !exists {
		return nil // No keys to remove
	}

	// Remove from external key index
	for _, apiKey := range apiKeys {
		if apiKey.Source == "external" {
			var indexKey string
			if apiKey.IndexKey != "" {
				indexKey = apiKey.IndexKey
			} else {
				indexKey = computeExternalKeyIndexKey(apiKey.APIKey)
				if indexKey == "" {
					return fmt.Errorf("failed to compute index key")
				}
			}
			delete(aks.externalKeyIndex[apiKey.APIId], indexKey)
		}
	}

	// Remove from API-specific map
	delete(aks.apiKeysByAPI, apiId)

	return nil
}

// ClearAll removes all API keys from the store
func (aks *APIkeyStore) ClearAll() error {
	aks.mu.Lock()
	defer aks.mu.Unlock()

	// Clear the API-specific keys map
	aks.apiKeysByAPI = make(map[string]map[string]*APIKey)
	// Clear the external key index
	aks.externalKeyIndex = make(map[string]map[string]*string)

	return nil
}

// compareAPIKeys compares API keys for external use
// Returns true if the plain API key matches the hash, false otherwise
// If hashing is disabled, performs plain text comparison
func compareAPIKeys(providedAPIKey, storedAPIKey string) bool {
	if providedAPIKey == "" || storedAPIKey == "" {
		return false
	}

	// Check if it's an SHA-256 hash (format: $sha256$<salt_hex>$<hash_hex>)
	if strings.HasPrefix(storedAPIKey, "$sha256$") {
		return compareSHA256Hash(providedAPIKey, storedAPIKey)
	}

	// Check if it's a bcrypt hash (starts with $2a$, $2b$, or $2y$)
	if strings.HasPrefix(storedAPIKey, "$2a$") ||
		strings.HasPrefix(storedAPIKey, "$2b$") ||
		strings.HasPrefix(storedAPIKey, "$2y$") {
		return compareBcryptHash(providedAPIKey, storedAPIKey)
	}

	// Check if it's an Argon2id hash
	if strings.HasPrefix(storedAPIKey, "$argon2id$") {
		err := compareArgon2id(providedAPIKey, storedAPIKey)
		return err == nil
	}

	// If no hash format is detected and hashing is enabled, try plain text comparison as fallback
	// This handles migration scenarios where some keys might still be stored as plain text
	return subtle.ConstantTimeCompare([]byte(providedAPIKey), []byte(storedAPIKey)) == 1
}

// compareSHA256Hash validates an encoded SHA-256 hash and compares it to the provided password.
// Expected format: $sha256$<salt_hex>$<hash_hex>
// Returns true if the plain API key matches the hash, false otherwise
func compareSHA256Hash(apiKey, encoded string) bool {
	if apiKey == "" || encoded == "" {
		return false
	}

	// Parse the hash format: $sha256$<salt_hex>$<hash_hex>
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[1] != "sha256" {
		return false
	}

	// Decode salt and hash from hex
	salt, err := hex.DecodeString(parts[2])
	if err != nil {
		return false
	}

	storedHash, err := hex.DecodeString(parts[3])
	if err != nil {
		return false
	}

	// Compute hash of the provided key with the stored salt
	hasher := sha256.New()
	hasher.Write([]byte(apiKey))
	hasher.Write(salt)
	computedHash := hasher.Sum(nil)

	// Constant-time comparison
	return subtle.ConstantTimeCompare(computedHash, storedHash) == 1
}

// compareBcryptHash validates an encoded bcrypt hash and compares it to the provided password.
// Returns true if the plain API key matches the hash, false otherwise
func compareBcryptHash(apiKey, encoded string) bool {
	if apiKey == "" || encoded == "" {
		return false
	}

	// Compare the provided key with the stored bcrypt hash
	err := bcrypt.CompareHashAndPassword([]byte(encoded), []byte(apiKey))
	return err == nil
}

// compareArgon2id parses an encoded Argon2id hash and compares it to the provided password.
// Expected format: $argon2id$v=19$m=<m>,t=<t>,p=<p>$<salt_b64>$<hash_b64>
func compareArgon2id(apiKey, encoded string) error {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return fmt.Errorf("invalid argon2id hash format")
	}

	// parts[2] -> v=19
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return err
	}
	if version != argon2.Version {
		return fmt.Errorf("unsupported argon2 version: %d", version)
	}

	// parts[3] -> m=<m>,t=<t>,p=<p>
	var mem uint32
	var iters uint32
	var threads uint8
	var t, m, p uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return err
	}
	mem = m
	iters = t
	threads = uint8(p)

	// decode salt and hash (try RawStd then Std)
	salt, err := decodeBase64(parts[4])
	if err != nil {
		return err
	}
	hash, err := decodeBase64(parts[5])
	if err != nil {
		return err
	}

	derived := argon2.IDKey([]byte(apiKey), salt, iters, mem, threads, uint32(len(hash)))
	if subtle.ConstantTimeCompare(derived, hash) == 1 {
		return nil
	}
	return errors.New("API key mismatch")
}

// decodeBase64 decodes a base64 string, trying RawStdEncoding first, then StdEncoding
func decodeBase64(s string) ([]byte, error) {
	b, err := base64.RawStdEncoding.DecodeString(s)
	if err == nil {
		return b, nil
	}
	// try StdEncoding as a fallback
	return base64.StdEncoding.DecodeString(s)
}

// parseAPIKey splits an API key value into its key and ID components
func parseAPIKey(value string) (ParsedAPIKey, bool) {
	idx := strings.LastIndex(value, APIKeySeparator)
	if idx <= 0 || idx == len(value)-1 {
		return ParsedAPIKey{}, false
	}

	apiKey := value[:idx]
	encodedID := value[idx+1:]

	// The ID is already base64url encoded (22 chars)
	// with underscores replaced by tildes (~)
	return ParsedAPIKey{
		APIKey: apiKey,
		ID:     encodedID, // Use the encoded ID directly (contains ~ instead of _)
	}, true
}

// computeExternalKeyIndexKey computes a SHA-256 hash of the plain-text API key for fast lookup
// Returns the index key as "hash_hex" (SHA-256 of the plain key)
func computeExternalKeyIndexKey(plainAPIKey string) string {
	trimmedAPIKey := strings.TrimSpace(plainAPIKey)
	if trimmedAPIKey == "" {
		return ""
	}

	hasher := sha256.New()
	hasher.Write([]byte(trimmedAPIKey))
	hash := hasher.Sum(nil)
	return hex.EncodeToString(hash)
}

// removeFromAPIMapping removes an API key from the API mapping
func (aks *APIkeyStore) removeFromAPIMapping(apiKey *APIKey) {
	apiKeys, apiIdExists := aks.apiKeysByAPI[apiKey.APIId]
	if apiIdExists {
		delete(apiKeys, apiKey.ID)
		// clean up empty maps
		if len(aks.apiKeysByAPI[apiKey.APIId]) == 0 {
			delete(aks.apiKeysByAPI, apiKey.APIId)
		}
	}

	// Remove from external key index if it's an external key
	if apiKey.Source == "external" {
		if aks.externalKeyIndex[apiKey.APIId] == nil {
			return
		}
		var indexKey string
		if apiKey.IndexKey != "" {
			indexKey = apiKey.IndexKey
		} else {
			indexKey = computeExternalKeyIndexKey(apiKey.APIKey)
			if indexKey == "" {
				return
			}
		}
		delete(aks.externalKeyIndex[apiKey.APIId], indexKey)
	}
}
