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
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
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
	apiKeys         map[string]*models.APIKey   // key: API key ID
	apiKeysByAPI    map[string][]*models.APIKey // key: API ID
	resourceVersion int64
	logger          *zap.Logger
}

// NewAPIKeyStore creates a new API key store
func NewAPIKeyStore(logger *zap.Logger) *APIKeyStore {
	return &APIKeyStore{
		apiKeys:      make(map[string]*models.APIKey),
		apiKeysByAPI: make(map[string][]*models.APIKey),
		logger:       logger,
	}
}

// Store adds or updates an API key
func (s *APIKeyStore) Store(apiKey *models.APIKey) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove old entry if updating
	if existing, exists := s.apiKeys[apiKey.ID]; exists {
		s.removeFromAPIMapping(existing)
	}

	// Store the API key
	s.apiKeys[apiKey.ID] = apiKey
	s.addToAPIMapping(apiKey)

	s.logger.Debug("Stored API key",
		zap.String("id", apiKey.ID),
		zap.String("api_id", apiKey.APIId),
		zap.String("status", string(apiKey.Status)))
}

// GetByID retrieves an API key by its ID
func (s *APIKeyStore) GetByID(id string) (*models.APIKey, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	apiKey, exists := s.apiKeys[id]
	return apiKey, exists
}

// GetByAPI retrieves all API keys for a specific API
func (s *APIKeyStore) GetByAPI(apiId string) []*models.APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()

	apiKeys := s.apiKeysByAPI[apiId]
	// Return a copy to avoid external modification
	result := make([]*models.APIKey, len(apiKeys))
	copy(result, apiKeys)
	return result
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
func (s *APIKeyStore) Revoke(apiId, plainAPIKeyValue string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get all API keys for the specified API
	apiKeys, exists := s.apiKeysByAPI[apiId]
	if !exists {
		s.logger.Debug("No API keys found for API",
			zap.String("api_id", apiId))
		return false
	}

	// Find the API key by comparing plain text key against stored hashes
	for _, apiKey := range apiKeys {
		// Compare plain text key with stored hashed key using Argon2id
		err := compareArgon2id(plainAPIKeyValue, apiKey.APIKey)
		if err == nil {
			// Hash matches - this is our target API key
			apiKey.Status = models.APIKeyStatusRevoked

			s.logger.Debug("Revoked API key",
				zap.String("id", apiKey.ID),
				zap.String("name", apiKey.Name),
				zap.String("api_id", apiKey.APIId))

			return true
		}
	}

	s.logger.Debug("API key not found for revocation",
		zap.String("api_id", apiId))

	return false
}

// RemoveByID removes an API key by its ID
func (s *APIKeyStore) RemoveByID(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	apiKey, exists := s.apiKeys[id]
	if !exists {
		return false
	}

	delete(s.apiKeys, id)
	s.removeFromAPIMapping(apiKey)

	s.logger.Debug("Removed API key",
		zap.String("id", id),
		zap.String("api_id", apiKey.APIId))

	return true
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
	apiKeys := s.apiKeysByAPI[apiKey.APIId]
	s.apiKeysByAPI[apiKey.APIId] = append(apiKeys, apiKey)
}

// removeFromAPIMapping removes an API key from the API mapping
func (s *APIKeyStore) removeFromAPIMapping(apiKey *models.APIKey) {
	apiKeys := s.apiKeysByAPI[apiKey.APIId]
	for i, ak := range apiKeys {
		if ak.ID == apiKey.ID {
			// Remove the element at index i
			s.apiKeysByAPI[apiKey.APIId] = append(apiKeys[:i], apiKeys[i+1:]...)
			break
		}
	}

	// If no API keys left for this API, remove the mapping
	if len(s.apiKeysByAPI[apiKey.APIId]) == 0 {
		delete(s.apiKeysByAPI, apiKey.APIId)
	}
}

// compareArgon2id validates a plain text key against an Argon2id hash
func compareArgon2id(apiKey, hashedAPIKey string) error {
	if apiKey == "" {
		return fmt.Errorf("plain API key cannot be empty")
	}
	if hashedAPIKey == "" {
		return fmt.Errorf("hashed API key cannot be empty")
	}

	parts := strings.Split(hashedAPIKey, "$")
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
