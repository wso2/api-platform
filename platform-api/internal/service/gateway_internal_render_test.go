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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
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

// renderITEventAt is the deploy event's performedAt. The gateway echoes it in
// its ack, and the ack guard compares it with deployment_status.performed_at.
var renderITEventAt = time.Date(2026, 10, 10, 8, 59, 15, 0, time.UTC)

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

// deploy stores a DEPLOYING deployment (desired DEPLOYED) of the artifact on
// the gateway, stamped with the deploy event's performedAt, and returns its id.
func (e *renderITEnv) deploy(t *testing.T, artifactID, gatewayID string, content []byte) string {
	t.Helper()
	_, err := e.db.Exec(`INSERT OR IGNORE INTO artifacts (uuid, type, organization_uuid) VALUES (?, ?, ?)`, artifactID, constants.RestApi, renderITOrg)
	require.NoError(t, err)
	deploymentID := "dep-" + artifactID + "-" + gatewayID
	_, err = e.db.Exec(`INSERT INTO deployments (uuid, display_name, artifact_uuid, organization_uuid, gateway_uuid, content, metadata, created_by)
		VALUES (?, 'dep', ?, ?, ?, ?, '{}', 'tester')`, deploymentID, artifactID, renderITOrg, gatewayID, content)
	require.NoError(t, err)
	_, err = e.db.Exec(`INSERT INTO deployment_status (artifact_uuid, organization_uuid, gateway_uuid, deployment_uuid, status, status_desired, performed_at)
		VALUES (?, ?, ?, ?, 'DEPLOYING', 'DEPLOYED', ?)`, artifactID, renderITOrg, gatewayID, deploymentID, renderITEventAt)
	require.NoError(t, err)
	return deploymentID
}

