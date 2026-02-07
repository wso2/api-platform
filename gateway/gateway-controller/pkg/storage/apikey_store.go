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
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// APIKeyStore manages API keys in memory with thread-safe operations
type APIKeyStore struct {
	mu               sync.RWMutex
	apiKeys          map[string]*models.APIKey            // key: configID:APIKeyName → Value: *APIKey
	apiKeysByAPI     map[string]map[string]*models.APIKey // Key: configID → Value: map[keyID]*APIKey
	externalKeyIndex map[string]map[string]*string        // Key: configID → Value: map[indexKey]*string
	resourceVersion  int64
	logger           *slog.Logger
}

// NewAPIKeyStore creates a new API key store
func NewAPIKeyStore(logger *slog.Logger) *APIKeyStore {
	return &APIKeyStore{
		apiKeys:          make(map[string]*models.APIKey),
		apiKeysByAPI:     make(map[string]map[string]*models.APIKey),
		externalKeyIndex: make(map[string]map[string]*string),
		logger:           logger,
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

	compositeKey := GetCompositeKey(apiKey.APIId, apiKey.Name)
	if existingKeyID != "" {
		// Handle both rotation and generation scenarios for existing key name
		delete(s.apiKeys, compositeKey)
		delete(s.apiKeysByAPI[apiKey.APIId], existingKeyID)
		if apiKey.Source == "external" {
			delete(s.externalKeyIndex[apiKey.APIId], *apiKey.IndexKey)
		}
		// Store the new key (could be same ID with new value, or new ID entirely)
		s.apiKeys[compositeKey] = apiKey
		s.apiKeysByAPI[apiKey.APIId][apiKey.ID] = apiKey
		if apiKey.Source == "external" {
			s.externalKeyIndex[apiKey.APIId][*apiKey.IndexKey] = &apiKey.ID
		}
	} else {
		// Store the API key
		s.apiKeys[compositeKey] = apiKey
		s.addToAPIMapping(apiKey)
	}

	s.logger.Debug("Successfully stored API key",
		slog.String("id", apiKey.ID),
		slog.String("api_id", apiKey.APIId),
		slog.String("status", string(apiKey.Status)))

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

// Revoke marks an API key as revoked by finding it through API ID and key name lookup
func (s *APIKeyStore) Revoke(apiId, apiKeyName string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	compositeKey := GetCompositeKey(apiId, apiKeyName)

	apiKey, exists := s.apiKeys[compositeKey]
	if !exists {
		s.logger.Debug("API key ID not found for revocation",
			slog.String("api_id", apiId),
			slog.String("api_key", apiKeyName))
		return false
	}

	if apiKey != nil {
		apiKey.Status = models.APIKeyStatusRevoked

		delete(s.apiKeys, compositeKey)
		s.removeFromAPIMapping(apiKey)

		s.logger.Debug("Revoked API key",
			slog.String("id", apiKey.ID),
			slog.String("name", apiKey.Name),
			slog.String("api_id", apiKey.APIId))

		return true
	}

	s.logger.Debug("API key not found for revocation",
		slog.String("api_id", apiId),
		slog.String("api_key", apiKeyName))

	return false
}

// RemoveByAPI removes all API keys for a specific API
func (s *APIKeyStore) RemoveByAPI(apiId string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	apiKeys := s.apiKeysByAPI[apiId]
	count := len(apiKeys)

	for _, apiKey := range apiKeys {
		compositeKey := GetCompositeKey(apiKey.APIId, apiKey.Name)
		delete(s.apiKeys, compositeKey)
	}
	delete(s.apiKeysByAPI, apiId)
	delete(s.externalKeyIndex, apiId)

	s.logger.Debug("Removed API keys by API",
		slog.String("api_id", apiId),
		slog.Int("count", count))

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

	// Initialize the map for this API ID if it doesn't exist
	if s.externalKeyIndex[apiKey.APIId] == nil {
		s.externalKeyIndex[apiKey.APIId] = make(map[string]*string)
	}

	// Store by API key ID
	s.apiKeysByAPI[apiKey.APIId][apiKey.ID] = apiKey
	if apiKey.Source == "external" {
		externalKeyIndexKey := *apiKey.IndexKey
		s.externalKeyIndex[apiKey.APIId][externalKeyIndexKey] = &apiKey.ID
	}
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
		if apiKey.Source == "external" {
			externalKeyIndexKey := *apiKey.IndexKey
			delete(s.externalKeyIndex[apiKey.APIId], externalKeyIndexKey)
		}
		// clean up empty maps
		if len(s.externalKeyIndex[apiKey.APIId]) == 0 {
			delete(s.externalKeyIndex, apiKey.APIId)
		}
	}
}

// GetCompositeKey generates a composite key for storing/retrieving API keys
func GetCompositeKey(apiId, keyName string) string {
	return fmt.Sprintf("%s:%s", apiId, keyName)
}
