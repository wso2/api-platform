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
	"strings"
	"testing"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"gotest.tools/v3/assert"
)

func setupTestSQLServerStorage(t *testing.T) Storage {
	t.Helper()

	dsn := os.Getenv("SQLSERVER_TEST_DSN")
	if dsn == "" {
		t.Skip("SQLSERVER_TEST_DSN is not set; skipping sqlserver integration tests")
	}

	prevMetricsEnabled := metrics.IsEnabled()
	t.Cleanup(func() { metrics.SetEnabled(prevMetricsEnabled) })
	metrics.SetEnabled(false)
	metrics.Init()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ms, err := NewStorage(BackendConfig{Type: "sqlserver", SQLServer: SQLServerConnectionConfig{DSN: dsn}}, logger)
	assert.NilError(t, err)
	// Registered first so it runs last (t.Cleanup is LIFO) — after any
	// per-test row cleanups that still need the connection.
	t.Cleanup(func() { _ = ms.Close() })
	return ms
}

func TestNewSQLServerStorage_Success(t *testing.T) {
	ms := setupTestSQLServerStorage(t)
	assert.Assert(t, ms != nil)
}

func TestSQLServerStorage_ConfigCRUD(t *testing.T) {
	ms := setupTestSQLServerStorage(t)

	cfg := createTestStoredConfig()
	assert.NilError(t, ms.SaveConfig(cfg))
	t.Cleanup(func() {
		if err := ms.DeleteConfig(cfg.UUID); err != nil && !IsNotFoundError(err) {
			t.Errorf("cleanup DeleteConfig(%s): %v", cfg.UUID, err)
		}
	})

	stored, err := ms.GetConfig(cfg.UUID)
	assert.NilError(t, err)
	assert.Equal(t, stored.UUID, cfg.UUID)
	assert.Equal(t, stored.Handle, cfg.Handle)

	assert.NilError(t, ms.DeleteConfig(cfg.UUID))
	_, err = ms.GetConfig(cfg.UUID)
	assert.Assert(t, err != nil)
	assert.Assert(t, IsNotFoundError(err))
}

func TestSQLServerStorage_TemplateAndAPIKeyCRUD(t *testing.T) {
	ms := setupTestSQLServerStorage(t)

	tmpl := createTestLLMProviderTemplate()
	assert.NilError(t, ms.SaveLLMProviderTemplate(tmpl))
	t.Cleanup(func() {
		if err := ms.DeleteLLMProviderTemplate(tmpl.UUID); err != nil && !IsNotFoundError(err) {
			t.Errorf("cleanup DeleteLLMProviderTemplate(%s): %v", tmpl.UUID, err)
		}
	})

	loadedTemplate, err := ms.GetLLMProviderTemplate(tmpl.UUID)
	assert.NilError(t, err)
	assert.Equal(t, loadedTemplate.UUID, tmpl.UUID)
	assert.Equal(t, loadedTemplate.GetHandle(), tmpl.GetHandle())

	cfg := createTestStoredConfig()
	assert.NilError(t, ms.SaveConfig(cfg))
	t.Cleanup(func() {
		if err := ms.DeleteConfig(cfg.UUID); err != nil && !IsNotFoundError(err) {
			t.Errorf("cleanup DeleteConfig(%s): %v", cfg.UUID, err)
		}
	})

	apiKey := createTestAPIKey()
	apiKey.ArtifactUUID = cfg.UUID
	apiKey.Source = "local"

	preInsertCount, err := ms.CountActiveAPIKeysByUserAndAPI(apiKey.ArtifactUUID, apiKey.CreatedBy)
	assert.NilError(t, err)

	assert.NilError(t, ms.SaveAPIKey(apiKey))
	t.Cleanup(func() {
		if err := ms.RemoveAPIKeyAPIAndName(apiKey.ArtifactUUID, apiKey.Name); err != nil && !IsNotFoundError(err) {
			t.Errorf("cleanup RemoveAPIKeyAPIAndName(%s, %s): %v", apiKey.ArtifactUUID, apiKey.Name, err)
		}
	})

	loadedKey, err := ms.GetAPIKeyByID(apiKey.UUID)
	assert.NilError(t, err)
	assert.Equal(t, loadedKey.UUID, apiKey.UUID)
	assert.Equal(t, loadedKey.ArtifactUUID, apiKey.ArtifactUUID)

	postInsertCount, err := ms.CountActiveAPIKeysByUserAndAPI(apiKey.ArtifactUUID, apiKey.CreatedBy)
	assert.NilError(t, err)
	assert.Equal(t, postInsertCount, preInsertCount+1)
}

func TestSQLServerStorage_SaveLLMProviderTemplate_UniqueConstraintError(t *testing.T) {
	ms := setupTestSQLServerStorage(t)

	template := createTestLLMProviderTemplate()
	assert.NilError(t, ms.SaveLLMProviderTemplate(template))
	t.Cleanup(func() {
		if err := ms.DeleteLLMProviderTemplate(template.UUID); err != nil && !IsNotFoundError(err) {
			t.Errorf("cleanup DeleteLLMProviderTemplate(%s): %v", template.UUID, err)
		}
	})

	conflictingTemplate := createTestLLMProviderTemplate()
	conflictingTemplate.Configuration.Metadata.Name = template.Configuration.Metadata.Name

	err := ms.SaveLLMProviderTemplate(conflictingTemplate)
	assert.Assert(t, errors.Is(err, ErrConflict))
}

// TestSanitizeSQLServerDSN verifies passwords are redacted before logging across
// the DSN formats go-mssqldb accepts: URL (userinfo and query), ADO and ODBC
// (semicolon-separated). This is a pure-unit test (no database required).
func TestSanitizeSQLServerDSN(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
	}{
		{"url userinfo", "sqlserver://sa:secret@host:1433?database=db"},
		{"url query password", "sqlserver://host:1433?user+id=sa&password=secret&database=db"},
		{"ado semicolon", "server=host;user id=sa;password=secret;encrypt=disable"},
		{"odbc semicolon", "odbc:server=host;uid=sa;pwd=secret;"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeSQLServerDSN(tc.dsn)
			assert.Assert(t, !strings.Contains(got, "secret"),
				"password leaked in sanitized DSN: %q", got)
			assert.Assert(t, got != tc.dsn,
				"DSN was returned unchanged (password not redacted): %q", got)
		})
	}
}
