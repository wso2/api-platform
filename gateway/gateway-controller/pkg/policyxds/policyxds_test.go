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

	if manager.GetRouteCache() == nil {
		t.Error("GetRouteCache() returned nil")
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

func TestRouteConfigTypeURL(t *testing.T) {
	expected := "api-platform.wso2.org/v1.RouteConfig"
	if RouteConfigTypeURL != expected {
		t.Errorf("RouteConfigTypeURL = %q, want %q", RouteConfigTypeURL, expected)
	}
}

func TestTranslator_TranslateRuntimeConfigs(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	translator := NewTranslator(logger)

	// Test with empty configs
	t.Run("empty configs", func(t *testing.T) {
		resources, err := translator.TranslateRuntimeConfigs([]*models.RuntimeDeployConfig{})
		if err != nil {
			t.Fatalf("TranslateRuntimeConfigs failed: %v", err)
		}
		if resources == nil {
			t.Error("TranslateRuntimeConfigs returned nil resources")
		}
		if _, ok := resources[PolicyChainTypeURL]; !ok {
			t.Error("Expected PolicyChainTypeURL in resources")
		}
		if _, ok := resources[RouteConfigTypeURL]; !ok {
			t.Error("Expected RouteConfigTypeURL in resources")
		}
	})

	// Test with a RuntimeDeployConfig
	t.Run("with runtime config", func(t *testing.T) {
		rdcs := []*models.RuntimeDeployConfig{
			{
				Metadata: models.Metadata{
					Kind:        "RestApi",
					Handle:      "test-handle",
					Name:        "TestAPI",
					Version:     "v1",
					DisplayName: "TestAPI",
				},
				Context:             "/api",
				PolicyChainResolver: "route-key",
				Routes: map[string]*models.Route{
					"GET|/api/v1/users|localhost": {
						Method:        "GET",
						Path:          "/api/v1/users",
						OperationPath: "/users",
						Vhost:         "localhost",
						Upstream: models.RouteUpstream{
							ClusterKey: "upstream_main_localhost_8080",
						},
					},
				},
				PolicyChains: map[string]*models.PolicyChain{
					"GET|/api/v1/users|localhost": {
						Policies: []models.Policy{
							{Name: "rate-limit", Version: "v1"},
						},
					},
				},
				UpstreamClusters: map[string]*models.UpstreamCluster{
					"upstream_main_localhost_8080": {
						BasePath:  "/",
						Endpoints: []models.Endpoint{{Host: "localhost", Port: 8080}},
					},
				},
			},
		}

		resources, err := translator.TranslateRuntimeConfigs(rdcs)
		if err != nil {
			t.Fatalf("TranslateRuntimeConfigs failed: %v", err)
		}
		if len(resources[PolicyChainTypeURL]) != 1 {
			t.Errorf("Expected 1 policy resource, got %d", len(resources[PolicyChainTypeURL]))
		}
		if len(resources[RouteConfigTypeURL]) != 1 {
			t.Errorf("Expected 1 route config resource, got %d", len(resources[RouteConfigTypeURL]))
		}
	})

	// Test that empty policy chains are still sent as xDS resources (not skipped)
	t.Run("empty policy chain is sent", func(t *testing.T) {
		rdcs := []*models.RuntimeDeployConfig{
			{
				Metadata: models.Metadata{
					Kind:        "RestApi",
					Handle:      "test-handle",
					Name:        "TestAPI",
					Version:     "v1",
					DisplayName: "TestAPI",
				},
				Context:             "/api",
				PolicyChainResolver: "route-key",
				Routes: map[string]*models.Route{
					"GET|/api/v1/pets|localhost": {
						Method:        "GET",
						Path:          "/api/v1/pets",
						OperationPath: "/pets",
						Vhost:         "localhost",
						Upstream: models.RouteUpstream{
							ClusterKey: "upstream_main_localhost_8080",
						},
					},
				},
				PolicyChains: map[string]*models.PolicyChain{
					"GET|/api/v1/pets|localhost": {
						Policies: []models.Policy{}, // empty policy chain
					},
				},
				UpstreamClusters: map[string]*models.UpstreamCluster{
					"upstream_main_localhost_8080": {
						BasePath:  "/",
						Endpoints: []models.Endpoint{{Host: "localhost", Port: 8080}},
					},
				},
			},
		}

		resources, err := translator.TranslateRuntimeConfigs(rdcs)
		if err != nil {
			t.Fatalf("TranslateRuntimeConfigs failed: %v", err)
		}
		if len(resources[PolicyChainTypeURL]) != 1 {
			t.Errorf("Expected 1 policy resource for empty chain, got %d", len(resources[PolicyChainTypeURL]))
		}
	})
}

func TestSnapshotManager_UpdateSnapshotLegacy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewPolicyStore()
	manager := NewSnapshotManager(store, logger)

	// Test update with empty store
	t.Run("empty store", func(t *testing.T) {
		err := manager.UpdateSnapshotLegacy(nil, store)
		if err != nil {
			t.Errorf("UpdateSnapshotLegacy failed: %v", err)
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

		err := manager.UpdateSnapshotLegacy(nil, store)
		if err != nil {
			t.Errorf("UpdateSnapshotLegacy failed: %v", err)
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
