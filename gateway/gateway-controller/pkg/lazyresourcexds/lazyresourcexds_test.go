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

package lazyresourcexds

import (
	"log/slog"
	"os"
	"testing"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

func TestNewLazyResourceSnapshotManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewLazyResourceStore(logger)

	manager := NewLazyResourceSnapshotManager(store, logger)
	if manager == nil {
		t.Fatal("NewLazyResourceSnapshotManager returned nil")
	}

	if manager.GetCache() == nil {
		t.Error("GetCache() returned nil")
	}
}

func TestNewLazyResourceStateManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewLazyResourceStore(logger)
	snapshotManager := NewLazyResourceSnapshotManager(store, logger)

	manager := NewLazyResourceStateManager(store, snapshotManager, logger)
	if manager == nil {
		t.Fatal("NewLazyResourceStateManager returned nil")
	}

	// Test GetResourceCount when empty
	if count := manager.GetResourceCount(); count != 0 {
		t.Errorf("GetResourceCount() = %d, want 0", count)
	}
}

func TestLazyResourceTranslator(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	translator := NewLazyResourceTranslator(logger)

	if translator == nil {
		t.Fatal("NewLazyResourceTranslator returned nil")
	}

	// Test with empty resources
	t.Run("empty resources", func(t *testing.T) {
		resources, err := translator.TranslateResources([]*storage.LazyResource{})
		if err != nil {
			t.Fatalf("TranslateResources failed: %v", err)
		}
		if resources == nil {
			t.Error("TranslateResources returned nil resources")
		}
		if _, ok := resources[LazyResourceTypeURL]; !ok {
			t.Error("Expected LazyResourceTypeURL in resources")
		}
	})

	// Test with resources
	t.Run("with resources", func(t *testing.T) {
		lazyResources := []*storage.LazyResource{
			{
				ID:           "res1",
				ResourceType: "llm-provider",
				Resource:     map[string]interface{}{"name": "test-provider", "endpoint": "https://api.example.com"},
			},
			{
				ID:           "res2",
				ResourceType: "mcp-proxy",
				Resource:     map[string]interface{}{"name": "test-proxy", "target": "https://mcp.example.com"},
			},
		}

		resources, err := translator.TranslateResources(lazyResources)
		if err != nil {
			t.Fatalf("TranslateResources failed: %v", err)
		}
		if resources == nil {
			t.Error("TranslateResources returned nil resources")
		}
		if len(resources[LazyResourceTypeURL]) != 1 {
			t.Errorf("Expected 1 resource, got %d", len(resources[LazyResourceTypeURL]))
		}
	})
}

func TestLazyResourceSnapshotManager_UpdateSnapshot(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewLazyResourceStore(logger)
	manager := NewLazyResourceSnapshotManager(store, logger)

	// Test update with empty store
	t.Run("empty store", func(t *testing.T) {
		err := manager.UpdateSnapshot(nil)
		if err != nil {
			t.Errorf("UpdateSnapshot failed: %v", err)
		}
	})

	// Add a resource and update
	t.Run("with resource", func(t *testing.T) {
		resource := &storage.LazyResource{
			ID:           "res1",
			ResourceType: "llm-provider",
			Resource:     map[string]interface{}{"name": "test"},
		}
		store.Store(resource)

		err := manager.UpdateSnapshot(nil)
		if err != nil {
			t.Errorf("UpdateSnapshot failed: %v", err)
		}
	})
}

func TestLazyResourceSnapshotManager_StoreResource(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewLazyResourceStore(logger)
	manager := NewLazyResourceSnapshotManager(store, logger)

	resource := &storage.LazyResource{
		ID:           "res1",
		ResourceType: "llm-provider",
		Resource:     map[string]interface{}{"name": "test"},
	}

	err := manager.StoreResource(resource)
	if err != nil {
		t.Fatalf("StoreResource failed: %v", err)
	}

	// Verify resource is stored
	if store.Count() != 1 {
		t.Errorf("Store count = %d, want 1", store.Count())
	}
}

func TestLazyResourceSnapshotManager_RemoveResourceByIDAndType(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewLazyResourceStore(logger)
	manager := NewLazyResourceSnapshotManager(store, logger)

	// Store a resource first
	resource := &storage.LazyResource{
		ID:           "res1",
		ResourceType: "llm-provider",
		Resource:     map[string]interface{}{"name": "test"},
	}
	store.Store(resource)

	// Remove by ID and type
	err := manager.RemoveResourceByIDAndType("res1", "llm-provider")
	if err != nil {
		t.Fatalf("RemoveResourceByIDAndType failed: %v", err)
	}

	// Verify resource is removed
	if store.Count() != 0 {
		t.Errorf("Store count = %d, want 0", store.Count())
	}

	// Try removing non-existent resource
	err = manager.RemoveResourceByIDAndType("non-existent", "llm-provider")
	if err == nil {
		t.Error("RemoveResourceByIDAndType should fail for non-existent resource")
	}
}

func TestLazyResourceSnapshotManager_RemoveResourcesByType(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewLazyResourceStore(logger)
	manager := NewLazyResourceSnapshotManager(store, logger)

	// Store multiple resources
	store.Store(&storage.LazyResource{ID: "res1", ResourceType: "llm-provider", Resource: map[string]interface{}{}})
	store.Store(&storage.LazyResource{ID: "res2", ResourceType: "llm-provider", Resource: map[string]interface{}{}})
	store.Store(&storage.LazyResource{ID: "res3", ResourceType: "mcp-proxy", Resource: map[string]interface{}{}})

	// Remove by type
	err := manager.RemoveResourcesByType("llm-provider")
	if err != nil {
		t.Fatalf("RemoveResourcesByType failed: %v", err)
	}

	// Verify only mcp-proxy remains
	if store.Count() != 1 {
		t.Errorf("Store count = %d, want 1", store.Count())
	}
}

