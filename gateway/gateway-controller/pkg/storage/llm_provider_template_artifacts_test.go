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
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"gotest.tools/v3/assert"
)

// pendingContains reports whether the pending-CP-sync artifact list contains the given UUID.
func pendingContains(configs []*models.StoredConfig, uuid string) bool {
	for _, c := range configs {
		if c.UUID == uuid {
			return true
		}
	}
	return false
}

// TestSaveLLMProviderTemplate_TracksArtifact verifies that creating a template writes an
// artifacts tracking row (origin gateway_api, cp_sync_status pending) so the template
// participates in DP->CP sync — including the connect/reconnect retry, which reads
// GetPendingCPSyncArtifacts and reconstructs the full config via GetConfig.
func TestSaveLLMProviderTemplate_TracksArtifact(t *testing.T) {
	store := setupTestStorage(t)
	tmpl := createTestLLMProviderTemplate()

	assert.NilError(t, store.SaveLLMProviderTemplate(tmpl))

	// The artifacts row is created and reconstructable as a StoredConfig.
	cfg, err := store.GetConfig(tmpl.UUID)
	assert.NilError(t, err)
	assert.Equal(t, cfg.Kind, string(models.KindLlmProviderTemplate))
	assert.Equal(t, cfg.Handle, tmpl.GetHandle())
	assert.Equal(t, cfg.Origin, models.OriginGatewayAPI)
	assert.Equal(t, cfg.CPSyncStatus, models.CPSyncStatusPending)
	assert.Equal(t, cfg.DesiredState, models.StateDeployed)
	// The full template configuration is reconstructed from llm_provider_templates.
	assert.Assert(t, cfg.Configuration != nil, "template configuration should be reconstructed")

	// It shows up in the pending-CP-sync set the reconnect push consumes.
	pending, err := store.GetPendingCPSyncArtifacts()
	assert.NilError(t, err)
	assert.Assert(t, pendingContains(pending, tmpl.UUID), "new template should be pending CP sync")

	// The template's own table row is intact too.
	got, err := store.GetLLMProviderTemplate(tmpl.UUID)
	assert.NilError(t, err)
	assert.Equal(t, got.GetHandle(), tmpl.GetHandle())
}

// TestUpdateLLMProviderTemplate_ResetsCPSyncPending verifies that updating a template whose
// previous push already succeeded resets its CP sync state back to pending, so the new
// version re-syncs (and is retried on reconnect).
func TestUpdateLLMProviderTemplate_ResetsCPSyncPending(t *testing.T) {
	store := setupTestStorage(t)
	tmpl := createTestLLMProviderTemplate()
	assert.NilError(t, store.SaveLLMProviderTemplate(tmpl))

	// Simulate a successful push: status success + a CP-assigned artifact id.
	const cpID = "cp-artifact-123"
	assert.NilError(t, store.UpdateCPSyncStatus(tmpl.UUID, cpID, models.CPSyncStatusSuccess, ""))

	cfg, err := store.GetConfig(tmpl.UUID)
	assert.NilError(t, err)
	assert.Equal(t, cfg.CPSyncStatus, models.CPSyncStatusSuccess)

	pending, err := store.GetPendingCPSyncArtifacts()
	assert.NilError(t, err)
	assert.Assert(t, !pendingContains(pending, tmpl.UUID), "synced template must not be pending")

	// Update the template (same handle) -> CP sync resets to pending.
	tmpl.Configuration.Spec.DisplayName = "Updated Display Name"
	assert.NilError(t, store.UpdateLLMProviderTemplate(tmpl))

	cfg, err = store.GetConfig(tmpl.UUID)
	assert.NilError(t, err)
	assert.Equal(t, cfg.CPSyncStatus, models.CPSyncStatusPending)
	// The CP artifact id is preserved across the update.
	assert.Equal(t, cfg.CPArtifactID, cpID)

	pending, err = store.GetPendingCPSyncArtifacts()
	assert.NilError(t, err)
	assert.Assert(t, pendingContains(pending, tmpl.UUID), "updated template should be pending CP sync again")
}

