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

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

type apiKeyStoreWriter interface {
	Store(*models.APIKey) error
}

// LoadFromDatabase loads all configurations from database into the in-memory cache.
func LoadFromDatabase(storage Storage, cache *ConfigStore) error {
	configs, err := storage.GetAllConfigs()
	if err != nil {
		return fmt.Errorf("failed to load configurations from database: %w", err)
	}

	for _, cfg := range configs {
		if err := cache.Add(cfg); err != nil {
			return fmt.Errorf("failed to load config %s into cache: %w", cfg.UUID, err)
		}
	}

	return nil
}

// LoadLLMProviderTemplatesFromDatabase loads all LLM Provider templates from database into in-memory store.
func LoadLLMProviderTemplatesFromDatabase(storage Storage, cache *ConfigStore) error {
	templates, err := storage.GetAllLLMProviderTemplates()
	if err != nil {
		return fmt.Errorf("failed to load templates from database: %w", err)
	}

	for _, template := range templates {
		if err := cache.AddTemplate(template); err != nil {
			return fmt.Errorf("failed to load llm provider template %s into cache: %w", template.GetHandle(), err)
		}
	}

	return nil
}

// LoadAPIKeysFromDatabase loads active API keys from database into both the ConfigStore and APIKeyStore.
func LoadAPIKeysFromDatabase(storage Storage, configStore *ConfigStore, apiKeyStore apiKeyStoreWriter) error {
	apiKeys, err := storage.GetAllAPIKeys()
	if err != nil {
		return fmt.Errorf("failed to load API keys from database: %w", err)
	}

	for _, apiKey := range apiKeys {
		if err := configStore.StoreAPIKey(apiKey); err != nil {
			return fmt.Errorf("failed to load API key %s into ConfigStore: %w", apiKey.UUID, err)
		}

		if err := storeAPIKeySafely(apiKeyStore, apiKey); err != nil {
			rollbackErr := configStore.RemoveAPIKeyByID(apiKey.ArtifactUUID, apiKey.UUID)
			if rollbackErr != nil {
				return fmt.Errorf("failed to load API key %s into APIKeyStore: %w; failed to rollback ConfigStore entry: %v", apiKey.UUID, err, rollbackErr)
			}
			return fmt.Errorf("failed to load API key %s into APIKeyStore: %w (rolled back ConfigStore entry)", apiKey.UUID, err)
		}
	}

	return nil
}

func storeAPIKeySafely(apiKeyStore apiKeyStoreWriter, apiKey *models.APIKey) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in apiKeyStore.Store for API key %s: %v", apiKey.UUID, r)
		}
	}()

	return apiKeyStore.Store(apiKey)
}
