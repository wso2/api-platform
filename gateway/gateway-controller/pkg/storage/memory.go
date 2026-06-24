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
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
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
	templates map[string]*models.StoredLLMProviderTemplate // Key: template ID
	// templateIdByGroupVersionID maps a handle to the UUID of its LATEST (most recently
	// created) version, so handle-based lookups resolve to the newest template.
	templateIdByGroupVersionID map[string]string
	// templateIdByGroupVersionIDAndVersion maps a (handle, version) pair to a UUID, allowing
	// multiple versions of the same handle to coexist. A typed composite key is
	// used instead of a delimiter-joined string so distinct pairs can never
	// collide on the same map key.
	templateIdByGroupVersionIDAndVersion map[templateVersionKey]string

	// API Keys storage
	apiKeysByAPI map[string]map[string]*models.APIKey // Key: configID → Value: map[keyID]*APIKey

	// Labels storage
	labelsByAPI map[string]map[string]string // Key: API handle (metadata.name) → Value: labels map
}

// NewConfigStore creates a new in-memory config store
func NewConfigStore() *ConfigStore {
	return &ConfigStore{
		configs:            make(map[string]*models.StoredConfig),
		nameVersion:        make(map[string]string),
		handle:             make(map[string]string),
		snapVersion:        0,
		TopicManager:       NewTopicManager(),
		templates:                 make(map[string]*models.StoredLLMProviderTemplate),
		templateIdByGroupVersionID:         make(map[string]string),
		templateIdByGroupVersionIDAndVersion: make(map[templateVersionKey]string),
		apiKeysByAPI:       make(map[string]map[string]*models.APIKey),
		labelsByAPI:        make(map[string]map[string]string),
	}
}

// Add stores a new configuration in memory
func (cs *ConfigStore) Add(cfg *models.StoredConfig) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	key := cfg.GetCompositeKey()
	handleKey := cfg.Kind + ":" + cfg.Handle
	if existingID, exists := cs.handle[handleKey]; exists {
		return fmt.Errorf("%w: configuration with handle '%s' already exists (ID: %s)",
			ErrConflict, cfg.Handle, existingID)
	}
	if existingID, exists := cs.nameVersion[key]; exists {
		return fmt.Errorf("%w: configuration with displayName '%s' and version '%s' already exists (ID: %s)",
			ErrConflict, cfg.DisplayName, cfg.Version, existingID)
	}

	cs.configs[cfg.UUID] = cfg
	cs.handle[handleKey] = cfg.UUID
	cs.nameVersion[key] = cfg.UUID

	// Store labels if present
	if labels := cfg.GetLabels(); labels != nil {
		labelsCopy := make(map[string]string)
		for k, v := range *labels {
			labelsCopy[k] = v
		}
		cs.labelsByAPI[cfg.Handle] = labelsCopy
	}

	if cfg.Kind == "WebSubApi" {
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

	existing, exists := cs.configs[cfg.UUID]
	if !exists {
		return fmt.Errorf("configuration with UUID '%s' not found", cfg.UUID)
	}

	// If handle changed, update the handle index
	oldHandle := existing.Handle
	newHandle := cfg.Handle
	oldHandleKey := existing.Kind + ":" + oldHandle
	newHandleKey := cfg.Kind + ":" + newHandle

	if oldHandleKey != newHandleKey {
		// Check if new handle already exists
		if existingUUID, exists := cs.handle[newHandleKey]; exists && existingUUID != cfg.UUID {
			return fmt.Errorf("%w: configuration with handle '%s' already exists (UUID: %s)",
				ErrConflict, newHandle, existingUUID)
		}
		delete(cs.handle, oldHandleKey)
		cs.handle[newHandleKey] = cfg.UUID
	}

	// If name/version changed, update the nameVersion index
	oldKey := existing.GetCompositeKey()
	newKey := cfg.GetCompositeKey()

	if oldKey != newKey {
		// Check if new name:version combination already exists
		if existingUUID, exists := cs.nameVersion[newKey]; exists && existingUUID != cfg.UUID {
			return fmt.Errorf("%w: configuration with displayName '%s' and version '%s' already exists (UUID: %s)",
				ErrConflict, cfg.DisplayName, cfg.Version, existingUUID)
		}
		delete(cs.nameVersion, oldKey)
		cs.nameVersion[newKey] = cfg.UUID
	}

	if cfg.Kind == "WebSubApi" {
		err := cs.updateTopics(cfg)
		if err != nil {
			return err
		}
	}

	cs.configs[cfg.UUID] = cfg

	// Store labels with new handle
	labels := cfg.GetLabels()
	if oldHandle != newHandle {
		delete(cs.labelsByAPI, oldHandle)
		if labels != nil {
			labelsCopy := make(map[string]string)
			for k, v := range *labels {
				labelsCopy[k] = v
			}
			cs.labelsByAPI[newHandle] = labelsCopy
		}
	} else {
		if labels != nil {
			labelsCopy := make(map[string]string)
			for k, v := range *labels {
				labelsCopy[k] = v
			}
			cs.labelsByAPI[oldHandle] = labelsCopy
		} else {
			delete(cs.labelsByAPI, oldHandle)
		}
	}

	return nil
}

