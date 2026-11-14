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

	"gotest.tools/v3/assert"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

func testModelConfig(handle, modelName string) *models.StoredConfig {
	m := api.ModelConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1",
		Kind:       "Model",
		Metadata:   api.Metadata{Name: handle},
		Spec: api.ModelConfigData{
			DisplayName: handle,
			Version:     "v1.0",
			ModelName:   modelName,
			Endpoints:   []string{"openai"},
			Targets: []api.ModelTarget{{
				Name: modelName, Provider: "openai", Alias: strPtr("openai"),
			}},
		},
	}
	now := time.Now()
	return &models.StoredConfig{
		UUID: "model-" + handle, Kind: models.KindModel, Handle: handle, DisplayName: handle, Version: "v1.0",
		Configuration: m, SourceConfiguration: m,
		DesiredState: models.StateDeployed, Origin: models.OriginGatewayAPI, CreatedAt: now, UpdatedAt: now,
	}
}

func testModelEndpointConfig(handle, context string) *models.StoredConfig {
	e := api.ModelEndpointConfiguration{
		ApiVersion: api.ModelEndpointConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.ModelEndpointConfigurationKindModelEndpoint,
		Metadata:   api.Metadata{Name: handle},
		Spec: api.ModelEndpointConfigData{
			DisplayName: handle + " endpoint", Version: "v1.0", Context: &context,
			Transformers: &[]api.ModelEndpointTransformer{{Template: strPtr("anthropic"), Type: "none"}},
		},
	}
	now := time.Now()
	return &models.StoredConfig{
		UUID: models.ModelEndpointUUID(handle), Kind: models.KindModelEndpoint, Handle: handle,
		DisplayName: e.Spec.DisplayName, Version: "v1.0", Configuration: e, SourceConfiguration: e,
		DesiredState: models.StateDeployed, Origin: models.OriginGatewayAPI, CreatedAt: now, UpdatedAt: now,
	}
}

func strPtr(s string) *string { return &s }

// testProviderAndProxy saves a provider and an ordinary proxy on it, so a listing of
// proxies has rows in both proxy tables.
func testProviderAndProxy(t *testing.T, s *sqlStore) {
	t.Helper()
	provider := createTestStoredConfig()
	provider.UUID, provider.Kind, provider.Handle = "provider-1", models.KindLlmProvider, "openai"
	provider.SourceConfiguration = api.LLMProviderConfiguration{
		ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProviderConfigurationKindLlmProvider,
		Metadata:   api.Metadata{Name: "openai"},
	}
	assert.NilError(t, s.SaveConfig(provider))

	proxy := createTestStoredConfig()
	proxy.UUID, proxy.Kind, proxy.Handle = "proxy-1", models.KindLlmProxy, "openai-proxy"
	proxy.SourceConfiguration = api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProxyConfigurationKindLlmProxy,
		Metadata:   api.Metadata{Name: "openai-proxy"},
		Spec:       api.LLMProxyConfigData{Provider: &api.LLMProxyProvider{Id: "openai"}},
	}
	assert.NilError(t, s.SaveConfig(proxy))
}

func TestSQLiteStorage_ModelRoundTrip(t *testing.T) {
	s := setupTestStorage(t)
	defer s.db.Close()

	cfg := testModelConfig("gpt", "gpt-4o")
	assert.NilError(t, s.SaveConfig(cfg))

	got, err := s.GetConfig(cfg.UUID)
	assert.NilError(t, err)
	assert.Equal(t, got.Kind, models.KindModel)
	m, ok := got.SourceConfiguration.(api.ModelConfiguration)
	assert.Assert(t, ok, "source is a ModelConfiguration, got %T", got.SourceConfiguration)
	assert.Equal(t, m.Spec.ModelName, "gpt-4o")
	assert.Equal(t, got.DataVersion, "1.0")

	byHandle, err := s.GetConfigByKindAndHandle(models.KindModel, "gpt")
	assert.NilError(t, err)
	assert.Equal(t, byHandle.UUID, cfg.UUID)

	m.Spec.ModelName = "gpt-4o-2024"
	cfg.SourceConfiguration, cfg.Configuration = m, m
	cfg.UpdatedAt = time.Now()
	assert.NilError(t, s.UpdateConfig(cfg))
	got, err = s.GetConfig(cfg.UUID)
	assert.NilError(t, err)
	assert.Equal(t, got.SourceConfiguration.(api.ModelConfiguration).Spec.ModelName, "gpt-4o-2024")

	list, err := s.GetAllConfigsByKind(models.KindModel)
	assert.NilError(t, err)
	assert.Equal(t, len(list), 1)

	assert.NilError(t, s.DeleteConfig(cfg.UUID))
	_, err = s.GetConfig(cfg.UUID)
	assert.Assert(t, errors.Is(err, ErrNotFound))
	var rows int
	assert.NilError(t, s.db.QueryRow(`SELECT COUNT(*) FROM models`).Scan(&rows))
	assert.Equal(t, rows, 0, "the models row is removed with its artifact")
}

