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
	"sort"
	"sync"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// BackendStore holds the gateway's Backends in memory, keyed by name. It is what the xDS
// translator and the Backend publisher read; the database stays the source of truth.
type BackendStore struct {
	mu     sync.RWMutex
	byName map[string]*models.StoredBackend
}

// NewBackendStore creates an empty in-memory Backend store.
func NewBackendStore() *BackendStore {
	return &BackendStore{byName: make(map[string]*models.StoredBackend)}
}

// Upsert adds a Backend or replaces the one with the same name.
func (s *BackendStore) Upsert(backend *models.StoredBackend) error {
	if backend == nil || backend.GetHandle() == "" {
		return fmt.Errorf("backend name is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byName[backend.GetHandle()] = backend
	return nil
}

// Delete removes a Backend by name, returning it, or nil when it was not present.
func (s *BackendStore) Delete(name string) *models.StoredBackend {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing := s.byName[name]
	delete(s.byName, name)
	return existing
}

// DeleteByUUID removes the Backend with this UUID, returning it, or nil when no Backend has that
// UUID. The lookup and the removal happen under one lock, so a delete for an old UUID never
// removes a Backend that has since been created again under the same name.
func (s *BackendStore) DeleteByUUID(uuid string) *models.StoredBackend {
	s.mu.Lock()
	defer s.mu.Unlock()
	for name, backend := range s.byName {
		if backend.UUID == uuid {
			delete(s.byName, name)
			return backend
		}
	}
	return nil
}

// Get returns a Backend by name.
func (s *BackendStore) Get(name string) (*models.StoredBackend, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	backend, ok := s.byName[name]
	return backend, ok
}

// GetByUUID returns a Backend by UUID.
func (s *BackendStore) GetByUUID(uuid string) (*models.StoredBackend, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, backend := range s.byName {
		if backend.UUID == uuid {
			return backend, true
		}
	}
	return nil, false
}

// ListBackends returns every Backend, ordered by name, so translations are deterministic.
func (s *BackendStore) ListBackends() []*models.StoredBackend {
	s.mu.RLock()
	defer s.mu.RUnlock()
	backends := make([]*models.StoredBackend, 0, len(s.byName))
	for _, backend := range s.byName {
		backends = append(backends, backend)
	}
	sort.Slice(backends, func(i, j int) bool { return backends[i].GetHandle() < backends[j].GetHandle() })
	return backends
}

// ReplaceAll replaces the stored set, e.g. when loading from the database at startup.
func (s *BackendStore) ReplaceAll(backends []*models.StoredBackend) {
	byName := make(map[string]*models.StoredBackend, len(backends))
	for _, backend := range backends {
		if backend != nil && backend.GetHandle() != "" {
			byName[backend.GetHandle()] = backend
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byName = byName
}
