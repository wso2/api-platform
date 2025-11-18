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

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// ConfigStore holds all API configurations in memory for fast access
type ConfigStore struct {
	mu           sync.RWMutex                       // Protects concurrent access
	configs      map[string]*models.StoredAPIConfig // Key: config ID
	nameVersion  map[string]string                  // Key: "name:version" → Value: config ID
	snapVersion  int64                              // Current xDS snapshot version
	TopicManager *TopicManager
}

// NewConfigStore creates a new in-memory config store
func NewConfigStore() *ConfigStore {
	return &ConfigStore{
		configs:      make(map[string]*models.StoredAPIConfig),
		nameVersion:  make(map[string]string),
		snapVersion:  0,
		TopicManager: NewTopicManager(),
	}
}

// Add stores a new configuration in memory
func (cs *ConfigStore) Add(cfg *models.StoredAPIConfig) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	key := cfg.GetCompositeKey()
	if existingID, exists := cs.nameVersion[key]; exists {
		return fmt.Errorf("%w: configuration with name '%s' and version '%s' already exists (ID: %s)",
			ErrConflict, cfg.GetAPIName(), cfg.GetAPIVersion(), existingID)
	}

	cs.configs[cfg.ID] = cfg
	cs.nameVersion[key] = cfg.ID

	if cfg.Configuration.Kind == "async/websub" {
		asyncData, err := cfg.Configuration.Data.AsWebhookAPIData()
		if err != nil {
			return fmt.Errorf("failed to parse async API data: %w", err)
		}
		// Maintaining a topic map to process topics
		// Running these inside Add or Delete configs might add extra latency to the API Deployment process
		// TODO: Optimize topic management if needed by maintaining a separate topic manager struct
		for _, topic := range asyncData.Channels {
			modifiedTopic := fmt.Sprintf("%s/%s%s", asyncData.Context, asyncData.Version, topic.Path)
			if _, exist := cs.TopicManager.topics[modifiedTopic]; !exist {
				cs.TopicManager.Add(cfg.ID, modifiedTopic)
				// cs.topics[modifiedTopic] = cfg.Configuration.Data.Name
			}
		}

		apiTopicsPerRevision := make(map[string]bool)
		for _, topic := range asyncData.Channels {
			modifiedTopic := fmt.Sprintf("%s/%s%s", asyncData.Context, asyncData.Version, topic.Path)
			apiTopicsPerRevision[modifiedTopic] = true
		}

		for topic := range cs.TopicManager.topics {
			if _, exists := apiTopicsPerRevision[topic]; !exists {
				cs.TopicManager.Remove(cfg.ID, topic)
				//delete(cs.topics, topic)
			}
		}
	}
	return nil
}

// Update modifies an existing configuration in memory
func (cs *ConfigStore) Update(cfg *models.StoredAPIConfig) error {
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
				ErrConflict, cfg.GetAPIName(), cfg.GetAPIVersion(), existingID)
		}
		delete(cs.nameVersion, oldKey)
		cs.nameVersion[newKey] = cfg.ID
	}

	cs.configs[cfg.ID] = cfg
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
		asyncData, err := cfg.Configuration.Data.AsWebhookAPIData()
		if err != nil {
			return fmt.Errorf("failed to parse async API data: %w", err)
		}
		// Maintaining a topic map to process topics
		// Running these inside Add or Delete configs might add extra latency to the API Deployment process
		// TODO: Optimize topic management if needed by maintaining a separate topic manager struct
		for _, topic := range asyncData.Channels {
			modifiedTopic := fmt.Sprintf("%s/%s/%s", asyncData.Context, asyncData.Version, topic.Path)
			if _, exist := cs.TopicManager.topics[modifiedTopic]; !exist {
				cs.TopicManager.Add(cfg.ID, modifiedTopic)
				//cs.TopicManager.topics[modifiedTopic] = cfg.Configuration.Data.Name
			}
		}

		apiTopicsPerRevision := make(map[string]bool)
		for _, topic := range asyncData.Channels {
			modifiedTopic := fmt.Sprintf("%s/%s/%s", asyncData.Context, asyncData.Version, topic.Path)
			apiTopicsPerRevision[modifiedTopic] = true
		}

		for _, topic := range cs.TopicManager.GetAllForConfig(cfg.ID) {
			if _, exists := apiTopicsPerRevision[topic]; !exists {
				cs.TopicManager.Remove(cfg.ID, topic)
				//delete(cs.TopicManager.topics, topic)
			}
		}
	}
	delete(cs.nameVersion, key)
	delete(cs.configs, id)
	return nil
}

// Get retrieves a configuration by ID
func (cs *ConfigStore) Get(id string) (*models.StoredAPIConfig, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	cfg, exists := cs.configs[id]
	if !exists {
		return nil, fmt.Errorf("configuration with ID '%s' not found", id)
	}
	return cfg, nil
}

// func (cs *ConfigStore) GetAllTopics() map[string]string {
// 	cs.mu.RLock()
// 	defer cs.mu.RUnlock()
// 	result := make(map[string]string)
// 	for topic, apiId := range cs.topics {
// 		result[topic] = apiId
// 	}
// 	return result
// }

// GetByNameVersion retrieves a configuration by name and version
func (cs *ConfigStore) GetByNameVersion(name, version string) (*models.StoredAPIConfig, error) {
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
func (cs *ConfigStore) GetAll() []*models.StoredAPIConfig {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	result := make([]*models.StoredAPIConfig, 0, len(cs.configs))
	for _, cfg := range cs.configs {
		result = append(result, cfg)
	}
	return result
}

func (cs *ConfigStore) GetAllAsyncAPIs() []*models.StoredAPIConfig {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	var result []*models.StoredAPIConfig
	for _, cfg := range cs.configs {
		if cfg.Configuration.Kind == api.APIConfigurationKindAsyncwebsub {
			result = append(result, cfg)
		}
	}
	return result
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
