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
	"sync"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// PolicyStore provides thread-safe in-memory storage for policy configurations
type PolicyStore struct {
	mu              sync.RWMutex
	policies        map[string]*models.StoredPolicyConfig // ID -> PolicyConfig
	byCompositeKey  map[string]*models.StoredPolicyConfig // "api_name:version:context" -> PolicyConfig
	resourceVersion int64
}

// NewPolicyStore creates a new policy store
func NewPolicyStore() *PolicyStore {
	return &PolicyStore{
		policies:        make(map[string]*models.StoredPolicyConfig),
		byCompositeKey:  make(map[string]*models.StoredPolicyConfig),
		resourceVersion: 0,
	}
}

// Set stores or updates a policy configuration
func (s *PolicyStore) Set(policy *models.StoredPolicyConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	compositeKey := policy.CompositeKey()

	// Check for duplicate composite key (different ID, same api_name:version:context)
	if existing, exists := s.byCompositeKey[compositeKey]; exists && existing.ID != policy.ID {
		return fmt.Errorf("policy configuration already exists with key %s (ID: %s)", compositeKey, existing.ID)
	}

	// Remove old composite key if ID exists (in case metadata changed)
	if existing, exists := s.policies[policy.ID]; exists {
		oldKey := existing.CompositeKey()
		if oldKey != compositeKey {
			delete(s.byCompositeKey, oldKey)
		}
	}

	// Store policy
	s.policies[policy.ID] = policy
	s.byCompositeKey[compositeKey] = policy

	return nil
}

// Get retrieves a policy configuration by ID
func (s *PolicyStore) Get(id string) (*models.StoredPolicyConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	policy, exists := s.policies[id]
	return policy, exists
}

// GetByCompositeKey retrieves a policy configuration by composite key (api_name:version:context)
func (s *PolicyStore) GetByCompositeKey(apiName, version, context string) (*models.StoredPolicyConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := apiName + ":" + version + ":" + context
	policy, exists := s.byCompositeKey[key]
	return policy, exists
}

// GetAll returns all policy configurations
func (s *PolicyStore) GetAll() []*models.StoredPolicyConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	policies := make([]*models.StoredPolicyConfig, 0, len(s.policies))
	for _, policy := range s.policies {
		policies = append(policies, policy)
	}

	return policies
}

// Delete removes a policy configuration by ID
func (s *PolicyStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	policy, exists := s.policies[id]
	if !exists {
		return fmt.Errorf("policy configuration with ID %s not found", id)
	}

	// Remove from both maps
	compositeKey := policy.CompositeKey()
	delete(s.policies, id)
	delete(s.byCompositeKey, compositeKey)

	return nil
}

// Count returns the total number of stored policy configurations
func (s *PolicyStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.policies)
}

// Clear removes all policy configurations
func (s *PolicyStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.policies = make(map[string]*models.StoredPolicyConfig)
	s.byCompositeKey = make(map[string]*models.StoredPolicyConfig)
}

// IncrementResourceVersion increments and returns the new resource version
func (s *PolicyStore) IncrementResourceVersion() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.resourceVersion++
	return s.resourceVersion
}

// GetResourceVersion returns the current resource version
func (s *PolicyStore) GetResourceVersion() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.resourceVersion
}
