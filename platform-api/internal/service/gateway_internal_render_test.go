/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package service

import (
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/vault"

	_ "github.com/mattn/go-sqlite3"
)

// Delivery-time secret rendering against a real SQLite database: the stored
// deployment keeps its placeholder, and only a gateway that cannot sync
// secrets receives plaintext.

const (
	renderITOrg       = "org-render-it"
	renderITSecret    = "render-it-key"
	renderITPlaintext = `sk-live "quoted" #hash: value`
)

type renderITEnv struct {
	db       *database.DB
	svc      *GatewayInternalAPIService
	secrets  *SecretService
	gateways map[string]string // version -> gateway uuid
}

// renderITContent is a REST artifact whose policy param references a secret.
func renderITContent(handle string) []byte {
	return []byte("apiVersion: gateway.api-platform.wso2.com/v1\nkind: RestApi\nmetadata:\n  name: demo\nspec:\n  displayName: Demo\n  policies:\n    - name: set-headers\n      version: v1\n      params:\n        value: '{{ secret \"" + handle + "\" }}'\n")
}

func setupRenderITEnv(t *testing.T) *renderITEnv {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "render-it.db"))
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	_, err = sqlDB.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	db := &database.DB{DB: sqlDB}

	schema, err := os.ReadFile(filepath.Join("..", "database", "schema.sqlite.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(schema))
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
		VALUES (?, 'render-it-org', 'Render IT Org', 'default', 'idp-ref', datetime('now'), datetime('now'))`, renderITOrg)
	require.NoError(t, err)

	gateways := map[string]string{}
	for i, version := range []string{"1.0.0", "1.1.0", "1.2.0", ""} {
		id := "gw-render-" + string(rune('a'+i))
		_, err = db.Exec(`INSERT INTO gateways (uuid, organization_uuid, handle, display_name, description, version, properties, is_active, created_at, updated_at)
			VALUES (?, ?, ?, 'GW', '', ?, '{}', 1, datetime('now'), datetime('now'))`, id, renderITOrg, id, version)
		require.NoError(t, err)
		gateways[version] = id
	}

	v, err := vault.NewInHouseVault([]byte("12345678901234567890123456789012"))
	require.NoError(t, err)
	identity := NewIdentityService(repository.NewUserIdentityMappingRepo(db))
	secretRepo := repository.NewSecretRepo(db)
	secretSvc := NewSecretService(secretRepo, v, identity)
	createTestSecret(t, secretSvc, renderITOrg, renderITSecret, renderITPlaintext)

	deploymentRepo := repository.NewDeploymentRepo(db, repository.NewArtifactTableRegistry())
	svc := NewGatewayInternalAPIService(nil, nil, nil, nil, nil, nil, nil,
		deploymentRepo, repository.NewGatewayRepo(db),
		nil, nil, nil, nil, secretRepo, &config.Server{}, slog.Default())
	svc.SetSecretService(secretSvc)

	return &renderITEnv{db: db, svc: svc, secrets: secretSvc, gateways: gateways}
}

// deploy stores a DEPLOYED artifact for the gateway and returns the deployment id.
func (e *renderITEnv) deploy(t *testing.T, artifactID, gatewayID string, content []byte) string {
	t.Helper()
	_, err := e.db.Exec(`INSERT OR IGNORE INTO artifacts (uuid, type, organization_uuid) VALUES (?, ?, ?)`, artifactID, constants.RestApi, renderITOrg)
	require.NoError(t, err)
	deploymentID := "dep-" + artifactID + "-" + gatewayID
	_, err = e.db.Exec(`INSERT INTO deployments (uuid, display_name, artifact_uuid, organization_uuid, gateway_uuid, content, metadata, created_by)
		VALUES (?, 'dep', ?, ?, ?, ?, '{}', 'tester')`, deploymentID, artifactID, renderITOrg, gatewayID, content)
	require.NoError(t, err)
	_, err = e.db.Exec(`INSERT INTO deployment_status (artifact_uuid, organization_uuid, gateway_uuid, deployment_uuid, status, status_desired)
		VALUES (?, ?, ?, ?, 'DEPLOYING', 'DEPLOYED')`, artifactID, renderITOrg, gatewayID, deploymentID)
	require.NoError(t, err)
	return deploymentID
}

func (e *renderITEnv) storedContent(t *testing.T, deploymentID string) []byte {
	t.Helper()
	var content []byte
	require.NoError(t, e.db.QueryRow(`SELECT content FROM deployments WHERE uuid = ?`, deploymentID).Scan(&content))
	return content
}

func paramValue(t *testing.T, content []byte) string {
	t.Helper()
	var doc struct {
		Spec struct {
			Policies []struct {
				Params map[string]string `yaml:"params"`
			} `yaml:"policies"`
		} `yaml:"spec"`
	}
	require.NoError(t, yaml.Unmarshal(content, &doc))
	require.Len(t, doc.Spec.Policies, 1)
	return doc.Spec.Policies[0].Params["value"]
}

func TestGatewayInternal_Delivery_OldGatewaysGetPlaintextStoredContentKeepsPlaceholder(t *testing.T) {
	env := setupRenderITEnv(t)
	const artifactID = "art-render-1"

	for _, version := range []string{"1.0.0", "1.1.0"} {
		t.Run("gateway "+version, func(t *testing.T) {
			gatewayID := env.gateways[version]
			deploymentID := env.deploy(t, artifactID, gatewayID, renderITContent(renderITSecret))

			out, err := env.svc.GetActiveDeploymentByGateway(artifactID, renderITOrg, gatewayID)
			require.NoError(t, err)
			delivered := []byte(out[artifactID])
			assert.Equal(t, renderITPlaintext, paramValue(t, delivered))
			assert.False(t, constants.SecretPlaceholderRe.Match(delivered), "no placeholder reaches the gateway")

			stored := env.storedContent(t, deploymentID)
			assert.True(t, constants.SecretPlaceholderRe.Match(stored), "the stored deployment keeps its placeholder")
			assert.Equal(t, renderITContent(renderITSecret), stored)
		})
	}
}

func TestGatewayInternal_Delivery_CurrentGatewaysGetStoredBytesUnchanged(t *testing.T) {
	env := setupRenderITEnv(t)
	const artifactID = "art-render-2"

	for _, version := range []string{"1.2.0", ""} {
		t.Run("gateway "+version, func(t *testing.T) {
			gatewayID := env.gateways[version]
			env.deploy(t, artifactID, gatewayID, renderITContent(renderITSecret))

			out, err := env.svc.GetActiveDeploymentByGateway(artifactID, renderITOrg, gatewayID)
			require.NoError(t, err)
			assert.Equal(t, string(renderITContent(renderITSecret)), out[artifactID], "byte-identical, placeholder included")
		})
	}
}

func TestGatewayInternal_Delivery_UnknownSecretFailsTheSingleFetch(t *testing.T) {
	env := setupRenderITEnv(t)
	gatewayID := env.gateways["1.1.0"]
	env.deploy(t, "art-render-3", gatewayID, renderITContent("does-not-exist"))

	_, err := env.svc.GetActiveDeploymentByGateway("art-render-3", renderITOrg, gatewayID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does-not-exist")
}

func TestGatewayInternal_Delivery_BatchSkipsOnlyTheFailingDeployment(t *testing.T) {
	env := setupRenderITEnv(t)
	gatewayID := env.gateways["1.1.0"]
	good := env.deploy(t, "art-render-4a", gatewayID, renderITContent(renderITSecret))
	bad := env.deploy(t, "art-render-4b", gatewayID, renderITContent("does-not-exist"))

	batch, err := env.svc.GetDeploymentContentBatch(renderITOrg, gatewayID, []string{good, bad})
	require.NoError(t, err)
	require.Len(t, batch, 1, "the failing deployment is left out, the rest is delivered")
	require.Contains(t, batch, good)
	assert.Equal(t, renderITPlaintext, paramValue(t, batch[good].Content))
	assert.Equal(t, "art-render-4a", batch[good].ArtifactID)
	assert.Equal(t, constants.RestApi, batch[good].Type)

	t.Run("a current gateway gets the batch untouched", func(t *testing.T) {
		current := env.gateways["1.2.0"]
		id := env.deploy(t, "art-render-4c", current, renderITContent(renderITSecret))
		batch, err := env.svc.GetDeploymentContentBatch(renderITOrg, current, []string{id})
		require.NoError(t, err)
		require.Len(t, batch, 1)
		assert.Equal(t, renderITContent(renderITSecret), batch[id].Content)
	})
}

func TestGatewayInternal_Delivery_WithoutSecretServiceFailsClosed(t *testing.T) {
	env := setupRenderITEnv(t)
	env.svc.SetSecretService(nil)
	old := &model.Gateway{ID: env.gateways["1.1.0"], OrganizationID: renderITOrg, Version: "1.1.0"}

	_, err := env.svc.renderContentForGateway(renderITOrg, old, renderITContent(renderITSecret))
	assert.Error(t, err, "an old gateway must never receive an unresolved placeholder")

	plain := []byte("apiVersion: v1\nkind: RestApi\nspec: {}\n")
	out, err := env.svc.renderContentForGateway(renderITOrg, old, plain)
	require.NoError(t, err)
	assert.Equal(t, plain, out, "content without placeholders needs no secret store")
}

func TestGatewayInternal_Delivery_GatewayMustBelongToTheOrganization(t *testing.T) {
	env := setupRenderITEnv(t)
	_, err := env.svc.deliverContent("another-org", env.gateways["1.1.0"], renderITContent(renderITSecret))
	assert.Error(t, err)
}
