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

package storage

import (
	"fmt"
	"sync"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// RuntimeConfigStore provides thread-safe in-memory storage for RuntimeDeployConfig.
// Keyed by "kind:handle" (e.g. "RestApi:petstore"), consistent with ConfigStore indexing.
type RuntimeConfigStore struct {
	mu              sync.RWMutex
	configs         map[string]*models.RuntimeDeployConfig
	resourceVersion int64
}

// NewRuntimeConfigStore creates a new RuntimeConfigStore.
func NewRuntimeConfigStore() *RuntimeConfigStore {
	return &RuntimeConfigStore{
		configs:         make(map[string]*models.RuntimeDeployConfig),
		resourceVersion: 0,
	}
}

// Key returns the store key for a RuntimeDeployConfig.
func Key(kind, handle string) string {
	return kind + ":" + handle
}

// Set stores or updates a RuntimeDeployConfig.
func (s *RuntimeConfigStore) Set(key string, rdc *models.RuntimeDeployConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configs[key] = rdc
}

// Get retrieves a RuntimeDeployConfig by key.
func (s *RuntimeConfigStore) Get(key string) (*models.RuntimeDeployConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rdc, exists := s.configs[key]
	return rdc, exists
}

// GetAll returns all RuntimeDeployConfigs.
func (s *RuntimeConfigStore) GetAll() []*models.RuntimeDeployConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*models.RuntimeDeployConfig, 0, len(s.configs))
	for _, rdc := range s.configs {
		result = append(result, rdc)
	}
	return result
}

// Delete removes a RuntimeDeployConfig by key.
func (s *RuntimeConfigStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.configs[key]; !exists {
		return fmt.Errorf("runtime config with key %s: %w", key, ErrPolicyNotFound)
	}
	delete(s.configs, key)
	return nil
}

// IncrementResourceVersion increments and returns the new resource version.
func (s *RuntimeConfigStore) IncrementResourceVersion() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resourceVersion++
	return s.resourceVersion
}

// GetResourceVersion returns the current resource version.
func (s *RuntimeConfigStore) GetResourceVersion() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.resourceVersion
}