// A model endpoint is its own kind with its own table, and LlmProxy reads see
// only proxies again.
func TestSQLiteStorage_ModelEndpointRoundTrip(t *testing.T) {
	s := setupTestStorage(t)
	defer s.db.Close()
	testProviderAndProxy(t, s)

	cfg := testModelEndpointConfig("partners", "/partners/v1")
	assert.NilError(t, s.SaveConfig(cfg))

	var rows int
	assert.NilError(t, s.db.QueryRow(`SELECT COUNT(*) FROM model_endpoints WHERE uuid = ?`, cfg.UUID).Scan(&rows))
	assert.Equal(t, rows, 1)

	got, err := s.GetConfigByKindAndHandle(models.KindModelEndpoint, "partners")
	assert.NilError(t, err)
	e, ok := got.SourceConfiguration.(api.ModelEndpointConfiguration)
	assert.Assert(t, ok, "source is a ModelEndpointConfiguration, got %T", got.SourceConfiguration)
	assert.Equal(t, *e.Spec.Context, "/partners/v1")
	assert.Equal(t, len(*e.Spec.Transformers), 1)
	assert.Equal(t, got.DataVersion, "1.0")

	proxies, err := s.GetAllConfigsByKind(models.KindLlmProxy)
	assert.NilError(t, err)
	assert.Equal(t, len(proxies), 1, "an endpoint is not an LlmProxy")
	endpoints, err := s.GetAllConfigsByKind(models.KindModelEndpoint)
	assert.NilError(t, err)
	assert.Equal(t, len(endpoints), 1)

	all, err := s.GetAllConfigs()
	assert.NilError(t, err)
	found := false
	for _, c := range all {
		if c.UUID == cfg.UUID {
			found = true
			assert.Equal(t, c.Kind, models.KindModelEndpoint)
		}
	}
	assert.Assert(t, found, "GetAllConfigs covers model_endpoints")

	e.Spec.Context = strPtr("/partners/v2")
	cfg.SourceConfiguration, cfg.Configuration = e, e
	cfg.UpdatedAt = time.Now()
	assert.NilError(t, s.UpdateConfig(cfg))
	got, err = s.GetConfig(cfg.UUID)
	assert.NilError(t, err)
	assert.Equal(t, *got.SourceConfiguration.(api.ModelEndpointConfiguration).Spec.Context, "/partners/v2")

	assert.NilError(t, s.DeleteConfig(cfg.UUID))
	assert.NilError(t, s.db.QueryRow(`SELECT COUNT(*) FROM model_endpoints`).Scan(&rows))
	assert.Equal(t, rows, 0, "the model_endpoints row is removed with its artifact")
}

// A version 6 database gains the two tables and is stamped version 7, keeping its rows.
func TestSQLite_UpgradeV6AddsModelTables(t *testing.T) {
	metrics.Init() // GetConfig records metrics; not left to another test to initialise
	dbPath := filepath.Join(t.TempDir(), "v6.db")
	gatewayID := "platform-gateway-id"
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create a current database, drop what v7 added, and stamp it v6.
	fresh, err := NewStorage(BackendConfig{Type: "sqlite", SQLitePath: dbPath, GatewayID: gatewayID}, logger)
	assert.NilError(t, err)
	existing := createTestStoredConfig()
	assert.NilError(t, fresh.SaveConfig(existing))
	assert.NilError(t, fresh.(*sqlStore).db.Close())

	rawDB, err := sql.Open("sqlite3", dbPath)
	assert.NilError(t, err)
	for _, stmt := range []string{`DROP TABLE models`, `DROP TABLE model_endpoints`,
		fmt.Sprintf("PRAGMA user_version = %d", previousSchemaVersion)} {
		_, err := rawDB.Exec(stmt)
		assert.NilError(t, err)
	}
	assert.NilError(t, rawDB.Close())

	upgraded, err := NewStorage(BackendConfig{Type: "sqlite", SQLitePath: dbPath, GatewayID: gatewayID}, logger)
	assert.NilError(t, err)
	s := upgraded.(*sqlStore)
	defer s.db.Close()

	var version int
	assert.NilError(t, s.db.QueryRow("PRAGMA user_version").Scan(&version))
	assert.Equal(t, version, currentSchemaVersion)

	_, err = s.GetConfig(existing.UUID)
	assert.NilError(t, err, "rows written before the upgrade survive it")

	model := testModelConfig("gpt", "gpt-4o")
	assert.NilError(t, s.SaveConfig(model))
	assert.NilError(t, s.SaveConfig(testModelEndpointConfig("openai", "/openai/v1")))
}