func (cs *ConfigStore) updateTopics(cfg *models.StoredConfig) error {
	webSubCfg, ok := cfg.Configuration.(api.WebSubAPI)
	if !ok {
		return fmt.Errorf("configuration is not a WebSubAPI")
	}
	asyncData := webSubCfg.Spec
	// Maintaining a topic map to process topics
	// Running these inside Add or Delete configs might add extra latency to the API Deployment process
	// TODO: Optimize topic management if needed by maintaining a separate topic manager struct

	apiTopicsPerRevision := make(map[string]bool)
	var channels map[string]api.WebSubChannel
	if asyncData.Channels != nil {
		channels = *asyncData.Channels
	}
	for chName := range channels {
		contextWithVersion := strings.ReplaceAll(asyncData.Context, "$version", asyncData.Version)
		contextWithVersion = strings.TrimPrefix(contextWithVersion, "/")
		contextWithVersion = strings.ReplaceAll(contextWithVersion, "/", "_")
		name := strings.TrimPrefix(chName, "/")
		modifiedTopic := fmt.Sprintf("%s_%s", contextWithVersion, name)
		cs.TopicManager.Add(cfg.UUID, modifiedTopic)
		apiTopicsPerRevision[modifiedTopic] = true
	}

	for _, topic := range cs.TopicManager.GetAllByConfig(cfg.UUID) {
		if _, exists := apiTopicsPerRevision[topic]; !exists {
			cs.TopicManager.Remove(cfg.UUID, topic)
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

	if cfg.Kind == "WebSubApi" {
		cs.TopicManager.RemoveAllForConfig(cfg.UUID)
	}
	delete(cs.handle, cfg.Kind+":"+cfg.Handle)
	delete(cs.nameVersion, cfg.GetCompositeKey())
	delete(cs.configs, id)
	// Remove from labels map
	delete(cs.labelsByAPI, cfg.Handle)
	return nil
}

// Get retrieves a configuration by ID
func (cs *ConfigStore) Get(id string) (*models.StoredConfig, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	cfg, exists := cs.configs[id]
	if !exists {
		return nil, fmt.Errorf("%w: id=%s", ErrNotFound, id)
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

// GetAllSensitiveValues aggregates SensitiveValues from all stored configs.
// Used by the config dump handler to redact resolved secret values from the dump output.
func (cs *ConfigStore) GetAllSensitiveValues() []string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	var result []string
	for _, cfg := range cs.configs {
		result = append(result, cfg.SensitiveValues...)
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
func (cs *ConfigStore) GetByKindNameAndVersion(kind string, name string, version string) (*models.StoredConfig, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	key := fmt.Sprintf("%s:%s:%s", kind, name, version)
	configID, exists := cs.nameVersion[key]
	if !exists {
		return nil, nil
	}

	cfg, exists := cs.configs[configID]
	if !exists {
		return nil, nil
	}
	return cfg, nil
}

// GetByKindAndHandle returns a configuration of a specific kind, and handle
func (cs *ConfigStore) GetByKindAndHandle(kind string, handle string) (*models.StoredConfig, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	handleKey := kind + ":" + handle
	configID, exists := cs.handle[handleKey]
	if !exists {
		return nil, nil
	}

	cfg, exists := cs.configs[configID]
	if !exists {
		return nil, nil
	}
	return cfg, nil
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

// templateVersionKey is a composite key type for indexing templates by (handle, version).
type templateVersionKey struct {
	handle  string
	version string
}

// groupVersionIDAndVersionKey builds the composite (handle, version) index key.
func groupVersionIDAndVersionKey(handle, version string) templateVersionKey {
	return templateVersionKey{handle: handle, version: version}
}

// recomputeLatestLocked re-evaluates which version of a handle is the latest
// (most recently created) and refreshes templateIdByGroupVersionID accordingly. The
// caller must hold cs.mu.
func (cs *ConfigStore) recomputeLatestLocked(handle string) {
	var latestID string
	var latestAt time.Time
	for id, t := range cs.templates {
		if strings.TrimSpace(t.GetGroupVersionID()) != handle {
			continue
		}
		if latestID == "" || !t.CreatedAt.Before(latestAt) {
			latestID = id
			latestAt = t.CreatedAt
		}
	}
	if latestID == "" {
		delete(cs.templateIdByGroupVersionID, handle)
		return
	}
	cs.templateIdByGroupVersionID[handle] = latestID
}

// AddTemplate adds a new LLM provider template version. UUID must be unique and
// immutable; (handle, version) must be unique. Multiple versions of the same
// handle may coexist; the most recently created one resolves as the latest.
func (cs *ConfigStore) AddTemplate(template *models.StoredLLMProviderTemplate) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Normalize inputs
	uuid := strings.TrimSpace(template.UUID)
	handle := strings.TrimSpace(template.GetGroupVersionID())
	version := template.GetVersion()

	if uuid == "" || handle == "" {
		return fmt.Errorf("template UUID and handle is required")
	}

	// Enforce unique immutable UUID: cannot add if UUID already exists
	if _, exists := cs.templates[uuid]; exists {
		return fmt.Errorf("template with uuid '%s' already exists", uuid)
	}

	// Enforce unique (handle, version): a given version of a handle is immutable
	hvKey := groupVersionIDAndVersionKey(handle, version)
	if _, exists := cs.templateIdByGroupVersionIDAndVersion[hvKey]; exists {
		return fmt.Errorf("template with handle '%s' and version '%s' already exists", handle, version)
	}

	// Store
	cs.templates[uuid] = template
	cs.templateIdByGroupVersionIDAndVersion[hvKey] = uuid
	cs.recomputeLatestLocked(handle)
	return nil
}

// UpdateTemplate updates an existing LLM provider template version in place. The
// UUID is immutable; the handle and version may change as long as the resulting
// (handle, version) does not collide with a different template.
func (cs *ConfigStore) UpdateTemplate(template *models.StoredLLMProviderTemplate) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Normalize inputs
	uuid := strings.TrimSpace(template.UUID)
	newHandle := strings.TrimSpace(template.GetGroupVersionID())

	if uuid == "" || newHandle == "" {
		return fmt.Errorf("template uuid and handle is required")
	}

	// Require existing template by ID (ID is immutable)
	existing, exists := cs.templates[uuid]
	if !exists {
		return fmt.Errorf("template with uuid '%s' not found", uuid)
	}

	oldHandle := strings.TrimSpace(existing.GetGroupVersionID())
	oldKey := groupVersionIDAndVersionKey(oldHandle, existing.GetVersion())
	newKey := groupVersionIDAndVersionKey(newHandle, template.GetVersion())

	// Ensure the new (handle, version) does not collide with a different template
	if newKey != oldKey {
		if mappedID, exists := cs.templateIdByGroupVersionIDAndVersion[newKey]; exists && mappedID != uuid {
			return fmt.Errorf("template with given handle '%s' and version '%s' already exists", newHandle, template.GetVersion())
		}
		delete(cs.templateIdByGroupVersionIDAndVersion, oldKey)
	}

	// Update stored template and refresh indexes
	cs.templates[uuid] = template
	cs.templateIdByGroupVersionIDAndVersion[newKey] = uuid
	if oldHandle != newHandle {
		cs.recomputeLatestLocked(oldHandle)
	}
	cs.recomputeLatestLocked(newHandle)
	return nil
}

// DeleteTemplate removes an LLM provider template version from the store by ID
func (cs *ConfigStore) DeleteTemplate(id string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	template, exists := cs.templates[id]
	if !exists {
		return fmt.Errorf("template with ID '%s' not found", id)
	}

	handle := template.GetGroupVersionID()
	delete(cs.templates, id)
	delete(cs.templateIdByGroupVersionIDAndVersion, groupVersionIDAndVersionKey(handle, template.GetVersion()))
	cs.recomputeLatestLocked(handle)
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

// GetTemplateByGroupVersionID retrieves an LLM provider template by handle identifier
func (cs *ConfigStore) GetTemplateByGroupVersionID(handle string) (*models.StoredLLMProviderTemplate, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	templateId, exists := cs.templateIdByGroupVersionID[handle]
	if !exists {
		return nil, fmt.Errorf("template with handle '%s' not found", handle)
	}

	return cs.templates[templateId], nil
}

// GetTemplateByGroupVersionIDAndVersion retrieves the specific (handle, version) template.
func (cs *ConfigStore) GetTemplateByGroupVersionIDAndVersion(handle, version string) (*models.StoredLLMProviderTemplate, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	templateId, exists := cs.templateIdByGroupVersionIDAndVersion[groupVersionIDAndVersionKey(strings.TrimSpace(handle), version)]
	if !exists {
		return nil, fmt.Errorf("template with handle '%s' and version '%s' not found", handle, version)
	}

	return cs.templates[templateId], nil
}

// GetTemplateByID resolves a template by its version-specific id
// ("<handle>-<sanitized-version>", e.g. "openai-v1-0"). For backward
// compatibility it falls back to a base-handle lookup (returning the latest
// version) when the id does not match any versioned template.
func (cs *ConfigStore) GetTemplateByID(id string) (*models.StoredLLMProviderTemplate, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	for _, template := range cs.templates {
		if template.GetID() == id {
			return template, nil
		}
	}

	// Fallback: treat the id as a base handle and resolve the latest version.
	if templateId, exists := cs.templateIdByGroupVersionID[id]; exists {
		return cs.templates[templateId], nil
	}

	return nil, fmt.Errorf("template with id '%s' not found", id)
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
	if strings.TrimSpace(apiKey.ArtifactUUID) == "" {
		return fmt.Errorf("API apiId cannot be empty")
	}

	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Check if an API key with the same apiId and name already exists
	existingKeys, apiIdExists := cs.apiKeysByAPI[apiKey.ArtifactUUID]
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
		// Prevent replacing an existing key's identity — the incoming key must have the same UUID
		if apiKey.UUID != existingKeyID {
			return fmt.Errorf("API key with name %q already exists for artifact %s with a different ID", apiKey.Name, apiKey.ArtifactUUID)
		}
		// Update the existing entry in apiKeysByAPI
		cs.apiKeysByAPI[apiKey.ArtifactUUID][existingKeyID] = apiKey
	} else {
		// Insert new API key
		// Check if API key ID already exists
		if _, exists := cs.apiKeysByAPI[apiKey.ArtifactUUID][apiKey.UUID]; exists {
			return ErrConflict
		}

		// Initialize the map for this API ID if it doesn't exist
		if cs.apiKeysByAPI[apiKey.ArtifactUUID] == nil {
			cs.apiKeysByAPI[apiKey.ArtifactUUID] = make(map[string]*models.APIKey)
		}

		// Store API key by API ID and API key ID
		cs.apiKeysByAPI[apiKey.ArtifactUUID][apiKey.UUID] = apiKey
	}

	return nil
}

// GetAPIKeyByID retrieves an API key by its ID
func (cs *ConfigStore) GetAPIKeyByID(apiId, id string) (*models.APIKey, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	apiKey, exists := cs.apiKeysByAPI[apiId][id]
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

	// Convert map values to slice and return a copy to prevent external modification
	result := make([]*models.APIKey, 0, len(apiKeys))
	for _, apiKey := range apiKeys {
		result = append(result, apiKey)
	}
	return result, nil
}

// CountActiveAPIKeysByUserAndAPI counts active API keys for a specific user and API
func (cs *ConfigStore) CountActiveAPIKeysByUserAndAPI(apiId, userID string) (int, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	apiKeys, exists := cs.apiKeysByAPI[apiId]
	if !exists {
		return 0, nil
	}

	count := 0
	for _, apiKey := range apiKeys {
		if apiKey.CreatedBy == userID && apiKey.Status == models.APIKeyStatusActive {
			count++
		}
	}
	return count, nil
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

// RemoveAPIKeyByID removes an API key from the in-memory cache by its ID
func (cs *ConfigStore) RemoveAPIKeyByID(apiId, id string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	apiKeys, exists := cs.apiKeysByAPI[apiId]
	if !exists {
		return ErrNotFound
	}
	_, exists = apiKeys[id]
	if !exists {
		return ErrNotFound
	}

	// Remove from apiKeysByAPI map
	delete(apiKeys, id)

	// Clean up empty maps
	if len(cs.apiKeysByAPI[apiId]) == 0 {
		delete(cs.apiKeysByAPI, apiId)
	}

	return nil
}

// RemoveAPIKeysByAPI removes all API keys for a specific API
func (cs *ConfigStore) RemoveAPIKeysByAPI(apiId string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	_, exists := cs.apiKeysByAPI[apiId]
	if !exists {
		return nil // No keys to remove
	}

	// Remove from API-specific map
	delete(cs.apiKeysByAPI, apiId)
	return nil
}

// GetLabels retrieves labels for an API
func (cs *ConfigStore) GetLabelsMap(handle string) (map[string]string, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	labels, exists := cs.labelsByAPI[handle]
	if !exists {
		return nil, ErrNotFound
	}

	// Return a copy to prevent external modification
	labelsCopy := make(map[string]string)
	for k, v := range labels {
		labelsCopy[k] = v
	}

	return labelsCopy, nil
}
