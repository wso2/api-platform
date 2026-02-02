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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

func TestNewConfigStore(t *testing.T) {
	cs := NewConfigStore()

	assert.NotNil(t, cs)
	assert.NotNil(t, cs.configs)
	assert.NotNil(t, cs.nameVersion)
	assert.NotNil(t, cs.handle)
	assert.NotNil(t, cs.templates)
	assert.NotNil(t, cs.templateIdByHandle)
	assert.NotNil(t, cs.apiKeysByAPI)
	assert.NotNil(t, cs.labelsByAPI)
	assert.NotNil(t, cs.TopicManager)
	assert.Equal(t, int64(0), cs.GetSnapshotVersion())
}

func TestConfigStore_SnapshotVersion(t *testing.T) {
	cs := NewConfigStore()

	// Initial version should be 0
	assert.Equal(t, int64(0), cs.GetSnapshotVersion())

	// Increment version
	v1 := cs.IncrementSnapshotVersion()
	assert.Equal(t, int64(1), v1)
	assert.Equal(t, int64(1), cs.GetSnapshotVersion())

	// Increment again
	v2 := cs.IncrementSnapshotVersion()
	assert.Equal(t, int64(2), v2)
	assert.Equal(t, int64(2), cs.GetSnapshotVersion())

	// Set version directly
	cs.SetSnapshotVersion(100)
	assert.Equal(t, int64(100), cs.GetSnapshotVersion())
}