func TestLazyResourceStateManager_StoreResource(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewLazyResourceStore(logger)
	snapshotManager := NewLazyResourceSnapshotManager(store, logger)
	manager := NewLazyResourceStateManager(store, snapshotManager, logger)

	resource := &storage.LazyResource{
		ID:           "res1",
		ResourceType: "llm-provider",
		Resource:     map[string]interface{}{"name": "test"},
	}

	err := manager.StoreResource(resource, "correlation-123")
	if err != nil {
		t.Fatalf("StoreResource failed: %v", err)
	}

	if count := manager.GetResourceCount(); count != 1 {
		t.Errorf("GetResourceCount() = %d, want 1", count)
	}
}

func TestLazyResourceStateManager_RemoveResourceByIDAndType(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewLazyResourceStore(logger)
	snapshotManager := NewLazyResourceSnapshotManager(store, logger)
	manager := NewLazyResourceStateManager(store, snapshotManager, logger)

	// Store first
	resource := &storage.LazyResource{
		ID:           "res1",
		ResourceType: "llm-provider",
		Resource:     map[string]interface{}{"name": "test"},
	}
	store.Store(resource)

	err := manager.RemoveResourceByIDAndType("res1", "llm-provider", "correlation-123")
	if err != nil {
		t.Fatalf("RemoveResourceByIDAndType failed: %v", err)
	}

	if count := manager.GetResourceCount(); count != 0 {
		t.Errorf("GetResourceCount() = %d, want 0", count)
	}
}

func TestLazyResourceStateManager_RemoveResourcesByType(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewLazyResourceStore(logger)
	snapshotManager := NewLazyResourceSnapshotManager(store, logger)
	manager := NewLazyResourceStateManager(store, snapshotManager, logger)

	// Store resources
	store.Store(&storage.LazyResource{ID: "res1", ResourceType: "llm-provider", Resource: map[string]interface{}{}})
	store.Store(&storage.LazyResource{ID: "res2", ResourceType: "llm-provider", Resource: map[string]interface{}{}})

	err := manager.RemoveResourcesByType("llm-provider", "correlation-123")
	if err != nil {
		t.Fatalf("RemoveResourcesByType failed: %v", err)
	}

	if count := manager.GetResourceCount(); count != 0 {
		t.Errorf("GetResourceCount() = %d, want 0", count)
	}
}

func TestLazyResourceStateManager_GetResourceByIDAndType(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewLazyResourceStore(logger)
	snapshotManager := NewLazyResourceSnapshotManager(store, logger)
	manager := NewLazyResourceStateManager(store, snapshotManager, logger)

	// Store a resource
	resource := &storage.LazyResource{
		ID:           "res1",
		ResourceType: "llm-provider",
		Resource:     map[string]interface{}{"name": "test"},
	}
	store.Store(resource)

	// Get existing resource
	found, exists := manager.GetResourceByIDAndType("res1", "llm-provider")
	if !exists {
		t.Error("Expected resource to exist")
	}
	if found == nil {
		t.Error("Expected non-nil resource")
	}

	// Get non-existent resource
	_, exists = manager.GetResourceByIDAndType("non-existent", "llm-provider")
	if exists {
		t.Error("Expected resource to not exist")
	}
}

func TestLazyResourceStateManager_GetResourcesByType(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewLazyResourceStore(logger)
	snapshotManager := NewLazyResourceSnapshotManager(store, logger)
	manager := NewLazyResourceStateManager(store, snapshotManager, logger)

	// Store resources
	store.Store(&storage.LazyResource{ID: "res1", ResourceType: "llm-provider", Resource: map[string]interface{}{}})
	store.Store(&storage.LazyResource{ID: "res2", ResourceType: "llm-provider", Resource: map[string]interface{}{}})
	store.Store(&storage.LazyResource{ID: "res3", ResourceType: "mcp-proxy", Resource: map[string]interface{}{}})

	resources := manager.GetResourcesByType("llm-provider")
	if len(resources) != 2 {
		t.Errorf("GetResourcesByType() returned %d resources, want 2", len(resources))
	}
}

func TestLazyResourceStateManager_GetAllResources(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewLazyResourceStore(logger)
	snapshotManager := NewLazyResourceSnapshotManager(store, logger)
	manager := NewLazyResourceStateManager(store, snapshotManager, logger)

	// Store resources
	store.Store(&storage.LazyResource{ID: "res1", ResourceType: "llm-provider", Resource: map[string]interface{}{}})
	store.Store(&storage.LazyResource{ID: "res2", ResourceType: "mcp-proxy", Resource: map[string]interface{}{}})

	resources := manager.GetAllResources()
	if len(resources) != 2 {
		t.Errorf("GetAllResources() returned %d resources, want 2", len(resources))
	}
}

func TestLazyResourceStateManager_RefreshSnapshot(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewLazyResourceStore(logger)
	snapshotManager := NewLazyResourceSnapshotManager(store, logger)
	manager := NewLazyResourceStateManager(store, snapshotManager, logger)

	err := manager.RefreshSnapshot()
	if err != nil {
		t.Fatalf("RefreshSnapshot failed: %v", err)
	}
}

func TestLazyResourceTypeURL(t *testing.T) {
	expected := "api-platform.wso2.org/v1.LazyResources"
	if LazyResourceTypeURL != expected {
		t.Errorf("LazyResourceTypeURL = %q, want %q", LazyResourceTypeURL, expected)
	}
}
