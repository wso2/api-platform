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
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

func createTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewAPIKeyStore(t *testing.T) {
	logger := createTestLogger()
	store := NewAPIKeyStore(logger)

	assert.NotNil(t, store)
	assert.NotNil(t, store.apiKeys)
	assert.NotNil(t, store.apiKeysByAPI)
	assert.Equal(t, int64(0), store.GetResourceVersion())
}

func TestAPIKeyStore_Store(t *testing.T) {
	logger := createTestLogger()
	store := NewAPIKeyStore(logger)

	apiKey := &models.APIKey{
		ID:     "key-1",
		Name:   "test-key",
		APIKey: "hashed-value",
		APIId:  "api-1",
		Status: models.APIKeyStatusActive,
	}

	err := store.Store(apiKey)
	require.NoError(t, err)

	assert.Equal(t, 1, store.Count())
}

func TestAPIKeyStore_Store_Update(t *testing.T) {
	logger := createTestLogger()
	store := NewAPIKeyStore(logger)

	// Store initial key
	apiKey := &models.APIKey{
		ID:     "key-1",
		Name:   "test-key",
		APIKey: "hashed-value-1",
		APIId:  "api-1",
		Status: models.APIKeyStatusActive,
	}
	err := store.Store(apiKey)
	require.NoError(t, err)

	// Store updated key with same name and same ID (rotation scenario)
	updatedKey := &models.APIKey{
		ID:     "key-1",
		Name:   "test-key",
		APIKey: "hashed-value-2",
		APIId:  "api-1",
		Status: models.APIKeyStatusActive,
	}
	err = store.Store(updatedKey)
	require.NoError(t, err)

	// Should still have only 1 key (the updated one replaced the old one)
	assert.Equal(t, 1, store.Count())

	// Verify the stored key has the updated API key value
	allKeys := store.GetAll()
	require.Len(t, allKeys, 1)
	assert.Equal(t, "key-1", allKeys[0].ID)
	assert.Equal(t, "test-key", allKeys[0].Name)
	assert.Equal(t, "hashed-value-2", allKeys[0].APIKey)
}

func TestAPIKeyStore_GetAll(t *testing.T) {
	logger := createTestLogger()
	store := NewAPIKeyStore(logger)

	// Add multiple keys
	for i := 1; i <= 3; i++ {
		apiKey := &models.APIKey{
			ID:     "key-" + string(rune('0'+i)),
			Name:   "test-key-" + string(rune('0'+i)),
			APIKey: "value-" + string(rune('0'+i)),
			APIId:  "api-1",
			Status: models.APIKeyStatusActive,
		}
		err := store.Store(apiKey)
		require.NoError(t, err)
	}

	all := store.GetAll()
	assert.Len(t, all, 3)
}

func TestAPIKeyStore_Revoke(t *testing.T) {
	logger := createTestLogger()
	store := NewAPIKeyStore(logger)

	apiKey := &models.APIKey{
		ID:     "key-1",
		Name:   "test-key",
		APIKey: "hashed-value",
		APIId:  "api-1",
		Status: models.APIKeyStatusActive,
	}
	err := store.Store(apiKey)
	require.NoError(t, err)

	// Revoke the key
	success := store.Revoke("api-1", "test-key")
	assert.True(t, success)

	// Count should now be 0 (revoked keys are removed)
	assert.Equal(t, 0, store.Count())
}

func TestAPIKeyStore_Revoke_NonExistent(t *testing.T) {
	logger := createTestLogger()
	store := NewAPIKeyStore(logger)

	// Try to revoke non-existent key
	success := store.Revoke("api-1", "non-existent")
	assert.False(t, success)
}

