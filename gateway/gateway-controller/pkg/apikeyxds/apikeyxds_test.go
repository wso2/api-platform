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

package apikeyxds

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		expected string
	}{
		{"short key", "abc", "****"},
		{"exactly 12 chars", "123456789012", "****"},
		{"normal key", "1234567890123456", "12345678****3456"},
		{"long key", "abcdefghijklmnopqrstuvwxyz", "abcdefgh****wxyz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskAPIKey(tt.apiKey)
			if result != tt.expected {
				t.Errorf("MaskAPIKey(%q) = %q, want %q", tt.apiKey, result, tt.expected)
			}
		})
	}
}

func TestAPIKeyStateManager_MaskAPIKey(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewAPIKeyStore(logger)
	snapshotManager := NewAPIKeySnapshotManager(store, logger)
	manager := NewAPIKeyStateManager(store, snapshotManager, logger)

	tests := []struct {
		name     string
		apiKey   string
		expected string
	}{
		{"short key", "abc", "****"},
		{"exactly 12 chars", "123456789012", "****"},
		{"normal key", "1234567890123456", "12345678****3456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.MaskAPIKey(tt.apiKey)
			if result != tt.expected {
				t.Errorf("MaskAPIKey(%q) = %q, want %q", tt.apiKey, result, tt.expected)
			}
		})
	}
}

func TestNewAPIKeySnapshotManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewAPIKeyStore(logger)

	manager := NewAPIKeySnapshotManager(store, logger)
	if manager == nil {
		t.Fatal("NewAPIKeySnapshotManager returned nil")
	}

	if manager.GetCache() == nil {
		t.Error("GetCache() returned nil")
	}
}

func TestNewAPIKeyStateManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewAPIKeyStore(logger)
	snapshotManager := NewAPIKeySnapshotManager(store, logger)

	manager := NewAPIKeyStateManager(store, snapshotManager, logger)
	if manager == nil {
		t.Fatal("NewAPIKeyStateManager returned nil")
	}

	// Test GetAPIKeyCount when empty
	if count := manager.GetAPIKeyCount(); count != 0 {
		t.Errorf("GetAPIKeyCount() = %d, want 0", count)
	}
}

func TestAPIKeyTranslator(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	translator := NewAPIKeyTranslator(logger)

	if translator == nil {
		t.Fatal("NewAPIKeyTranslator returned nil")
	}

	// Test with empty API keys
	t.Run("empty api keys", func(t *testing.T) {
		resources, err := translator.TranslateAPIKeys([]*models.APIKey{})
		if err != nil {
			t.Fatalf("TranslateAPIKeys failed: %v", err)
		}
		if resources == nil {
			t.Error("TranslateAPIKeys returned nil resources")
		}
		if _, ok := resources[APIKeyStateTypeURL]; !ok {
			t.Error("Expected APIKeyStateTypeURL in resources")
		}
	})

	// Test with API keys
	t.Run("with api keys", func(t *testing.T) {
		now := time.Now()
		expires := now.Add(24 * time.Hour)
		apiKeys := []*models.APIKey{
			{
				ID:         "key1",
				Name:       "test-key-1",
				APIKey:     "apikey123456789",
				APIId:      "api1",
				Operations: "*",
				Status:     models.APIKeyStatusActive,
				CreatedAt:  now,
				CreatedBy:  "user1",
				UpdatedAt:  now,
				ExpiresAt:  &expires,
			},
			{
				ID:         "key2",
				Name:       "test-key-2",
				APIKey:     "apikey987654321",
				APIId:      "api2",
				Operations: "GET,POST",
				Status:     models.APIKeyStatusActive,
				CreatedAt:  now,
				CreatedBy:  "user2",
				UpdatedAt:  now,
				ExpiresAt:  nil,
			},
		}

		resources, err := translator.TranslateAPIKeys(apiKeys)
		if err != nil {
			t.Fatalf("TranslateAPIKeys failed: %v", err)
		}
		if resources == nil {
			t.Error("TranslateAPIKeys returned nil resources")
		}
		if len(resources[APIKeyStateTypeURL]) != 1 {
			t.Errorf("Expected 1 resource, got %d", len(resources[APIKeyStateTypeURL]))
		}
	})
}

