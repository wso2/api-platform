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
	"strings"
	"sync"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// ConfigStore holds all API configurations in memory for fast access
type ConfigStore struct {
	mu           sync.RWMutex                    // Protects concurrent access
	configs      map[string]*models.StoredConfig // Key: config ID
	nameVersion  map[string]string               // Key: "name:version" → Value: config ID
	handle       map[string]string               // Key: "handle" → Value: config ID
	snapVersion  int64                           // Current xDS snapshot version
	TopicManager *TopicManager

	// LLM Provider Templates
	templates          map[string]*models.StoredLLMProviderTemplate // Key: template ID
	templateIdByHandle map[string]string

	// API Keys storage
	apiKeys      map[string]*models.APIKey   // Key: API key value → Value: APIKey
	apiKeysByAPI map[string][]*models.APIKey // Key: "configID" → Value: slice of APIKeys
}

// NewConfigStore creates a new in-memory config store
func NewConfigStore() *ConfigStore {
	return &ConfigStore{
		configs:            make(map[string]*models.StoredConfig),
		nameVersion:        make(map[string]string),
		handle:             make(map[string]string),
		snapVersion:        0,
		TopicManager:       NewTopicManager(),
		templates:          make(map[string]*models.StoredLLMProviderTemplate),
		templateIdByHandle: make(map[string]string),
		apiKeys:            make(map[string]*models.APIKey),
		apiKeysByAPI:       make(map[string][]*models.APIKey),
	}
}

// Add stores a new configuration in memory
func (cs *ConfigStore) Add(cfg *models.StoredConfig) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	key := cfg.GetCompositeKey()
	handle := cfg.GetHandle()
	if existingID, exists := cs.handle[handle]; exists {
		return fmt.Errorf("%w: configuration with handle '%s' already exists (ID: %s)",
			ErrConflict, handle, existingID)
	}
	if existingID, exists := cs.nameVersion[key]; exists {
		return fmt.Errorf("%w: configuration with displayName '%s' and version '%s' already exists (ID: %s)",
			ErrConflict, cfg.GetDisplayName(), cfg.GetVersion(), existingID)
	}

	cs.configs[cfg.ID] = cfg
	cs.handle[handle] = cfg.ID
	cs.nameVersion[key] = cfg.ID

	if cfg.Configuration.Kind == api.WebSubApi {
		err := cs.updateTopics(cfg)
		if err != nil {
			return err
		}
	}
	return nil
}

// Update modifies an existing configuration in memory
func (cs *ConfigStore) Update(cfg *models.StoredConfig) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	existing, exists := cs.configs[cfg.ID]
	if !exists {
		return fmt.Errorf("configuration with ID '%s' not found", cfg.ID)
	}

	// If handle changed, update the handle index
	oldHandle := existing.GetHandle()
	newHandle := cfg.GetHandle()

	if oldHandle != newHandle {
		// Check if new handle already exists
		if existingID, exists := cs.handle[newHandle]; exists && existingID != cfg.ID {
			return fmt.Errorf("%w: configuration with handle '%s' already exists (ID: %s)",
				ErrConflict, newHandle, existingID)
		}
		delete(cs.handle, oldHandle)
		cs.handle[newHandle] = cfg.ID
	}

	// If name/version changed, update the nameVersion index
	oldKey := existing.GetCompositeKey()
	newKey := cfg.GetCompositeKey()

	if oldKey != newKey {
		// Check if new name:version combination already exists
		if existingID, exists := cs.nameVersion[newKey]; exists && existingID != cfg.ID {
			return fmt.Errorf("%w: configuration with displayName '%s' and version '%s' already exists (ID: %s)",
				ErrConflict, cfg.GetDisplayName(), cfg.GetVersion(), existingID)
		}
		delete(cs.nameVersion, oldKey)
		cs.nameVersion[newKey] = cfg.ID
	}

	if cfg.Configuration.Kind == api.WebSubApi {
		err := cs.updateTopics(cfg)
		if err != nil {
			return err
		}
	}

	cs.configs[cfg.ID] = cfg
	return nil
}

func (cs *ConfigStore) updateTopics(cfg *models.StoredConfig) error {
	asyncData, err := cfg.Configuration.Spec.AsWebhookAPIData()
	if err != nil {
		return fmt.Errorf("failed to parse async API data: %w", err)
	}
	// Maintaining a topic map to process topics
	// Running these inside Add or Delete configs might add extra latency to the API Deployment process
	// TODO: Optimize topic management if needed by maintaining a separate topic manager struct

	apiTopicsPerRevision := make(map[string]bool)
	for _, topic := range asyncData.Channels {
		name := strings.TrimPrefix(asyncData.DisplayName, "/")
		context := strings.TrimPrefix(asyncData.Context, "/")
		version := strings.TrimPrefix(asyncData.Version, "/")
		path := strings.TrimPrefix(topic.Path, "/")
		modifiedTopic := fmt.Sprintf("%s_%s_%s_%s", name, context, version, path)
		cs.TopicManager.Add(cfg.ID, modifiedTopic)
		apiTopicsPerRevision[modifiedTopic] = true
	}

	for _, topic := range cs.TopicManager.GetAllByConfig(cfg.ID) {
		if _, exists := apiTopicsPerRevision[topic]; !exists {
			cs.TopicManager.Remove(cfg.ID, topic)
		}
	}
	return nil
}