// TestUpdateLLMProviderTemplate_BackfillsLegacyArtifactRow verifies that updating a template
// that predates artifacts-table tracking (only an llm_provider_templates row, no artifacts
// row) backfills the artifacts tracking row, so legacy templates become CP-syncable.
func TestUpdateLLMProviderTemplate_BackfillsLegacyArtifactRow(t *testing.T) {
	store := setupTestStorage(t)
	tmpl := createTestLLMProviderTemplate()

	// Insert ONLY the llm_provider_templates row, bypassing the dual-write, to mimic a
	// template created before this feature existed.
	cfgJSON, err := json.Marshal(tmpl.Configuration)
	assert.NilError(t, err)
	now := time.Now()
	_, err = store.exec(`
		INSERT INTO llm_provider_templates (uuid, gateway_id, handle, configuration, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, tmpl.UUID, store.gatewayId, tmpl.GetHandle(), string(cfgJSON), now, now)
	assert.NilError(t, err)

	// No artifacts row yet.
	_, err = store.GetConfig(tmpl.UUID)
	assert.Assert(t, errors.Is(err, ErrNotFound))

	// Updating backfills the artifacts row.
	assert.NilError(t, store.UpdateLLMProviderTemplate(tmpl))

	cfg, err := store.GetConfig(tmpl.UUID)
	assert.NilError(t, err)
	assert.Equal(t, cfg.Kind, string(models.KindLlmProviderTemplate))
	assert.Equal(t, cfg.Origin, models.OriginGatewayAPI)
	assert.Equal(t, cfg.CPSyncStatus, models.CPSyncStatusPending)

	pending, err := store.GetPendingCPSyncArtifacts()
	assert.NilError(t, err)
	assert.Assert(t, pendingContains(pending, tmpl.UUID), "backfilled template should be pending CP sync")
}

// TestDeleteLLMProviderTemplate_RemovesArtifactRow verifies that deleting a template removes
// both its configuration row and its artifacts tracking row, so it is no longer pushed.
func TestDeleteLLMProviderTemplate_RemovesArtifactRow(t *testing.T) {
	store := setupTestStorage(t)
	tmpl := createTestLLMProviderTemplate()
	assert.NilError(t, store.SaveLLMProviderTemplate(tmpl))

	assert.NilError(t, store.DeleteLLMProviderTemplate(tmpl.UUID))

	// The artifacts tracking row is gone.
	_, err := store.GetConfig(tmpl.UUID)
	assert.Assert(t, errors.Is(err, ErrNotFound))

	// The template row is gone.
	_, err = store.GetLLMProviderTemplate(tmpl.UUID)
	assert.Assert(t, errors.Is(err, ErrNotFound))

	// It is no longer pending CP sync.
	pending, err := store.GetPendingCPSyncArtifacts()
	assert.NilError(t, err)
	assert.Assert(t, !pendingContains(pending, tmpl.UUID), "deleted template must not be pending")
}

// TestSaveLLMProviderTemplate_DuplicateHandleConflicts verifies the dual-write still enforces
// the unique-handle constraint (now backed by both tables) and does not leave a partial row.
func TestSaveLLMProviderTemplate_DuplicateHandleConflicts(t *testing.T) {
	store := setupTestStorage(t)
	tmpl := createTestLLMProviderTemplate()
	assert.NilError(t, store.SaveLLMProviderTemplate(tmpl))

	dup := createTestLLMProviderTemplate()
	dup.Configuration.Metadata.Name = tmpl.GetHandle() // same handle, different UUID

	err := store.SaveLLMProviderTemplate(dup)
	assert.Assert(t, errors.Is(err, ErrConflict))

	// The conflicting save left no orphan artifacts row for the duplicate's UUID.
	_, err = store.GetConfig(dup.UUID)
	assert.Assert(t, errors.Is(err, ErrNotFound))
}

// TestGetGatewayOriginArtifactsForSync_IncludesSuccess verifies the full-reconcile query returns
// ALL gateway-origin artifacts regardless of cp_sync_status — including already-"success" ones —
// whereas GetPendingCPSyncArtifacts excludes them. This is what lets a reconnect re-sync artifacts
// to a new/purged control plane (#2659).
func TestGetGatewayOriginArtifactsForSync_IncludesSuccess(t *testing.T) {
	store := setupTestStorage(t)

	// A synced (success) template and a not-yet-synced (pending) one.
	synced := createTestLLMProviderTemplate()
	pendingTmpl := createTestLLMProviderTemplate()
	assert.NilError(t, store.SaveLLMProviderTemplate(synced))
	assert.NilError(t, store.SaveLLMProviderTemplate(pendingTmpl))

	// Mark the first as successfully synced.
	assert.NilError(t, store.UpdateCPSyncStatus(synced.UUID, "cp-uuid-1", models.CPSyncStatusSuccess, ""))

	// GetPendingCPSyncArtifacts excludes the success one.
	pending, err := store.GetPendingCPSyncArtifacts()
	assert.NilError(t, err)
	assert.Assert(t, !pendingContains(pending, synced.UUID), "success artifact must NOT be pending")
	assert.Assert(t, pendingContains(pending, pendingTmpl.UUID), "pending artifact must be pending")

	// The full-reconcile query includes BOTH.
	all, err := store.GetGatewayOriginArtifactsForSync()
	assert.NilError(t, err)
	assert.Assert(t, pendingContains(all, synced.UUID), "full reconcile must include the success artifact")
	assert.Assert(t, pendingContains(all, pendingTmpl.UUID), "full reconcile must include the pending artifact")
}