func TestAPIKeySnapshotManager_UpdateSnapshot(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewAPIKeyStore(logger)
	manager := NewAPIKeySnapshotManager(store, logger)

	// Test update with empty store
	t.Run("empty store", func(t *testing.T) {
		err := manager.UpdateSnapshot(nil)
		if err != nil {
			t.Errorf("UpdateSnapshot failed: %v", err)
		}
	})

	// Add an API key and update
	t.Run("with api key", func(t *testing.T) {
		apiKey := &models.APIKey{
			ID:         "key1",
			Name:       "test-key",
			APIKey:     "apikey123456789",
			APIId:      "api1",
			Operations: "*",
			Status:     models.APIKeyStatusActive,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		store.Store(apiKey)

		err := manager.UpdateSnapshot(nil)
		if err != nil {
			t.Errorf("UpdateSnapshot failed: %v", err)
		}
	})
}

func TestAPIKeySnapshotManager_StoreAndRevoke(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewAPIKeyStore(logger)
	manager := NewAPIKeySnapshotManager(store, logger)

	apiKey := &models.APIKey{
		ID:         "key1",
		Name:       "test-key",
		APIKey:     "apikey123456789",
		APIId:      "api1",
		Operations: "*",
		Status:     models.APIKeyStatusActive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Store API key
	err := manager.StoreAPIKey(apiKey)
	if err != nil {
		t.Fatalf("StoreAPIKey failed: %v", err)
	}

	// Revoke API key
	err = manager.RevokeAPIKey("api1", "test-key")
	if err != nil {
		t.Errorf("RevokeAPIKey failed: %v", err)
	}

	// Revoke non-existent key (should not error)
	err = manager.RevokeAPIKey("api1", "non-existent")
	if err != nil {
		t.Errorf("RevokeAPIKey for non-existent key failed: %v", err)
	}
}

func TestAPIKeySnapshotManager_RemoveAPIKeysByAPI(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewAPIKeyStore(logger)
	manager := NewAPIKeySnapshotManager(store, logger)

	// Add multiple API keys
	for i := 0; i < 3; i++ {
		apiKey := &models.APIKey{
			ID:         "key" + string(rune('1'+i)),
			Name:       "test-key-" + string(rune('1'+i)),
			APIKey:     "apikey" + string(rune('1'+i)),
			APIId:      "api1",
			Operations: "*",
			Status:     models.APIKeyStatusActive,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		store.Store(apiKey)
	}

	// Remove all keys for api1
	err := manager.RemoveAPIKeysByAPI("api1")
	if err != nil {
		t.Fatalf("RemoveAPIKeysByAPI failed: %v", err)
	}
}

func TestAPIKeyStateManager_StoreAPIKey(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewAPIKeyStore(logger)
	snapshotManager := NewAPIKeySnapshotManager(store, logger)
	manager := NewAPIKeyStateManager(store, snapshotManager, logger)

	apiKey := &models.APIKey{
		ID:         "key1",
		Name:       "test-key",
		APIKey:     "apikey123456789",
		APIId:      "api1",
		Operations: "*",
		Status:     models.APIKeyStatusActive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	err := manager.StoreAPIKey("api1", "TestAPI", "v1", apiKey, "correlation-123")
	if err != nil {
		t.Fatalf("StoreAPIKey failed: %v", err)
	}

	if count := manager.GetAPIKeyCount(); count != 1 {
		t.Errorf("GetAPIKeyCount() = %d, want 1", count)
	}
}

func TestAPIKeyStateManager_RevokeAPIKey(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewAPIKeyStore(logger)
	snapshotManager := NewAPIKeySnapshotManager(store, logger)
	manager := NewAPIKeyStateManager(store, snapshotManager, logger)

	// Store first
	apiKey := &models.APIKey{
		ID:         "key1",
		Name:       "test-key",
		APIKey:     "apikey123456789",
		APIId:      "api1",
		Operations: "*",
		Status:     models.APIKeyStatusActive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	store.Store(apiKey)

	err := manager.RevokeAPIKey("api1", "TestAPI", "v1", "test-key", "correlation-123")
	if err != nil {
		t.Fatalf("RevokeAPIKey failed: %v", err)
	}
}

func TestAPIKeyStateManager_RemoveAPIKeysByAPI(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewAPIKeyStore(logger)
	snapshotManager := NewAPIKeySnapshotManager(store, logger)
	manager := NewAPIKeyStateManager(store, snapshotManager, logger)

	// Store first
	apiKey := &models.APIKey{
		ID:         "key1",
		Name:       "test-key",
		APIKey:     "apikey123456789",
		APIId:      "api1",
		Operations: "*",
		Status:     models.APIKeyStatusActive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	store.Store(apiKey)

	err := manager.RemoveAPIKeysByAPI("api1", "TestAPI", "v1", "correlation-123")
	if err != nil {
		t.Fatalf("RemoveAPIKeysByAPI failed: %v", err)
	}

	if count := manager.GetAPIKeyCount(); count != 0 {
		t.Errorf("GetAPIKeyCount() = %d, want 0", count)
	}
}

func TestAPIKeyStateManager_RefreshSnapshot(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewAPIKeyStore(logger)
	snapshotManager := NewAPIKeySnapshotManager(store, logger)
	manager := NewAPIKeyStateManager(store, snapshotManager, logger)

	err := manager.RefreshSnapshot()
	if err != nil {
		t.Fatalf("RefreshSnapshot failed: %v", err)
	}
}