// Delete removes a configuration from memory
func (cs *ConfigStore) Delete(id string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cfg, exists := cs.configs[id]
	if !exists {
		return fmt.Errorf("configuration with ID '%s' not found", id)
	}

	key := cfg.GetCompositeKey()
	handle := cfg.GetHandle()

	if cfg.Configuration.Kind == api.WebSubApi {
		cs.TopicManager.RemoveAllForConfig(cfg.ID)
	}
	delete(cs.handle, handle)
	delete(cs.nameVersion, key)
	delete(cs.configs, id)
	return nil
}

// Get retrieves a configuration by ID
func (cs *ConfigStore) Get(id string) (*models.StoredConfig, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	cfg, exists := cs.configs[id]
	if !exists {
		return nil, fmt.Errorf("configuration with ID '%s' not found", id)
	}
	return cfg, nil
}

// GetByNameVersion retrieves a configuration by name and version
func (cs *ConfigStore) GetByNameVersion(name, version string) (*models.StoredConfig, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", name, version)
	configID, exists := cs.nameVersion[key]
	if !exists {
		return nil, fmt.Errorf("configuration with name '%s' and version '%s' not found", name, version)
	}

	cfg, exists := cs.configs[configID]
	if !exists {
		return nil, fmt.Errorf("configuration with name '%s' and version '%s' not found", name, version)
	}
	return cfg, nil
}

// GetByHandle retrieves a configuration by handle
func (cs *ConfigStore) GetByHandle(handle string) (*models.StoredConfig, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	key := fmt.Sprintf("%s", handle)
	configID, exists := cs.handle[key]
	if !exists {
		return nil, fmt.Errorf("configuration with handle '%s' not found", handle)
	}

	cfg, exists := cs.configs[configID]
	if !exists {
		return nil, fmt.Errorf("configuration with handle '%s' not found", handle)
	}

	if cfg.GetHandle() != handle {
		return nil, fmt.Errorf("configuration with handle '%s' not found", handle)
	}
	return cfg, nil
}

// GetAll returns all configurations
func (cs *ConfigStore) GetAll() []*models.StoredConfig {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	result := make([]*models.StoredConfig, 0, len(cs.configs))
	for _, cfg := range cs.configs {
		result = append(result, cfg)
	}
	return result
}

// GetAllByKind returns all configurations of a specific kind
func (cs *ConfigStore) GetAllByKind(kind string) []*models.StoredConfig {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	result := make([]*models.StoredConfig, 0)
	for _, cfg := range cs.configs {
		if cfg.Kind == kind {
			result = append(result, cfg)
		}
	}
	return result
}

// GetByKindNameAndVersion returns a configuration of a specific kind, name and version
func (cs *ConfigStore) GetByKindNameAndVersion(kind string, name string, version string) *models.StoredConfig {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	for _, cfg := range cs.configs {
		if cfg.Kind != kind {
			continue
		}
		sc := cfg
		if sc.GetDisplayName() == name && sc.GetVersion() == version {
			return sc
		}
	}
	return nil
}

// GetByKindAndHandle returns a configuration of a specific kind, and handle
func (cs *ConfigStore) GetByKindAndHandle(kind string, handle string) *models.StoredConfig {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	for _, cfg := range cs.configs {
		if cfg.Kind != kind {
			continue
		}
		sc := cfg
		if sc.GetHandle() == handle {
			return sc
		}
	}
	return nil
}

// IncrementSnapshotVersion atomically increments and returns the next snapshot version
func (cs *ConfigStore) IncrementSnapshotVersion() int64 {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.snapVersion++
	return cs.snapVersion
}

// GetSnapshotVersion returns the current snapshot version
func (cs *ConfigStore) GetSnapshotVersion() int64 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.snapVersion
}

// SetSnapshotVersion sets the snapshot version (used during startup)
func (cs *ConfigStore) SetSnapshotVersion(version int64) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.snapVersion = version
}

// ========================================
// LLM Provider Template Methods
// ========================================