// status returns the deployment_status row of a deployment.
func (e *renderITEnv) status(t *testing.T, deploymentID string) (status, desired, reason string) {
	t.Helper()
	var reasonNull sql.NullString
	require.NoError(t, e.db.QueryRow(`SELECT status, status_desired, status_reason FROM deployment_status WHERE deployment_uuid = ?`,
		deploymentID).Scan(&status, &desired, &reasonNull))
	return status, desired, reasonNull.String
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

// singleFetches are the per-kind fetches that read the deployment directly.
// The MCP, Agent, WebSub and WebBroker fetches look the artifact up in their
// own repository first, then go through the same deliverDeployment.
var singleFetches = []struct {
	kind  string
	fetch func(*GatewayInternalAPIService, string, string, string) (map[string]string, error)
}{
	{"REST API", (*GatewayInternalAPIService).GetActiveDeploymentByGateway},
	{"LLM provider", (*GatewayInternalAPIService).GetActiveLLMProviderDeploymentByGateway},
	{"LLM proxy", (*GatewayInternalAPIService).GetActiveLLMProxyDeploymentByGateway},
}

// An old gateway asking for a deployment whose secret cannot be resolved is
// refused with a typed error, and the refusal is recorded on the deployment
// in a way the gateway's own failed ack for that deploy event cannot undo.
func TestGatewayInternal_Delivery_UnresolvableSecretRefusesTheSingleFetch(t *testing.T) {
	env := setupRenderITEnv(t)
	gatewayID := env.gateways["1.1.0"]

	// Control: on a deployment nobody refused, the gateway's failed ack for
	// the deploy event lands, so the guard below is a real one.
	control := env.deploy(t, "art-render-3-control", gatewayID, renderITContent(renderITSecret))
	rows, err := env.svc.deploymentRepo.UpdateStatusWithPerformedAtGuard("art-render-3-control", renderITOrg, gatewayID,
		model.DeploymentStatusFailed, model.DeploymentErrorGatewayFailure, renderITEventAt, nil)
	require.NoError(t, err)
	require.EqualValues(t, 1, rows, "the ack guard matches the deploy event's performed_at")
	_, _, reason := env.status(t, control)
	require.Equal(t, model.DeploymentErrorGatewayFailure, reason)

	for _, tc := range singleFetches {
		t.Run(tc.kind, func(t *testing.T) {
			artifactID := "art-render-3-" + strings.ReplaceAll(strings.ToLower(tc.kind), " ", "-")
			deploymentID := env.deploy(t, artifactID, gatewayID, renderITContent("does-not-exist"))

			_, err := tc.fetch(env.svc, artifactID, renderITOrg, gatewayID)
			require.Error(t, err)
			assert.True(t, apperror.DeploymentSecretResolutionFailed.Is(err), "typed, so the handler can tell the gateway why: %v", err)
			assert.Contains(t, err.Error(), "does-not-exist")

			status, desired, reason := env.status(t, deploymentID)
			assert.Equal(t, string(model.DeploymentStatusFailed), status)
			assert.Equal(t, string(model.DeploymentStatusDeployed), desired, "the next startup sync asks for it again")
			assert.Equal(t, model.DeploymentErrorSecretResolutionFailed, reason)
			assert.Equal(t, renderITContent("does-not-exist"), env.storedContent(t, deploymentID), "stored content is untouched")

			// The gateway acks the deploy event it was refused as failed with
			// its generic code. The refusal re-stamped performed_at, so that
			// ack is discarded and the specific reason stays.
			rows, err := env.svc.deploymentRepo.UpdateStatusWithPerformedAtGuard(artifactID, renderITOrg, gatewayID,
				model.DeploymentStatusFailed, model.DeploymentErrorGatewayFailure, renderITEventAt, nil)
			require.NoError(t, err)
			assert.Zero(t, rows, "the gateway's failed ack for the refused deploy event is discarded")
			_, _, reason = env.status(t, deploymentID)
			assert.Equal(t, model.DeploymentErrorSecretResolutionFailed, reason)
		})
	}

	t.Run("deprecated secret", func(t *testing.T) {
		createTestSecret(t, env.secrets, renderITOrg, "render-it-retired", "old-value")
		_, err := env.db.Exec(`UPDATE secrets SET status = ? WHERE organization_uuid = ? AND handle = ?`,
			model.SecretStatusDeprecated, renderITOrg, "render-it-retired")
		require.NoError(t, err)
		deploymentID := env.deploy(t, "art-render-3-deprecated", gatewayID, renderITContent("render-it-retired"))

		_, err = env.svc.GetActiveDeploymentByGateway("art-render-3-deprecated", renderITOrg, gatewayID)
		require.Error(t, err)
		assert.True(t, apperror.DeploymentSecretResolutionFailed.Is(err), "%v", err)
		_, _, reason := env.status(t, deploymentID)
		assert.Equal(t, model.DeploymentErrorSecretResolutionFailed, reason)
	})
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

	// The gateway will not retry a missing batch entry, so the skipped
	// deployment is marked FAILED with a reason while its desired state and
	// stored content are untouched; the delivered one keeps its status.
	status, desired, reason := env.status(t, bad)
	assert.Equal(t, string(model.DeploymentStatusFailed), status)
	assert.Equal(t, string(model.DeploymentStatusDeployed), desired)
	assert.Equal(t, model.DeploymentErrorSecretResolutionFailed, reason)
	assert.Equal(t, renderITContent("does-not-exist"), env.storedContent(t, bad))
	status, _, reason = env.status(t, good)
	assert.Equal(t, "DEPLOYING", status)
	assert.Empty(t, reason)

	t.Run("a current gateway gets the batch untouched", func(t *testing.T) {
		current := env.gateways["1.2.0"]
		id := env.deploy(t, "art-render-4c", current, renderITContent(renderITSecret))
		batch, err := env.svc.GetDeploymentContentBatch(renderITOrg, current, []string{id})
		require.NoError(t, err)
		require.Len(t, batch, 1)
		assert.Equal(t, renderITContent(renderITSecret), batch[id].Content)
	})
}

// A secret row written by the v1 -> v2 migration has NULL description,
// created_by and updated_by. It must still be readable, so an old gateway's
// fetch of a deployment that references it is delivered with the plaintext.
func TestGatewayInternal_Delivery_MigratedSecretWithNullColumnsIsDelivered(t *testing.T) {
	env := setupRenderITEnv(t)
	createTestSecret(t, env.secrets, renderITOrg, "render-it-migrated", "sk-migrated")
	_, err := env.db.Exec(`UPDATE secrets SET description = NULL, created_by = NULL, updated_by = NULL WHERE organization_uuid = ? AND handle = ?`,
		renderITOrg, "render-it-migrated")
	require.NoError(t, err)
	gatewayID := env.gateways["1.1.0"]
	deploymentID := env.deploy(t, "art-render-7", gatewayID, renderITContent("render-it-migrated"))

	out, err := env.svc.GetActiveLLMProviderDeploymentByGateway("art-render-7", renderITOrg, gatewayID)
	require.NoError(t, err)
	assert.Equal(t, "sk-migrated", paramValue(t, []byte(out["art-render-7"])))
	status, _, reason := env.status(t, deploymentID)
	assert.Equal(t, "DEPLOYING", status)
	assert.Empty(t, reason)
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
	gatewayID := env.gateways["1.1.0"]
	deploymentID := env.deploy(t, "art-render-6", gatewayID, renderITContent(renderITSecret))
	deployment := &model.Deployment{DeploymentID: deploymentID, ArtifactID: "art-render-6", Content: renderITContent(renderITSecret)}

	_, err := env.svc.deliverDeployment("another-org", gatewayID, deployment)
	assert.Error(t, err)
	status, _, reason := env.status(t, deploymentID)
	assert.Equal(t, "DEPLOYING", status, "a refused gateway lookup records nothing on the deployment")
	assert.Empty(t, reason)
}

// A failure to look the secret up at all — the store unreachable — is not the
// deployment's fault: the single fetch fails with a plain error and records
// nothing, and a batch fails as a whole instead of marking its entries.
func TestGatewayInternal_Delivery_RepositoryFailureIsNotRecorded(t *testing.T) {
	env := setupRenderITEnv(t)
	gatewayID := env.gateways["1.1.0"]
	deploymentID := env.deploy(t, "art-render-8", gatewayID, renderITContent(renderITSecret))
	_, err := env.db.Exec(`ALTER TABLE secrets RENAME TO secrets_unreachable`)
	require.NoError(t, err)

	_, err = env.svc.GetActiveDeploymentByGateway("art-render-8", renderITOrg, gatewayID)
	require.Error(t, err)
	assert.False(t, apperror.DeploymentSecretResolutionFailed.Is(err), "a repository error is not an unresolvable secret: %v", err)
	status, _, reason := env.status(t, deploymentID)
	assert.Equal(t, "DEPLOYING", status)
	assert.Empty(t, reason)

	_, err = env.svc.GetDeploymentContentBatch(renderITOrg, gatewayID, []string{deploymentID})
	require.Error(t, err)
	status, _, reason = env.status(t, deploymentID)
	assert.Equal(t, "DEPLOYING", status)
	assert.Empty(t, reason)
}

// A refusal records nothing when a newer deployment replaced the fetched one
// on the gateway meanwhile: the status row belongs to the newer deployment.
func TestGatewayInternal_Delivery_RefusalLeavesANewerDeploymentAlone(t *testing.T) {
	env := setupRenderITEnv(t)
	gatewayID := env.gateways["1.1.0"]
	old := env.deploy(t, "art-render-9", gatewayID, renderITContent("does-not-exist"))
	_, err := env.db.Exec(`INSERT INTO deployments (uuid, display_name, artifact_uuid, organization_uuid, gateway_uuid, content, metadata, created_by)
		VALUES ('dep-render-9-newer', 'dep', 'art-render-9', ?, ?, ?, '{}', 'tester')`, renderITOrg, gatewayID, renderITContent(renderITSecret))
	require.NoError(t, err)
	_, err = env.db.Exec(`UPDATE deployment_status SET deployment_uuid = 'dep-render-9-newer' WHERE deployment_uuid = ?`, old)
	require.NoError(t, err)

	env.svc.recordSecretResolutionFailure(renderITOrg, gatewayID, "art-render-9", old)

	status, _, reason := env.status(t, "dep-render-9-newer")
	assert.Equal(t, "DEPLOYING", status, "the newer deployment keeps its status")
	assert.Empty(t, reason)
}
