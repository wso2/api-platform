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

// Package webhooksecret provides an in-memory store for per-API plaintext HMAC
// secrets used by the websub-hmac-auth policy at request validation time.
// Secrets are stored as plaintext (not hashed) because HMAC computation requires
// the raw secret bytes. The store is populated on startup from the database and
// kept in sync via EventHub events.
package webhooksecret

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Common storage errors — implementation agnostic.
var (
	// ErrNotFound is returned when a secret is not found.
	ErrNotFound = errors.New("webhook secret not found")

	// ErrConflict is returned when a secret with the same name already exists.
	ErrConflict = errors.New("webhook secret already exists")

	// ErrInvalidInput is returned when input validation fails.
	ErrInvalidInput = errors.New("invalid input")
)

// Singleton instance.
var (
	instance *WebhookSecretStore
	once     sync.Once
)

// WebhookSecretStore holds per-API HMAC secrets in memory for fast access.
// The inner map uses the secret name as key and the plaintext value as value.
// Key: "API ID" → Value: map[name]plaintext
type WebhookSecretStore struct {
	mu           sync.RWMutex
	secretsByAPI map[string]map[string]string
}

// NewWebhookSecretStore creates a new empty store.
func NewWebhookSecretStore() *WebhookSecretStore {
	return &WebhookSecretStore{
		secretsByAPI: make(map[string]map[string]string),
	}
}

// GetStoreInstance returns the process-wide singleton store.
func GetStoreInstance() *WebhookSecretStore {
	once.Do(func() {
		instance = NewWebhookSecretStore()
	})
	return instance
}

// Store saves a plaintext secret keyed by (apiId, name). If a secret with the
// same name already exists for this API, its value is replaced (rotation).
func (s *WebhookSecretStore) Store(apiId, name, plaintext string) error {
	apiId = strings.TrimSpace(apiId)
	name = strings.TrimSpace(name)
	plaintext = strings.TrimSpace(plaintext)

	if apiId == "" {
		return fmt.Errorf("%w: apiId cannot be empty", ErrInvalidInput)
	}
	if name == "" {
		return fmt.Errorf("%w: name cannot be empty", ErrInvalidInput)
	}
	if plaintext == "" {
		return fmt.Errorf("%w: plaintext cannot be empty", ErrInvalidInput)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.secretsByAPI[apiId] == nil {
		s.secretsByAPI[apiId] = make(map[string]string)
	}
	s.secretsByAPI[apiId][name] = plaintext
	return nil
}

// Remove deletes the named secret for an API. Returns ErrNotFound when absent
// (idempotent callers may ignore this).
func (s *WebhookSecretStore) Remove(apiId, name string) error {
	apiId = strings.TrimSpace(apiId)
	name = strings.TrimSpace(name)

	s.mu.Lock()
	defer s.mu.Unlock()

	m, ok := s.secretsByAPI[apiId]
	if !ok {
		return ErrNotFound
	}
	if _, exists := m[name]; !exists {
		return ErrNotFound
	}
	delete(m, name)
	if len(m) == 0 {
		delete(s.secretsByAPI, apiId)
	}
	return nil
}

// RemoveAllByAPI removes every secret associated with the given API.
func (s *WebhookSecretStore) RemoveAllByAPI(apiId string) error {
	apiId = strings.TrimSpace(apiId)

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.secretsByAPI, apiId)
	return nil
}

// GetAllByAPI returns the plaintext values of all active secrets for the API.
// The HMAC policy calls this and tries each value until one produces a matching
// signature, supporting multiple simultaneous active secrets (zero-downtime rotation).
// Returns an empty slice when no secrets exist for the API.
func (s *WebhookSecretStore) GetAllByAPI(apiId string) []string {
	apiId = strings.TrimSpace(apiId)

	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.secretsByAPI[apiId]
	if !ok || len(m) == 0 {
		return nil
	}

	result := make([]string, 0, len(m))
	for _, v := range m {
		result = append(result, v)
	}
	return result
}

// ReplaceAll atomically replaces the entire store contents with a new snapshot.
// Used during startup bulk-load to swap state in one critical-section operation.
func (s *WebhookSecretStore) ReplaceAll(newMap map[string]map[string]string) error {
	replacement := make(map[string]map[string]string, len(newMap))
	for apiId, secrets := range newMap {
		apiId = strings.TrimSpace(apiId)
		if apiId == "" {
			return fmt.Errorf("%w: apiId cannot be empty", ErrInvalidInput)
		}
		if len(secrets) == 0 {
			continue
		}
		clone := make(map[string]string, len(secrets))
		for name, plaintext := range secrets {
			name = strings.TrimSpace(name)
			plaintext = strings.TrimSpace(plaintext)
			if name == "" {
				return fmt.Errorf("%w: secret name cannot be empty", ErrInvalidInput)
			}
			if plaintext == "" {
				return fmt.Errorf("%w: secret plaintext cannot be empty", ErrInvalidInput)
			}
			clone[name] = plaintext
		}
		replacement[apiId] = clone
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.secretsByAPI = replacement
	return nil
}

// GetAll returns a deep copy of the full store contents keyed by (apiId → name → plaintext).
// Used by snapshot managers to serialize the store for xDS delivery.
func (s *WebhookSecretStore) GetAll() map[string]map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]map[string]string, len(s.secretsByAPI))
	for apiID, secrets := range s.secretsByAPI {
		clone := make(map[string]string, len(secrets))
		for name, plaintext := range secrets {
			clone[name] = plaintext
		}
		result[apiID] = clone
	}
	return result
}

// ClearAll removes all secrets from the store. Primarily used in tests.
func (s *WebhookSecretStore) ClearAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secretsByAPI = make(map[string]map[string]string)
}

// BuildWebhookSecretEntityID constructs the composite entity ID used in EventHub
// events. Format: "<artifactUUID>_<secretUUID>_<secretName>".
// The name segment allows delete-path processors to skip a DB round-trip.
func BuildWebhookSecretEntityID(artifactUUID, secretUUID, secretName string) string {
	return artifactUUID + "_" + secretUUID + "_" + secretName
}

// ParseWebhookSecretEntityID decomposes an entity ID produced by
// BuildWebhookSecretEntityID back into its three components.
func ParseWebhookSecretEntityID(entityID string) (artifactUUID, secretUUID, secretName string, err error) {
	parts := strings.SplitN(entityID, "_", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", fmt.Errorf("invalid webhook secret entity ID: %q", entityID)
	}
	return parts[0], parts[1], parts[2], nil
}
