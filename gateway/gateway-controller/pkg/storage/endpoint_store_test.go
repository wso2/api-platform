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
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"gotest.tools/v3/assert"
)

var endpointCounter int

// createTestEndpoint builds an Endpoint with a unique UUID and the given name.
func createTestEndpoint(name string) *models.StoredEndpoint {
	endpointCounter++
	basePath := "/v1"
	labels := map[string]string{"team": "orders"}
	endpoint := &models.StoredEndpoint{
		UUID: fmt.Sprintf("test-endpoint-%d", endpointCounter),
		Configuration: api.EndpointConfiguration{
			ApiVersion: api.EndpointConfigurationApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.EndpointConfigurationKindEndpoint,
			Metadata:   api.Metadata{Name: name, Labels: &labels},
			Spec:       api.EndpointConfigurationData{BasePath: &basePath},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	endpoint.Configuration.Spec.Upstreams = append(endpoint.Configuration.Spec.Upstreams, struct {
		Url    string `json:"url" yaml:"url"`
		Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
	}{Url: "http://orders-" + name + ":9080"})
	return endpoint
}

func TestEndpointStore_SaveAndGet(t *testing.T) {
	store := setupTestStorage(t)
	endpoint := createTestEndpoint("tenant-a")
	assert.NilError(t, store.SaveEndpoint(endpoint))

	byID, err := store.GetEndpoint(endpoint.UUID)
	assert.NilError(t, err)
	assert.Equal(t, byID.GetHandle(), "tenant-a")
	assert.Equal(t, byID.BasePath(), "/v1")
	assert.DeepEqual(t, byID.Labels(), map[string]string{"team": "orders"})
	assert.Equal(t, byID.Configuration.Spec.Upstreams[0].Url, "http://orders-tenant-a:9080")

	byHandle, err := store.GetEndpointByHandle("tenant-a")
	assert.NilError(t, err)
	assert.Equal(t, byHandle.UUID, endpoint.UUID)

	_, err = store.GetEndpoint("missing")
	assert.Assert(t, errors.Is(err, ErrNotFound))
	_, err = store.GetEndpointByHandle("missing")
	assert.Assert(t, errors.Is(err, ErrNotFound))
}

// The artifacts row is what carries origin and control-plane sync state, and
// what keeps a control-plane reconcile from sweeping a local Endpoint away.
func TestEndpointStore_SaveTracksArtifact(t *testing.T) {
	store := setupTestStorage(t)
	endpoint := createTestEndpoint("tenant-a")
	assert.NilError(t, store.SaveEndpoint(endpoint))

	cfg, err := store.GetConfig(endpoint.UUID)
	assert.NilError(t, err)
	assert.Equal(t, cfg.Kind, string(models.KindEndpoint))
	assert.Equal(t, cfg.Handle, "tenant-a")
	assert.Equal(t, cfg.Origin, models.OriginGatewayAPI)
	assert.Equal(t, cfg.CPSyncStatus, models.CPSyncStatusPending)
	_, ok := cfg.Configuration.(api.EndpointConfiguration)
	assert.Assert(t, ok, "the Endpoint configuration should be reconstructed, got %T", cfg.Configuration)

	controlPlane, err := store.GetAllConfigsByOrigin(models.OriginControlPlane)
	assert.NilError(t, err)
	assert.Assert(t, !pendingContains(controlPlane, endpoint.UUID),
		"a gateway-created Endpoint must not look control-plane owned")
}

// Every reader of GetAllConfigs assumes each row is an API with routes.
func TestEndpointStore_EndpointsAreNotAPIConfigs(t *testing.T) {
	store := setupTestStorage(t)
	assert.NilError(t, store.SaveEndpoint(createTestEndpoint("tenant-a")))
	apiConfig := createTestStoredConfig()
	assert.NilError(t, store.SaveConfig(apiConfig))

	configs, err := store.GetAllConfigs()
	assert.NilError(t, err)
	assert.Equal(t, len(configs), 1)
	assert.Equal(t, configs[0].UUID, apiConfig.UUID)

	for _, table := range builtinResourceTables {
		assert.Assert(t, table != endpointsResourceTable, "endpoints must stay out of builtinResourceTables")
	}
}

func TestEndpointStore_NameIsUnique(t *testing.T) {
	store := setupTestStorage(t)
	assert.NilError(t, store.SaveEndpoint(createTestEndpoint("tenant-a")))

	err := store.SaveEndpoint(createTestEndpoint("tenant-a"))
	assert.Assert(t, errors.Is(err, ErrConflict), "got %v", err)

	// The failed insert left nothing behind.
	endpoints, err := store.GetAllEndpoints()
	assert.NilError(t, err)
	assert.Equal(t, len(endpoints), 1)
	var artifacts int
	assert.NilError(t, store.db.QueryRow(`SELECT COUNT(*) FROM artifacts WHERE kind = 'Endpoint'`).Scan(&artifacts))
	assert.Equal(t, artifacts, 1)
}

// An API and an Endpoint are different kinds, so they may share a handle.
func TestEndpointStore_NameDoesNotCollideWithAnAPIHandle(t *testing.T) {
	store := setupTestStorage(t)
	apiConfig := createTestStoredConfig()
	assert.NilError(t, store.SaveConfig(apiConfig))

	assert.NilError(t, store.SaveEndpoint(createTestEndpoint(apiConfig.Handle)))
}

func TestEndpointStore_Update(t *testing.T) {
	store := setupTestStorage(t)
	endpoint := createTestEndpoint("tenant-a")
	assert.NilError(t, store.SaveEndpoint(endpoint))
	assert.NilError(t, store.UpdateCPSyncStatus(endpoint.UUID, "cp-id", models.CPSyncStatusSuccess, ""))

	newBasePath := "/v2"
	endpoint.Configuration.Spec.BasePath = &newBasePath
	endpoint.Configuration.Spec.Upstreams[0].Url = "http://orders-moved:9080"
	assert.NilError(t, store.UpdateEndpoint(endpoint))

	got, err := store.GetEndpointByHandle("tenant-a")
	assert.NilError(t, err)
	assert.Equal(t, got.BasePath(), "/v2")
	assert.Equal(t, got.Configuration.Spec.Upstreams[0].Url, "http://orders-moved:9080")
	assert.Equal(t, got.UUID, endpoint.UUID)

	// A change re-arms control-plane sync and keeps the control-plane id.
	cfg, err := store.GetConfig(endpoint.UUID)
	assert.NilError(t, err)
	assert.Equal(t, cfg.CPSyncStatus, models.CPSyncStatusPending)
	assert.Equal(t, cfg.CPArtifactID, "cp-id")
}

func TestEndpointStore_UpdateRequiresTheSameNameAndAnExistingRow(t *testing.T) {
	store := setupTestStorage(t)
	endpoint := createTestEndpoint("tenant-a")
	assert.NilError(t, store.SaveEndpoint(endpoint))

	// The name is the Endpoint's identity; an update never renames.
	renamed := *endpoint
	renamed.Configuration.Metadata.Name = "tenant-b"
	assert.Assert(t, errors.Is(store.UpdateEndpoint(&renamed), ErrNotFound))
	_, err := store.GetEndpointByHandle("tenant-a")
	assert.NilError(t, err)

	assert.Assert(t, errors.Is(store.UpdateEndpoint(createTestEndpoint("tenant-z")), ErrNotFound))
}

func TestEndpointStore_Delete(t *testing.T) {
	store := setupTestStorage(t)
	endpoint := createTestEndpoint("tenant-a")
	assert.NilError(t, store.SaveEndpoint(endpoint))

	assert.NilError(t, store.DeleteEndpoint(endpoint.UUID))

	_, err := store.GetEndpoint(endpoint.UUID)
	assert.Assert(t, errors.Is(err, ErrNotFound))
	var artifacts int
	assert.NilError(t, store.db.QueryRow(`SELECT COUNT(*) FROM artifacts WHERE uuid = ?`, endpoint.UUID).Scan(&artifacts))
	assert.Equal(t, artifacts, 0, "the artifacts tracking row must go with the Endpoint")

	assert.Assert(t, errors.Is(store.DeleteEndpoint(endpoint.UUID), ErrNotFound))

	// The name is free again.
	assert.NilError(t, store.SaveEndpoint(createTestEndpoint("tenant-a")))
}

func TestEndpointStore_GetAllIsOrderedByName(t *testing.T) {
	store := setupTestStorage(t)

	empty, err := store.GetAllEndpoints()
	assert.NilError(t, err)
	assert.Equal(t, len(empty), 0)

	for _, name := range []string{"tenant-c", "tenant-a", "tenant-b"} {
		assert.NilError(t, store.SaveEndpoint(createTestEndpoint(name)))
	}
	endpoints, err := store.GetAllEndpoints()
	assert.NilError(t, err)
	var names []string
	for _, endpoint := range endpoints {
		names = append(names, endpoint.GetHandle())
	}
	assert.DeepEqual(t, names, []string{"tenant-a", "tenant-b", "tenant-c"})
}

func TestEndpointStore_IsScopedToTheGateway(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "shared.db")
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	metrics.Init()

	first, err := NewStorage(BackendConfig{Type: "sqlite", SQLitePath: dbPath, GatewayID: "gateway-one"}, logger)
	assert.NilError(t, err)
	assert.NilError(t, first.SaveEndpoint(createTestEndpoint("tenant-a")))
	assert.NilError(t, first.Close())

	second, err := NewStorage(BackendConfig{Type: "sqlite", SQLitePath: dbPath, GatewayID: "gateway-two"}, logger)
	assert.NilError(t, err)
	defer second.Close()

	endpoints, err := second.GetAllEndpoints()
	assert.NilError(t, err)
	assert.Equal(t, len(endpoints), 0)
	_, err = second.GetEndpointByHandle("tenant-a")
	assert.Assert(t, errors.Is(err, ErrNotFound))
	// The same name is free on another gateway.
	assert.NilError(t, second.SaveEndpoint(createTestEndpoint("tenant-a")))
}

// A database created before Endpoints existed is at the current schema version
// and has no endpoints table. The guarded CREATE in the schema adds it on the
// next start; nothing else about the database changes.
func TestEndpointStore_ExistingDatabaseGainsTheTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "existing.db")
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	metrics.Init()

	store, err := NewStorage(BackendConfig{Type: "sqlite", SQLitePath: dbPath, GatewayID: "platform-gateway-id"}, logger)
	assert.NilError(t, err)
	apiConfig := createTestStoredConfig()
	assert.NilError(t, store.SaveConfig(apiConfig))
	assert.NilError(t, store.Close())

	rawDB, err := sql.Open("sqlite3", dbPath)
	assert.NilError(t, err)
	_, err = rawDB.Exec(`DROP TABLE endpoints`)
	assert.NilError(t, err)
	assert.NilError(t, rawDB.Close())

	upgraded, err := NewStorage(BackendConfig{Type: "sqlite", SQLitePath: dbPath, GatewayID: "platform-gateway-id"}, logger)
	assert.NilError(t, err)
	defer upgraded.Close()

	assert.NilError(t, upgraded.SaveEndpoint(createTestEndpoint("tenant-a")))
	configs, err := upgraded.GetAllConfigs()
	assert.NilError(t, err)
	assert.Equal(t, len(configs), 1)
	assert.Equal(t, configs[0].UUID, apiConfig.UUID)
}

