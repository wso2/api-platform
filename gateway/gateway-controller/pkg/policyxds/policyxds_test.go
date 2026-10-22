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

// Gateway-owned runtime configs (the JWKS route) belong in every snapshot,
// including one with nothing deployed, without ever entering the runtime store.
func TestSnapshotManager_IncludesGatewayRuntimeConfigs(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sm := NewSnapshotManager(logger)
	store := storage.NewRuntimeConfigStore()
	sm.SetRuntimeStore(store)

	const key = "GET|/.well-known/jwks|*"
	sm.SetGatewayRuntimeConfigs(&models.RuntimeDeployConfig{
		Metadata:            models.Metadata{UUID: "gateway-jwks", Kind: "GatewayJWKS", Handle: "gateway-jwks"},
		PolicyChainResolver: models.RouteKeyResolverName,
		Routes: map[string]*models.Route{
			key: {Method: "GET", Path: "/.well-known/jwks", OperationPath: "/.well-known/jwks", Vhost: "*"},
		},
		PolicyChains: map[string]*models.PolicyChain{
			key: {Policies: []models.Policy{{Name: "wso2_apip_sys_jwks", Version: "v1"}}},
		},
	})

	if err := sm.UpdateSnapshot(t.Context()); err != nil {
		t.Fatalf("UpdateSnapshot: %v", err)
	}
	if _, ok := sm.policyCache.GetResources()[key]; !ok {
		t.Fatal("the gateway JWKS chain is missing from the policy snapshot")
	}
	if _, ok := sm.routeCache.GetResources()[key]; !ok {
		t.Fatal("the gateway JWKS route config is missing from the route snapshot")
	}
	if got := len(store.GetAll()); got != 0 {
		t.Fatalf("gateway configs leaked into the runtime store: %d entries", got)
	}
}

// gatewayTestRDC is a single-route runtime config keyed key, shaped like the
// gateway JWKS config the controller sets at startup.
func gatewayTestRDC(handle, key, path string) *models.RuntimeDeployConfig {
	return &models.RuntimeDeployConfig{
		Metadata:            models.Metadata{UUID: handle, Kind: "GatewayJWKS", Handle: handle},
		PolicyChainResolver: models.RouteKeyResolverName,
		Routes: map[string]*models.Route{
			key: {Method: "GET", Path: path, OperationPath: path, Vhost: "*"},
		},
		PolicyChains: map[string]*models.PolicyChain{
			key: {Policies: []models.Policy{{Name: "wso2_apip_sys_jwks", Version: "v1"}}},
		},
	}
}

func newGatewayConfigsTestManager(t *testing.T) (*SnapshotManager, *storage.RuntimeConfigStore) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sm := NewSnapshotManager(logger)
	store := storage.NewRuntimeConfigStore()
	sm.SetRuntimeStore(store)
	return sm, store
}

func snapshotHasKey(t *testing.T, sm *SnapshotManager, key string) bool {
	t.Helper()
	_, inPolicy := sm.policyCache.GetResources()[key]
	_, inRoute := sm.routeCache.GetResources()[key]
	if inPolicy != inRoute {
		t.Fatalf("key %q present in policy snapshot = %v but in route snapshot = %v", key, inPolicy, inRoute)
	}
	return inPolicy
}

// Deployed configs and gateway-owned configs are merged into one snapshot.
func TestSnapshotManager_GatewayRuntimeConfigsAlongsideDeployed(t *testing.T) {
	sm, store := newGatewayConfigsTestManager(t)

	const deployedKey = "GET|/orders|*"
	const gatewayKey = "GET|/.well-known/jwks|*"
	store.Set("RestApi:orders", gatewayTestRDC("orders", deployedKey, "/orders"))
	sm.SetGatewayRuntimeConfigs(gatewayTestRDC("gateway-jwks", gatewayKey, "/.well-known/jwks"))

	if err := sm.UpdateSnapshot(t.Context()); err != nil {
		t.Fatalf("UpdateSnapshot: %v", err)
	}
	if !snapshotHasKey(t, sm, deployedKey) {
		t.Fatal("the deployed config is missing from the snapshot")
	}
	if !snapshotHasKey(t, sm, gatewayKey) {
		t.Fatal("the gateway config is missing from the snapshot")
	}
	if got := len(store.GetAll()); got != 1 {
		t.Fatalf("runtime store must hold only the deployed config, got %d entries", got)
	}
}

// SetGatewayRuntimeConfigs with no arguments clears what an earlier call set.
func TestSnapshotManager_SetGatewayRuntimeConfigsEmptyClears(t *testing.T) {
	sm, _ := newGatewayConfigsTestManager(t)

	const gatewayKey = "GET|/.well-known/jwks|*"
	sm.SetGatewayRuntimeConfigs(gatewayTestRDC("gateway-jwks", gatewayKey, "/.well-known/jwks"))
	if err := sm.UpdateSnapshot(t.Context()); err != nil {
		t.Fatalf("UpdateSnapshot: %v", err)
	}
	if !snapshotHasKey(t, sm, gatewayKey) {
		t.Fatal("the gateway config is missing from the first snapshot")
	}

	sm.SetGatewayRuntimeConfigs()
	if err := sm.UpdateSnapshot(t.Context()); err != nil {
		t.Fatalf("UpdateSnapshot: %v", err)
	}
	if snapshotHasKey(t, sm, gatewayKey) {
		t.Fatal("the gateway config must be gone after clearing")
	}
}

// A second SetGatewayRuntimeConfigs replaces the first set rather than adding to it.
func TestSnapshotManager_SetGatewayRuntimeConfigsReplaces(t *testing.T) {
	sm, _ := newGatewayConfigsTestManager(t)

	const firstKey = "GET|/.well-known/jwks|*"
	const secondKey = "GET|/.well-known/other|*"
	sm.SetGatewayRuntimeConfigs(gatewayTestRDC("first", firstKey, "/.well-known/jwks"))
	sm.SetGatewayRuntimeConfigs(gatewayTestRDC("second", secondKey, "/.well-known/other"))

	if err := sm.UpdateSnapshot(t.Context()); err != nil {
		t.Fatalf("UpdateSnapshot: %v", err)
	}
	if snapshotHasKey(t, sm, firstKey) {
		t.Fatal("the first gateway config must be replaced, not kept")
	}
	if !snapshotHasKey(t, sm, secondKey) {
		t.Fatal("the second gateway config is missing from the snapshot")
	}
}