// AddTemplate adds a new LLM provider template. ID must be unique and immutable; name must be unique.
func (cs *ConfigStore) AddTemplate(template *models.StoredLLMProviderTemplate) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Normalize inputs
	uuid := strings.TrimSpace(template.ID)
	handle := strings.TrimSpace(template.GetHandle())

	if uuid == "" || handle == "" {
		return fmt.Errorf("template ID and handle is required")
	}

	// Enforce unique immutable ID: cannot add if ID already exists
	if _, exists := cs.templates[uuid]; exists {
		return fmt.Errorf("template with uuid '%s' already exists", uuid)
	}

	// Enforce unique handle: cannot add if handle already mapped to a different UUID
	if _, exists := cs.templateIdByHandle[handle]; exists {
		return fmt.Errorf("template with handle '%s' already exists", handle)
	}

	// Store
	cs.templates[uuid] = template
	cs.templateIdByHandle[handle] = uuid
	return nil
}

// UpdateTemplate updates an existing LLM provider template's metadata. ID cannot change; only name can change.
func (cs *ConfigStore) UpdateTemplate(template *models.StoredLLMProviderTemplate) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Normalize inputs
	uuid := strings.TrimSpace(template.ID)
	newHandle := strings.TrimSpace(template.GetHandle())

	if uuid == "" || newHandle == "" {
		return fmt.Errorf("template uuid and handle is required")
	}

	// Require existing template by ID (ID is immutable)
	existing, exists := cs.templates[uuid]
	if !exists {
		return fmt.Errorf("template with uuid '%s' not found", uuid)
	}

	oldName := strings.TrimSpace(existing.GetHandle())

	// If name is changing, ensure no collision with another template
	if newHandle != oldName {
		if mappedID, exists := cs.templateIdByHandle[newHandle]; exists && mappedID != uuid {
			return fmt.Errorf("template with given handle '%s' already exists", newHandle)
		}
		// Remove old handle mapping if it points to this ID
		if mappedID, ok := cs.templateIdByHandle[oldName]; ok && mappedID == uuid {
			delete(cs.templateIdByHandle, oldName)
		}
	}

	// Update stored template and refresh name mapping
	cs.templates[uuid] = template
	cs.templateIdByHandle[newHandle] = uuid
	return nil
}

// DeleteTemplate removes an LLM provider template from the store by ID
func (cs *ConfigStore) DeleteTemplate(id string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	template, exists := cs.templates[id]
	if !exists {
		return fmt.Errorf("template with ID '%s' not found", id)
	}

	delete(cs.templates, id)
	delete(cs.templateIdByHandle, template.GetHandle())
	return nil
}

// GetTemplate retrieves an LLM provider template by ID
func (cs *ConfigStore) GetTemplate(id string) (*models.StoredLLMProviderTemplate, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	template, exists := cs.templates[id]
	if !exists {
		return nil, fmt.Errorf("template with ID '%s' not found", id)
	}

	return template, nil
}

// GetTemplateByHandle retrieves an LLM provider template by handle identifier
func (cs *ConfigStore) GetTemplateByHandle(handle string) (*models.StoredLLMProviderTemplate, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	templateId, exists := cs.templateIdByHandle[handle]
	if !exists {
		return nil, fmt.Errorf("template with handle '%s' not found", handle)
	}

	return cs.templates[templateId], nil
}

// GetAllTemplates retrieves all LLM provider templates
func (cs *ConfigStore) GetAllTemplates() []*models.StoredLLMProviderTemplate {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	templates := make([]*models.StoredLLMProviderTemplate, 0, len(cs.templates))
	for _, template := range cs.templates {
		templates = append(templates, template)
	}

	return templates
}

// StoreAPIKey stores an API key in the in-memory cache
func (cs *ConfigStore) StoreAPIKey(apiKey *models.APIKey) error {
	if apiKey == nil {
		return fmt.Errorf("API key cannot be nil")
	}
	if strings.TrimSpace(apiKey.Name) == "" {
		return fmt.Errorf("API key name cannot be empty")
	}
	if strings.TrimSpace(apiKey.APIKey) == "" {
		return fmt.Errorf("API key value cannot be empty")
	}
	if strings.TrimSpace(apiKey.APIId) == "" {
		return fmt.Errorf("API apiId cannot be empty")
	}

	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Check if an API key with the same apiId and name already exists
	existingKeys, apiIdExists := cs.apiKeysByAPI[apiKey.APIId]
	var existingKeyIndex = -1
	var oldAPIKeyValue string

	if apiIdExists {
		for i, existingKey := range existingKeys {
			if existingKey.Name == apiKey.Name {
				existingKeyIndex = i
				oldAPIKeyValue = existingKey.APIKey
				break
			}
		}
	}

	// Check if the new API key value already exists (but with different apiId/name)
	if _, keyExists := cs.apiKeys[apiKey.APIKey]; keyExists && oldAPIKeyValue != apiKey.APIKey {
		return ErrConflict
	}

	if existingKeyIndex >= 0 {
		// Update existing API key
		// Remove old API key value from apiKeys map if it's different
		if oldAPIKeyValue != apiKey.APIKey {
			delete(cs.apiKeys, oldAPIKeyValue)
		}

		// Update the existing entry in apiKeysByAPI
		cs.apiKeysByAPI[apiKey.APIId][existingKeyIndex] = apiKey

		// Store by new API key value
		cs.apiKeys[apiKey.APIKey] = apiKey
	} else {
		// Insert new API key
		// Check if API key value already exists
		if _, exists := cs.apiKeys[apiKey.APIKey]; exists {
			return ErrConflict
		}

		// Store by API key value
		cs.apiKeys[apiKey.APIKey] = apiKey

		// Store by API apiId
		cs.apiKeysByAPI[apiKey.APIId] = append(cs.apiKeysByAPI[apiKey.APIId], apiKey)
	}

	return nil
}

