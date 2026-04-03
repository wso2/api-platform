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
)

func TestNewSnapshotManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	manager := NewSnapshotManager(logger)
	if manager == nil {
		t.Fatal("NewSnapshotManager returned nil")
	}

	if manager.GetPolicyCache() == nil {
		t.Error("GetCache() returned nil")
	}

	if manager.GetRouteCache() == nil {
		t.Error("GetRouteCache() returned nil")
	}
}

func TestNewPolicyManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	snapshotManager := NewSnapshotManager(logger)

	manager := NewPolicyManager(snapshotManager, logger)
	if manager == nil {
		t.Fatal("NewPolicyManager returned nil")
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
}

func TestPolicyManager_UpsertAndDeleteAPIConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	snapshotManager := NewSnapshotManager(logger)
	manager := NewPolicyManager(snapshotManager, logger)

	runtimeStore := storage.NewRuntimeConfigStore()
	snapshotManager.SetRuntimeStore(runtimeStore)
	manager.SetRuntimeStore(runtimeStore)

	// UpsertAPIConfig without transformers set — should return error
	cfg := &models.StoredConfig{
		UUID:   "api-1",
		Kind:   "RestApi",
		Handle: "test-api",
	}
	err := manager.UpsertAPIConfig(cfg)
	if err == nil {
		t.Error("UpsertAPIConfig should fail when transformers are not set")
	}

	// DeleteAPIConfig — should be a no-op (not found is silently ignored)
	err = manager.DeleteAPIConfig("RestApi", "test-api")
	if err != nil {
		t.Errorf("DeleteAPIConfig should not fail for non-existent key, got: %v", err)
	}
}

func TestPolicyManager_GetResourceVersion(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	snapshotManager := NewSnapshotManager(logger)
	manager := NewPolicyManager(snapshotManager, logger)

	runtimeStore := storage.NewRuntimeConfigStore()
	manager.SetRuntimeStore(runtimeStore)

	v0 := manager.GetResourceVersion()
	runtimeStore.IncrementResourceVersion()
	runtimeStore.IncrementResourceVersion()
	v2 := manager.GetResourceVersion()

	if v2 <= v0 {
		t.Errorf("GetResourceVersion should increase after increments: v0=%d, v2=%d", v0, v2)
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
