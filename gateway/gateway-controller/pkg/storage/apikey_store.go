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

package storage

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"go.uber.org/zap"
	"golang.org/x/crypto/argon2"
)

// APIKeyStore manages API keys in memory with thread-safe operations
type APIKeyStore struct {
	mu              sync.RWMutex
	apiKeys         map[string]*models.APIKey            // key: API key ID
	apiKeysByAPI    map[string]map[string]*models.APIKey // Key: configID → Value: map[keyID]*APIKey
	resourceVersion int64
	logger          *zap.Logger
}

// NewAPIKeyStore creates a new API key store
func NewAPIKeyStore(logger *zap.Logger) *APIKeyStore {
	return &APIKeyStore{
		apiKeys:      make(map[string]*models.APIKey),
		apiKeysByAPI: make(map[string]map[string]*models.APIKey),
		logger:       logger,
	}
}

// Store adds or updates an API key
func (s *APIKeyStore) Store(apiKey *models.APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if an API key with the same APIId and name already exists
	existingKeys, apiIdExists := s.apiKeysByAPI[apiKey.APIId]
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
		// Handle both rotation and generation scenarios for existing key name
		delete(s.apiKeys, existingKeyID)
		delete(s.apiKeysByAPI[apiKey.APIId], existingKeyID)
		// Store the new key (could be same ID with new value, or new ID entirely)
		s.apiKeys[apiKey.ID] = apiKey
		s.apiKeysByAPI[apiKey.APIId][apiKey.ID] = apiKey
	} else {
		// Store the API key
		s.apiKeys[apiKey.ID] = apiKey
		s.addToAPIMapping(apiKey)
	}

	s.logger.Debug("Successfully stored API key",
		zap.String("id", apiKey.ID),
		zap.String("api_id", apiKey.APIId),
		zap.String("status", string(apiKey.Status)))

	return nil
}

// GetAll retrieves all API keys
func (s *APIKeyStore) GetAll() []*models.APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*models.APIKey, 0, len(s.apiKeys))
	for _, apiKey := range s.apiKeys {
		result = append(result, apiKey)
	}
	return result
}

// Revoke marks an API key as revoked by finding it through hash comparison
func (s *APIKeyStore) Revoke(apiId, apiKeyID, plainAPIKeyValue string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	apiKey, exists := s.apiKeys[apiKeyID]
	if !exists {
		s.logger.Debug("API key ID not found for revocation",
			zap.String("api_id", apiId),
			zap.String("api_key_id", apiKeyID))
		return false
	}

	if apiKey != nil {
		if compareAPIKeys(plainAPIKeyValue, apiKey.APIKey) {
			// Hash matches - this is our target API key
			apiKey.Status = models.APIKeyStatusRevoked

			delete(s.apiKeys, apiKey.ID)
			s.removeFromAPIMapping(apiKey)

			s.logger.Debug("Revoked API key",
				zap.String("id", apiKey.ID),
				zap.String("name", apiKey.Name),
				zap.String("api_id", apiKey.APIId))

			return true
		}
	}

	s.logger.Debug("API key not found for revocation",
		zap.String("api_id", apiId),
		zap.String("api_key_id", apiKeyID))

	return false
}

// RemoveByAPI removes all API keys for a specific API
func (s *APIKeyStore) RemoveByAPI(apiId string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	apiKeys := s.apiKeysByAPI[apiId]
	count := len(apiKeys)

	for _, apiKey := range apiKeys {
		delete(s.apiKeys, apiKey.ID)
	}
	delete(s.apiKeysByAPI, apiId)

	s.logger.Debug("Removed API keys by API",
		zap.String("api_id", apiId),
		zap.Int("count", count))

	return count
}

// Count returns the total number of API keys
func (s *APIKeyStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.apiKeys)
}

// IncrementResourceVersion increments and returns the resource version
func (s *APIKeyStore) IncrementResourceVersion() int64 {
	return atomic.AddInt64(&s.resourceVersion, 1)
}

// GetResourceVersion returns the current resource version
func (s *APIKeyStore) GetResourceVersion() int64 {
	return atomic.LoadInt64(&s.resourceVersion)
}

// addToAPIMapping adds an API key to the API mapping
func (s *APIKeyStore) addToAPIMapping(apiKey *models.APIKey) {
	// Initialize the map for this API ID if it doesn't exist
	if s.apiKeysByAPI[apiKey.APIId] == nil {
		s.apiKeysByAPI[apiKey.APIId] = make(map[string]*models.APIKey)
	}

	// Store by API key ID
	s.apiKeysByAPI[apiKey.APIId][apiKey.ID] = apiKey
}

// removeFromAPIMapping removes an API key from the API mapping
func (s *APIKeyStore) removeFromAPIMapping(apiKey *models.APIKey) {
	apiKeys, apiIdExists := s.apiKeysByAPI[apiKey.APIId]
	if apiIdExists {
		delete(apiKeys, apiKey.ID)
		// clean up empty maps
		if len(s.apiKeysByAPI[apiKey.APIId]) == 0 {
			delete(s.apiKeysByAPI, apiKey.APIId)
		}
	}
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
// Expected format: $argon2id$v=19$m=65536,t=3,p=4$<salt_b64>$<hash_b64>
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

	// parts[3] -> m=65536,t=3,p=4
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