// GetAPIKeyByKey retrieves an API key by its key value
func (cs *ConfigStore) GetAPIKeyByKey(key string) (*models.APIKey, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	apiKey, exists := cs.apiKeys[key]
	if !exists {
		return nil, ErrNotFound
	}

	return apiKey, nil
}

// GetAPIKeysByAPI retrieves all API keys for a specific API
func (cs *ConfigStore) GetAPIKeysByAPI(apiId string) ([]*models.APIKey, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	apiKeys, exists := cs.apiKeysByAPI[apiId]
	if !exists {
		return []*models.APIKey{}, nil // Return empty slice instead of nil
	}

	// Return a copy to prevent external modification
	result := make([]*models.APIKey, len(apiKeys))
	copy(result, apiKeys)
	return result, nil
}

// GetAPIKeyByName retrieves an API key by its apiId and name
func (cs *ConfigStore) GetAPIKeyByName(apiId, name string) (*models.APIKey, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	apiKeys, exists := cs.apiKeysByAPI[apiId]
	if !exists {
		return nil, ErrNotFound
	}

	// Search for the API key with the matching name
	for _, apiKey := range apiKeys {
		if apiKey.Name == name {
			return apiKey, nil
		}
	}

	return nil, ErrNotFound
}

// RemoveAPIKey removes an API key from the in-memory cache
func (cs *ConfigStore) RemoveAPIKey(apiKey string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Get the API key first to find its API association
	key, exists := cs.apiKeys[apiKey]
	if !exists {
		return ErrNotFound
	}

	// Remove from main map
	delete(cs.apiKeys, apiKey)

	// Remove from API-specific map
	if apiKeys, exists := cs.apiKeysByAPI[key.APIId]; exists {
		for i, k := range apiKeys {
			if k.APIKey == apiKey {
				// Remove from slice
				cs.apiKeysByAPI[key.APIId] = append(apiKeys[:i], apiKeys[i+1:]...)
				break
			}
		}
		// Clean up empty slices
		if len(cs.apiKeysByAPI[key.APIId]) == 0 {
			delete(cs.apiKeysByAPI, key.APIId)
		}
	}

	return nil
}

// RemoveAPIKeysByAPI removes all API keys for a specific API
func (cs *ConfigStore) RemoveAPIKeysByAPI(apiId string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	apiKeys, exists := cs.apiKeysByAPI[apiId]
	if !exists {
		return nil // No keys to remove
	}

	// Remove from main map
	for _, key := range apiKeys {
		delete(cs.apiKeys, key.APIKey)
	}

	// Remove from API-specific map
	delete(cs.apiKeysByAPI, apiId)

	return nil
}

// RemoveAPIKeyByName removes an API key from the in-memory cache by its apiId and name
func (cs *ConfigStore) RemoveAPIKeyByName(apiId, name string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Get API keys for the apiId
	apiKeys, exists := cs.apiKeysByAPI[apiId]
	if !exists {
		return ErrNotFound
	}

	// Find the API key with the matching name
	var targetAPIKey *models.APIKey
	var targetIndex = -1

	for i, apiKey := range apiKeys {
		if apiKey.Name == name {
			targetAPIKey = apiKey
			targetIndex = i
			break
		}
	}

	if targetAPIKey == nil {
		return ErrNotFound
	}

	// Remove from main apiKeys map
	delete(cs.apiKeys, targetAPIKey.APIKey)

	// Remove from apiKeysByAPI slice
	cs.apiKeysByAPI[apiId] = append(apiKeys[:targetIndex], apiKeys[targetIndex+1:]...)

	// Clean up empty slices
	if len(cs.apiKeysByAPI[apiId]) == 0 {
		delete(cs.apiKeysByAPI, apiId)
	}

	return nil
}
