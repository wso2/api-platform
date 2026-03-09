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
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"gotest.tools/v3/assert"
)

func setupTestPostgresStorage(t *testing.T) Storage {
	t.Helper()

	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN is not set; skipping postgres integration tests")
	}

	metrics.SetEnabled(false)
	metrics.Init()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pg, err := NewStorage(BackendConfig{Type: "postgres", Postgres: PostgresConnectionConfig{DSN: dsn}}, logger)
	assert.NilError(t, err)
	return pg
}

func TestNewPostgresStorage_Success(t *testing.T) {
	pg := setupTestPostgresStorage(t)
	defer pg.Close()
	assert.Assert(t, pg != nil)
}

func TestPostgresStorage_ConfigCRUD(t *testing.T) {
	pg := setupTestPostgresStorage(t)
	defer pg.Close()

	cfg := createTestStoredConfig()
	assert.NilError(t, pg.SaveConfig(cfg))
	t.Cleanup(func() {
		if err := pg.DeleteConfig(cfg.UUID); err != nil && !IsNotFoundError(err) {
			t.Errorf("cleanup DeleteConfig(%s): %v", cfg.UUID, err)
		}
	})

	stored, err := pg.GetConfig(cfg.UUID)
	assert.NilError(t, err)
	assert.Equal(t, stored.UUID, cfg.UUID)
	assert.Equal(t, stored.Handle, cfg.Handle)

	assert.NilError(t, pg.DeleteConfig(cfg.UUID))
	_, err = pg.GetConfig(cfg.UUID)
	assert.Assert(t, err != nil)
	assert.Assert(t, IsNotFoundError(err))
}

func TestPostgresStorage_TemplateAndAPIKeyCRUD(t *testing.T) {
	pg := setupTestPostgresStorage(t)
	defer pg.Close()

	tmpl := createTestLLMProviderTemplate()
	assert.NilError(t, pg.SaveLLMProviderTemplate(tmpl))
	t.Cleanup(func() {
		if err := pg.DeleteLLMProviderTemplate(tmpl.UUID); err != nil && !IsNotFoundError(err) {
			t.Errorf("cleanup DeleteLLMProviderTemplate(%s): %v", tmpl.UUID, err)
		}
	})

	loadedTemplate, err := pg.GetLLMProviderTemplate(tmpl.UUID)
	assert.NilError(t, err)
	assert.Equal(t, loadedTemplate.UUID, tmpl.UUID)
	assert.Equal(t, loadedTemplate.GetHandle(), tmpl.GetHandle())

	cfg := createTestStoredConfig()
	assert.NilError(t, pg.SaveConfig(cfg))
	t.Cleanup(func() {
		if err := pg.DeleteConfig(cfg.UUID); err != nil && !IsNotFoundError(err) {
			t.Errorf("cleanup DeleteConfig(%s): %v", cfg.UUID, err)
		}
	})

	apiKey := createTestAPIKey()
	apiKey.ArtifactUUID = cfg.UUID
	apiKey.Source = "local"

	preInsertCount, err := pg.CountActiveAPIKeysByUserAndAPI(apiKey.ArtifactUUID, apiKey.CreatedBy)
	assert.NilError(t, err)

	assert.NilError(t, pg.SaveAPIKey(apiKey))
	t.Cleanup(func() {
		if err := pg.RemoveAPIKeyAPIAndName(apiKey.ArtifactUUID, apiKey.Name); err != nil && !IsNotFoundError(err) {
			t.Errorf("cleanup RemoveAPIKeyAPIAndName(%s, %s): %v", apiKey.ArtifactUUID, apiKey.Name, err)
		}
	})

	loadedKey, err := pg.GetAPIKeyByID(apiKey.UUID)
	assert.NilError(t, err)
	assert.Equal(t, loadedKey.UUID, apiKey.UUID)
	assert.Equal(t, loadedKey.ArtifactUUID, apiKey.ArtifactUUID)

	postInsertCount, err := pg.CountActiveAPIKeysByUserAndAPI(apiKey.ArtifactUUID, apiKey.CreatedBy)
	assert.NilError(t, err)
	assert.Equal(t, postInsertCount, preInsertCount+1)
}

func TestPostgresStorage_SaveLLMProviderTemplate_UniqueConstraintError(t *testing.T) {
	pg := setupTestPostgresStorage(t)
	defer pg.Close()

	template := createTestLLMProviderTemplate()
	assert.NilError(t, pg.SaveLLMProviderTemplate(template))
	t.Cleanup(func() {
		if err := pg.DeleteLLMProviderTemplate(template.UUID); err != nil && !IsNotFoundError(err) {
			t.Errorf("cleanup DeleteLLMProviderTemplate(%s): %v", template.UUID, err)
		}
	})

	conflictingTemplate := createTestLLMProviderTemplate()
	conflictingTemplate.Configuration.Metadata.Name = template.Configuration.Metadata.Name

	err := pg.SaveLLMProviderTemplate(conflictingTemplate)
	assert.Assert(t, errors.Is(err, ErrConflict))
}