func TestConfigStore_TemplateOperations(t *testing.T) {
	cs := NewConfigStore()

	template := &models.StoredLLMProviderTemplate{
		ID: "template-1",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{
				Name: "openai-template",
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Add template
	err := cs.AddTemplate(template)
	require.NoError(t, err)

	// Get by ID
	retrieved, err := cs.GetTemplate("template-1")
	require.NoError(t, err)
	assert.Equal(t, "openai-template", retrieved.GetHandle())

	// Get by handle
	retrieved, err = cs.GetTemplateByHandle("openai-template")
	require.NoError(t, err)
	assert.Equal(t, "template-1", retrieved.ID)

	// Get all templates
	all := cs.GetAllTemplates()
	assert.Len(t, all, 1)

	// Update template - create a new struct to avoid pointer issues
	updatedTemplate := &models.StoredLLMProviderTemplate{
		ID: "template-1",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{
				Name: "updated-template",
			},
		},
		CreatedAt: template.CreatedAt,
		UpdatedAt: time.Now(),
	}
	err = cs.UpdateTemplate(updatedTemplate)
	require.NoError(t, err)

	// Verify update
	retrieved, err = cs.GetTemplateByHandle("updated-template")
	require.NoError(t, err)
	assert.Equal(t, "template-1", retrieved.ID)

	// Old handle should not work
	_, err = cs.GetTemplateByHandle("openai-template")
	assert.Error(t, err)

	// Delete template
	err = cs.DeleteTemplate("template-1")
	require.NoError(t, err)

	// Verify deletion
	_, err = cs.GetTemplate("template-1")
	assert.Error(t, err)
}

func TestConfigStore_TemplateErrors(t *testing.T) {
	cs := NewConfigStore()

	// Add template with empty ID
	err := cs.AddTemplate(&models.StoredLLMProviderTemplate{
		ID: "",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{
				Name: "test",
			},
		},
	})
	assert.Error(t, err)

	// Add template with empty handle
	err = cs.AddTemplate(&models.StoredLLMProviderTemplate{
		ID: "id-1",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{
				Name: "",
			},
		},
	})
	assert.Error(t, err)

	// Add duplicate ID
	template := &models.StoredLLMProviderTemplate{
		ID: "dup-id",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{
				Name: "handle-1",
			},
		},
	}
	err = cs.AddTemplate(template)
	require.NoError(t, err)

	err = cs.AddTemplate(&models.StoredLLMProviderTemplate{
		ID: "dup-id",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{
				Name: "handle-2",
			},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	// Add duplicate handle
	err = cs.AddTemplate(&models.StoredLLMProviderTemplate{
		ID: "different-id",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{
				Name: "handle-1",
			},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	// Update non-existent template
	err = cs.UpdateTemplate(&models.StoredLLMProviderTemplate{
		ID: "non-existent",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{
				Name: "test",
			},
		},
	})
	assert.Error(t, err)

	// Delete non-existent template
	err = cs.DeleteTemplate("non-existent")
	assert.Error(t, err)

	// Get non-existent template
	_, err = cs.GetTemplate("non-existent")
	assert.Error(t, err)

	_, err = cs.GetTemplateByHandle("non-existent")
	assert.Error(t, err)
}

func TestConfigStore_APIKeyOperations(t *testing.T) {
	cs := NewConfigStore()

	apiKey := &models.APIKey{
		ID:        "key-1",
		Name:      "test-key",
		APIKey:    "hashed-key-value",
		APIId:     "api-1",
		Status:    models.APIKeyStatusActive,
		CreatedBy: "user-1",
		CreatedAt: time.Now(),
	}

	// Store API key
	err := cs.StoreAPIKey(apiKey)
	require.NoError(t, err)

	// Get by ID
	retrieved, err := cs.GetAPIKeyByID("api-1", "key-1")
	require.NoError(t, err)
	assert.Equal(t, "test-key", retrieved.Name)

	// Get by name
	retrieved, err = cs.GetAPIKeyByName("api-1", "test-key")
	require.NoError(t, err)
	assert.Equal(t, "key-1", retrieved.ID)

	// Get all keys for API
	keys, err := cs.GetAPIKeysByAPI("api-1")
	require.NoError(t, err)
	assert.Len(t, keys, 1)

	// Count active keys
	count, err := cs.CountActiveAPIKeysByUserAndAPI("api-1", "user-1")
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Remove API key
	err = cs.RemoveAPIKeyByID("api-1", "key-1")
	require.NoError(t, err)

	// Verify removal
	_, err = cs.GetAPIKeyByID("api-1", "key-1")
	assert.Error(t, err)
}

func TestConfigStore_APIKeyErrors(t *testing.T) {
	cs := NewConfigStore()

	// Store nil key
	err := cs.StoreAPIKey(nil)
	assert.Error(t, err)

	// Store key with empty name
	err = cs.StoreAPIKey(&models.APIKey{
		ID:     "key-1",
		Name:   "",
		APIKey: "value",
		APIId:  "api-1",
	})
	assert.Error(t, err)

	// Store key with empty value
	err = cs.StoreAPIKey(&models.APIKey{
		ID:     "key-1",
		Name:   "test",
		APIKey: "",
		APIId:  "api-1",
	})
	assert.Error(t, err)

	// Store key with empty API ID
	err = cs.StoreAPIKey(&models.APIKey{
		ID:     "key-1",
		Name:   "test",
		APIKey: "value",
		APIId:  "",
	})
	assert.Error(t, err)

	// Get non-existent key
	_, err = cs.GetAPIKeyByID("non-existent", "key-1")
	assert.Error(t, err)

	_, err = cs.GetAPIKeyByName("non-existent", "test")
	assert.Error(t, err)

	// Remove non-existent key
	err = cs.RemoveAPIKeyByID("non-existent", "key-1")
	assert.Error(t, err)
}

func TestConfigStore_RemoveAPIKeysByAPI(t *testing.T) {
	cs := NewConfigStore()

	// Add multiple keys for an API
	for i := 1; i <= 3; i++ {
		err := cs.StoreAPIKey(&models.APIKey{
			ID:        "key-" + string(rune('0'+i)),
			Name:      "test-key-" + string(rune('0'+i)),
			APIKey:    "value-" + string(rune('0'+i)),
			APIId:     "api-1",
			Status:    models.APIKeyStatusActive,
			CreatedBy: "user-1",
		})
		require.NoError(t, err)
	}

	// Verify all keys exist
	keys, err := cs.GetAPIKeysByAPI("api-1")
	require.NoError(t, err)
	assert.Len(t, keys, 3)

	// Remove all keys for API
	err = cs.RemoveAPIKeysByAPI("api-1")
	require.NoError(t, err)

	// Verify all removed
	keys, err = cs.GetAPIKeysByAPI("api-1")
	require.NoError(t, err)
	assert.Len(t, keys, 0)

	// Remove from non-existent API should not error
	err = cs.RemoveAPIKeysByAPI("non-existent")
	assert.NoError(t, err)
}

func TestConfigStore_GetAPIKeysByAPI_EmptyResult(t *testing.T) {
	cs := NewConfigStore()

	// Get keys for non-existent API should return empty slice, not error
	keys, err := cs.GetAPIKeysByAPI("non-existent")
	require.NoError(t, err)
	assert.NotNil(t, keys)
	assert.Len(t, keys, 0)
}

func TestConfigStore_CountActiveAPIKeysByUserAndAPI(t *testing.T) {
	cs := NewConfigStore()

	// Add active key
	err := cs.StoreAPIKey(&models.APIKey{
		ID:        "key-1",
		Name:      "active-key",
		APIKey:    "value-1",
		APIId:     "api-1",
		Status:    models.APIKeyStatusActive,
		CreatedBy: "user-1",
	})
	require.NoError(t, err)

	// Add revoked key
	err = cs.StoreAPIKey(&models.APIKey{
		ID:        "key-2",
		Name:      "revoked-key",
		APIKey:    "value-2",
		APIId:     "api-1",
		Status:    models.APIKeyStatusRevoked,
		CreatedBy: "user-1",
	})
	require.NoError(t, err)

	// Add key for different user
	err = cs.StoreAPIKey(&models.APIKey{
		ID:        "key-3",
		Name:      "other-user-key",
		APIKey:    "value-3",
		APIId:     "api-1",
		Status:    models.APIKeyStatusActive,
		CreatedBy: "user-2",
	})
	require.NoError(t, err)

	// Count for user-1 should be 1 (only active key)
	count, err := cs.CountActiveAPIKeysByUserAndAPI("api-1", "user-1")
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Count for user-2 should be 1
	count, err = cs.CountActiveAPIKeysByUserAndAPI("api-1", "user-2")
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Count for non-existent API should be 0
	count, err = cs.CountActiveAPIKeysByUserAndAPI("non-existent", "user-1")
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}