func TestAPIKeyStore_RemoveByAPI(t *testing.T) {
	logger := createTestLogger()
	store := NewAPIKeyStore(logger)

	// Add keys for multiple APIs
	for i := 1; i <= 3; i++ {
		apiKey := &models.APIKey{
			ID:     "key-" + string(rune('0'+i)),
			Name:   "test-key-" + string(rune('0'+i)),
			APIKey: "value-" + string(rune('0'+i)),
			APIId:  "api-1",
			Status: models.APIKeyStatusActive,
		}
		err := store.Store(apiKey)
		require.NoError(t, err)
	}

	apiKey := &models.APIKey{
		ID:     "key-other",
		Name:   "other-key",
		APIKey: "other-value",
		APIId:  "api-2",
		Status: models.APIKeyStatusActive,
	}
	err := store.Store(apiKey)
	require.NoError(t, err)

	assert.Equal(t, 4, store.Count())

	// Remove keys for api-1
	removed := store.RemoveByAPI("api-1")
	assert.Equal(t, 3, removed)
	assert.Equal(t, 1, store.Count())

	// Remove from non-existent API
	removed = store.RemoveByAPI("non-existent")
	assert.Equal(t, 0, removed)
}

func TestAPIKeyStore_ResourceVersion(t *testing.T) {
	logger := createTestLogger()
	store := NewAPIKeyStore(logger)

	// Initial version
	assert.Equal(t, int64(0), store.GetResourceVersion())

	// Increment
	v1 := store.IncrementResourceVersion()
	assert.Equal(t, int64(1), v1)
	assert.Equal(t, int64(1), store.GetResourceVersion())

	// Increment again
	v2 := store.IncrementResourceVersion()
	assert.Equal(t, int64(2), v2)
}

func TestAPIKeyStore_ConcurrentAccess(t *testing.T) {
	logger := createTestLogger()
	store := NewAPIKeyStore(logger)

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			apiKey := &models.APIKey{
				ID:     "key-" + string(rune('a'+idx)),
				Name:   "test-key-" + string(rune('a'+idx)),
				APIKey: "value-" + string(rune('a'+idx)),
				APIId:  "api-1",
				Status: models.APIKeyStatusActive,
			}
			_ = store.Store(apiKey)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = store.GetAll()
			_ = store.Count()
		}()
	}

	wg.Wait()
}

func TestGetCompositeKey(t *testing.T) {
	tests := []struct {
		apiId    string
		keyName  string
		expected string
	}{
		{"api-1", "key-1", "api-1:key-1"},
		{"", "key-1", ":key-1"},
		{"api-1", "", "api-1:"},
		{"", "", ":"},
		{"api/v1", "test:key", "api/v1:test:key"},
	}

	for _, tt := range tests {
		result := GetCompositeKey(tt.apiId, tt.keyName)
		assert.Equal(t, tt.expected, result)
	}
}

func TestAPIKeyStore_addToAPIMapping(t *testing.T) {
	logger := createTestLogger()
	store := NewAPIKeyStore(logger)

	apiKey := &models.APIKey{
		ID:    "key-1",
		Name:  "test-key",
		APIId: "api-1",
	}

	store.mu.Lock()
	store.addToAPIMapping(apiKey)
	store.mu.Unlock()

	store.mu.RLock()
	defer store.mu.RUnlock()

	assert.Contains(t, store.apiKeysByAPI, "api-1")
	assert.Contains(t, store.apiKeysByAPI["api-1"], "key-1")
}

func TestAPIKeyStore_removeFromAPIMapping(t *testing.T) {
	logger := createTestLogger()
	store := NewAPIKeyStore(logger)

	apiKey := &models.APIKey{
		ID:    "key-1",
		Name:  "test-key",
		APIId: "api-1",
	}

	// First add the key
	store.mu.Lock()
	store.addToAPIMapping(apiKey)
	store.mu.Unlock()

	// Then remove it
	store.mu.Lock()
	store.removeFromAPIMapping(apiKey)
	store.mu.Unlock()

	store.mu.RLock()
	defer store.mu.RUnlock()

	// API mapping should be cleaned up completely
	assert.NotContains(t, store.apiKeysByAPI, "api-1")
}
