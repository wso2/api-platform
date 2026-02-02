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

package policyxds

import (
	"log/slog"
	"os"
	"testing"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	policyenginev1 "github.com/wso2/api-platform/sdk/gateway/policyengine/v1"
)

func TestNewSnapshotManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewPolicyStore()

	manager := NewSnapshotManager(store, logger)
	if manager == nil {
		t.Fatal("NewSnapshotManager returned nil")
	}

	if manager.GetCache() == nil {
		t.Error("GetCache() returned nil")
	}
}

func TestNewPolicyManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewPolicyStore()
	snapshotManager := NewSnapshotManager(store, logger)

	manager := NewPolicyManager(store, snapshotManager, logger)
	if manager == nil {
		t.Fatal("NewPolicyManager returned nil")
	}

	// Test ListPolicies when empty
	policies := manager.ListPolicies()
	if len(policies) != 0 {
		t.Errorf("ListPolicies() returned %d policies, want 0", len(policies))
	}
}

func TestNewTranslator(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	translator := NewTranslator(logger)

	if translator == nil {
		t.Fatal("NewTranslator returned nil")
	}
}

func TestPolicyChainTypeURL(t *testing.T) {
	expected := "api-platform.wso2.org/v1.PolicyChainConfig"
	if PolicyChainTypeURL != expected {
		t.Errorf("PolicyChainTypeURL = %q, want %q", PolicyChainTypeURL, expected)
	}
}

func TestTranslator_TranslatePolicies(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	translator := NewTranslator(logger)

	// Test with empty policies
	t.Run("empty policies", func(t *testing.T) {
		resources, err := translator.TranslatePolicies([]*models.StoredPolicyConfig{})
		if err != nil {
			t.Fatalf("TranslatePolicies failed: %v", err)
		}
		if resources == nil {
			t.Error("TranslatePolicies returned nil resources")
		}
		if _, ok := resources[PolicyChainTypeURL]; !ok {
			t.Error("Expected PolicyChainTypeURL in resources")
		}
	})

	// Test with policies
	t.Run("with policies", func(t *testing.T) {
		policies := []*models.StoredPolicyConfig{
			{
				ID: "policy-1",
				Configuration: policyenginev1.Configuration{
					Routes: []policyenginev1.PolicyChain{
						{
							RouteKey: "GET:/api/v1/users",
							Policies: []policyenginev1.PolicyInstance{
								{Name: "rate-limit", Version: "v1", Enabled: true},
							},
						},
					},
					Metadata: policyenginev1.Metadata{
						APIName: "TestAPI",
						Version: "v1",
						Context: "/api",
					},
				},
				Version: 1,
			},
		}

		resources, err := translator.TranslatePolicies(policies)
		if err != nil {
			t.Fatalf("TranslatePolicies failed: %v", err)
		}
		if resources == nil {
			t.Error("TranslatePolicies returned nil resources")
		}
		if len(resources[PolicyChainTypeURL]) != 1 {
			t.Errorf("Expected 1 resource, got %d", len(resources[PolicyChainTypeURL]))
		}
	})
}

func TestSnapshotManager_UpdateSnapshot(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewPolicyStore()
	manager := NewSnapshotManager(store, logger)

	// Test update with empty store
	t.Run("empty store", func(t *testing.T) {
		err := manager.UpdateSnapshot(nil)
		if err != nil {
			t.Errorf("UpdateSnapshot failed: %v", err)
		}
	})

	// Add a policy and update
	t.Run("with policy", func(t *testing.T) {
		policy := &models.StoredPolicyConfig{
			ID: "policy-1",
			Configuration: policyenginev1.Configuration{
				Routes: []policyenginev1.PolicyChain{
					{
						RouteKey: "GET:/api/v1/test",
						Policies: []policyenginev1.PolicyInstance{
							{Name: "cors", Version: "v1", Enabled: true},
						},
					},
				},
				Metadata: policyenginev1.Metadata{
					APIName: "TestAPI",
					Version: "v1",
					Context: "/api",
				},
			},
			Version: 1,
		}
		store.Set(policy)

		err := manager.UpdateSnapshot(nil)
		if err != nil {
			t.Errorf("UpdateSnapshot failed: %v", err)
		}
	})
}

func TestPolicyManager_AddPolicy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewPolicyStore()
	snapshotManager := NewSnapshotManager(store, logger)
	manager := NewPolicyManager(store, snapshotManager, logger)

	policy := &models.StoredPolicyConfig{
		ID: "policy-1",
		Configuration: policyenginev1.Configuration{
			Routes: []policyenginev1.PolicyChain{
				{
					RouteKey: "GET:/api/v1/users",
					Policies: []policyenginev1.PolicyInstance{
						{Name: "rate-limit", Version: "v1", Enabled: true},
					},
				},
			},
			Metadata: policyenginev1.Metadata{
				APIName: "TestAPI",
				Version: "v1",
				Context: "/api",
			},
		},
		Version: 1,
	}

	err := manager.AddPolicy(policy)
	if err != nil {
		t.Fatalf("AddPolicy failed: %v", err)
	}

	// Verify policy is stored
	policies := manager.ListPolicies()
	if len(policies) != 1 {
		t.Errorf("ListPolicies() returned %d policies, want 1", len(policies))
	}
}

