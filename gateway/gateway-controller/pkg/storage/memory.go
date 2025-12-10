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

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// ConfigStore holds all API configurations in memory for fast access
type ConfigStore struct {
	mu           sync.RWMutex                    // Protects concurrent access
	configs      map[string]*models.StoredConfig // Key: config ID
	nameVersion  map[string]string               // Key: "name:version" → Value: config ID
	snapVersion  int64                           // Current xDS snapshot version
	TopicManager *TopicManager

	// LLM Provider Templates
	templates        map[string]*models.StoredLLMProviderTemplate // Key: template ID
	templateIdByName map[string]string
}

// NewConfigStore creates a new in-memory config store
func NewConfigStore() *ConfigStore {
	return &ConfigStore{
		configs:          make(map[string]*models.StoredConfig),
		nameVersion:      make(map[string]string),
		snapVersion:      0,
		TopicManager:     NewTopicManager(),
		templates:        make(map[string]*models.StoredLLMProviderTemplate),
		templateIdByName: make(map[string]string),
	}
}

// Add stores a new configuration in memory
func (cs *ConfigStore) Add(cfg *models.StoredConfig) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	key := cfg.GetCompositeKey()
	if existingID, exists := cs.nameVersion[key]; exists {
		return fmt.Errorf("%w: configuration with name '%s' and version '%s' already exists (ID: %s)",
			ErrConflict, cfg.GetName(), cfg.GetVersion(), existingID)
	}

	cs.configs[cfg.ID] = cfg
	cs.nameVersion[key] = cfg.ID

	if cfg.Configuration.Kind == "async/websub" {
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

	// If name/version changed, update the nameVersion index
	oldKey := existing.GetCompositeKey()
	newKey := cfg.GetCompositeKey()

	if oldKey != newKey {
		// Check if new name:version combination already exists
		if existingID, exists := cs.nameVersion[newKey]; exists && existingID != cfg.ID {
			return fmt.Errorf("%w: configuration with name '%s' and version '%s' already exists (ID: %s)",
				ErrConflict, cfg.GetName(), cfg.GetVersion(), existingID)
		}
		delete(cs.nameVersion, oldKey)
		cs.nameVersion[newKey] = cfg.ID
	}

	if cfg.Configuration.Kind == "async/websub" {
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
		name := strings.TrimPrefix(asyncData.Name, "/")
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

	if cfg.Configuration.Kind == "async/websub" {
		cs.TopicManager.RemoveAllForConfig(cfg.ID)
	}
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
		if sc.GetName() == name && sc.GetVersion() == version {
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

// AddTemplate adds an LLM provider template to the store
func (cs *ConfigStore) AddTemplate(template *models.StoredLLMProviderTemplate) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Check if template with same name already exists
	if _, exists := cs.templateIdByName[template.GetName()]; exists {
		return fmt.Errorf("template with name '%s' already exists", template.GetName())
	}

	cs.templates[template.ID] = template
	cs.templateIdByName[template.GetName()] = template.ID
	return nil
}

// UpdateTemplate updates an existing LLM provider template in the store
func (cs *ConfigStore) UpdateTemplate(template *models.StoredLLMProviderTemplate) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	existing, exists := cs.templates[template.ID]
	if !exists {
		return fmt.Errorf("template with ID '%s' not found", template.ID)
	}

	// Remove old name mapping if name changed
	if existing.GetName() != template.GetName() {
		delete(cs.templateIdByName, existing.GetName())
	}

	cs.templates[template.ID] = template
	cs.templateIdByName[template.GetName()] = template.ID
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
	delete(cs.templateIdByName, template.GetName())
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

// GetTemplateByName retrieves an LLM provider template by name
func (cs *ConfigStore) GetTemplateByName(name string) (*models.StoredLLMProviderTemplate, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	templateId, exists := cs.templateIdByName[name]
	if !exists {
		return nil, fmt.Errorf("template with name '%s' not found", name)
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