func TestLoadEndpointsFromDatabase(t *testing.T) {
	store := setupTestStorage(t)
	for _, name := range []string{"tenant-b", "tenant-a"} {
		assert.NilError(t, store.SaveEndpoint(createTestEndpoint(name)))
	}

	cache := NewConfigStore()
	assert.NilError(t, LoadEndpointsFromDatabase(store, cache))

	var names []string
	for _, endpoint := range cache.GetAllEndpoints() {
		names = append(names, endpoint.GetHandle())
	}
	assert.DeepEqual(t, names, []string{"tenant-a", "tenant-b"})
	assert.Equal(t, len(cache.GetAll()), 0, "Endpoints must not appear among the API configs")
}

func TestConfigStore_Endpoints(t *testing.T) {
	cache := NewConfigStore()
	endpoint := createTestEndpoint("tenant-a")

	assert.NilError(t, cache.SetEndpoint(endpoint))
	got, ok := cache.GetEndpoint("tenant-a")
	assert.Assert(t, ok)
	assert.Equal(t, got.UUID, endpoint.UUID)
	byUUID, ok := cache.GetEndpointByUUID(endpoint.UUID)
	assert.Assert(t, ok)
	assert.Equal(t, byUUID.GetHandle(), "tenant-a")

	// Replacing the same Endpoint is an update.
	updated := *endpoint
	newBasePath := "/v2"
	updated.Configuration.Spec.BasePath = &newBasePath
	assert.NilError(t, cache.SetEndpoint(&updated))
	got, _ = cache.GetEndpoint("tenant-a")
	assert.Equal(t, got.BasePath(), "/v2")
	assert.Equal(t, len(cache.GetAllEndpoints()), 1)

	// Another Endpoint cannot take the name, and a stored one cannot be renamed.
	assert.ErrorContains(t, cache.SetEndpoint(createTestEndpoint("tenant-a")), "already exists")
	renamed := *endpoint
	renamed.Configuration.Metadata.Name = "tenant-b"
	assert.ErrorContains(t, cache.SetEndpoint(&renamed), "cannot be renamed")
	assert.ErrorContains(t, cache.SetEndpoint(nil), "required")
	assert.ErrorContains(t, cache.SetEndpoint(&models.StoredEndpoint{}), "required")

	removed := cache.DeleteEndpoint("tenant-a")
	assert.Assert(t, removed != nil)
	assert.Assert(t, cache.DeleteEndpoint("tenant-a") == nil, "deleting twice is harmless")
	_, ok = cache.GetEndpoint("tenant-a")
	assert.Assert(t, !ok)
	_, ok = cache.GetEndpointByUUID(endpoint.UUID)
	assert.Assert(t, !ok)
}