func TestPolicyManager_RemovePolicy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewPolicyStore()
	snapshotManager := NewSnapshotManager(store, logger)
	manager := NewPolicyManager(store, snapshotManager, logger)

	// Add first
	policy := &models.StoredPolicyConfig{
		ID: "policy-1",
		Configuration: policyenginev1.Configuration{
			Routes: []policyenginev1.PolicyChain{},
			Metadata: policyenginev1.Metadata{
				APIName: "TestAPI",
				Version: "v1",
				Context: "/api",
			},
		},
		Version: 1,
	}
	store.Set(policy)

	// Remove
	err := manager.RemovePolicy("policy-1")
	if err != nil {
		t.Fatalf("RemovePolicy failed: %v", err)
	}

	// Verify policy is removed
	policies := manager.ListPolicies()
	if len(policies) != 0 {
		t.Errorf("ListPolicies() returned %d policies, want 0", len(policies))
	}

	// Remove non-existent (should error)
	err = manager.RemovePolicy("non-existent")
	if err == nil {
		t.Error("RemovePolicy should fail for non-existent policy")
	}
}

func TestPolicyManager_GetPolicy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewPolicyStore()
	snapshotManager := NewSnapshotManager(store, logger)
	manager := NewPolicyManager(store, snapshotManager, logger)

	// Add policy
	policy := &models.StoredPolicyConfig{
		ID: "policy-1",
		Configuration: policyenginev1.Configuration{
			Routes: []policyenginev1.PolicyChain{},
			Metadata: policyenginev1.Metadata{
				APIName: "TestAPI",
				Version: "v1",
				Context: "/api",
			},
		},
		Version: 1,
	}
	store.Set(policy)

	// Get existing policy
	found, err := manager.GetPolicy("policy-1")
	if err != nil {
		t.Fatalf("GetPolicy failed: %v", err)
	}
	if found == nil {
		t.Error("GetPolicy returned nil")
	}
	if found.ID != "policy-1" {
		t.Errorf("GetPolicy returned wrong policy ID: %s", found.ID)
	}

	// Get non-existent policy
	_, err = manager.GetPolicy("non-existent")
	if err == nil {
		t.Error("GetPolicy should fail for non-existent policy")
	}
}

func TestParsePolicyJSON(t *testing.T) {
	// Valid JSON
	t.Run("valid json", func(t *testing.T) {
		jsonStr := `{
			"routes": [
				{
					"routeKey": "GET:/api/test",
					"policies": [
						{"name": "cors", "version": "v1", "enabled": true}
					]
				}
			],
			"metadata": {
				"apiName": "TestAPI",
				"version": "v1",
				"context": "/api"
			}
		}`

		config, err := ParsePolicyJSON(jsonStr)
		if err != nil {
			t.Fatalf("ParsePolicyJSON failed: %v", err)
		}
		if config == nil {
			t.Error("ParsePolicyJSON returned nil config")
		}
		if len(config.Routes) != 1 {
			t.Errorf("Expected 1 route, got %d", len(config.Routes))
		}
	})

	// Invalid JSON
	t.Run("invalid json", func(t *testing.T) {
		_, err := ParsePolicyJSON("not valid json")
		if err == nil {
			t.Error("ParsePolicyJSON should fail for invalid JSON")
		}
	})
}

func TestCreateStoredPolicy(t *testing.T) {
	config := policyenginev1.Configuration{
		Routes: []policyenginev1.PolicyChain{
			{
				RouteKey: "GET:/test",
				Policies: []policyenginev1.PolicyInstance{},
			},
		},
		Metadata: policyenginev1.Metadata{
			APIName:         "TestAPI",
			Version:         "v1",
			Context:         "/api",
			ResourceVersion: 5,
		},
	}

	stored := CreateStoredPolicy("policy-123", config)
	if stored == nil {
		t.Fatal("CreateStoredPolicy returned nil")
	}
	if stored.ID != "policy-123" {
		t.Errorf("ID = %q, want %q", stored.ID, "policy-123")
	}
	if stored.Version != 5 {
		t.Errorf("Version = %d, want 5", stored.Version)
	}
}

func TestSlogAdapter(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	adapter := slogAdapter{logger}

	// Test all log methods - they should not panic
	t.Run("Debugf", func(t *testing.T) {
		adapter.Debugf("debug message %s", "test")
	})

	t.Run("Infof", func(t *testing.T) {
		adapter.Infof("info message %d", 42)
	})

	t.Run("Warnf", func(t *testing.T) {
		adapter.Warnf("warn message %v", "warning")
	})

	t.Run("Errorf", func(t *testing.T) {
		adapter.Errorf("error message %s %d", "error", 500)
	})
}
