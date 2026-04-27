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

package controlplane

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// syncDiffResult holds the categorised results of comparing remote deployments
// against the local database state.
type syncDiffResult struct {
	// toFetch contains deployments that are missing locally or have a newer
	// deployed_at on the remote side and need their full artifact fetched.
	toFetch []models.ControlPlaneDeployment

	// toUpdateStatus contains deployments whose state differs (e.g. remote
	// says "undeployed" but local still shows "deployed"). Only the status
	// needs updating — no artifact re-fetch required.
	toUpdateStatus []models.ControlPlaneDeployment

	// toDelete contains local artifact IDs that are not present in the remote
	// deployment list and should be deleted as orphans.
	toDelete []string
}

// syncDeployments performs a one-time bulk sync against the platform-API.
// It fetches the expected deployment list, computes a diff against local state,
// and processes fetches, status updates, and deletions with log-and-continue
// error handling so that a single failure does not block the rest of the sync.
func (c *Client) syncDeployments(gatewayID string) {
	c.logger.Info("Starting deployment sync",
		slog.String("gateway_id", gatewayID),
	)

	if c.apiUtilsService == nil {
		c.logger.Error("Cannot sync deployments: apiUtilsService is nil")
		return
	}

	// 1. Fetch expected deployments from platform-API (full sync — no since filter)
	remoteDeployments, err := c.apiUtilsService.FetchControlPlaneDeployments(nil)
	if err != nil {
		c.logger.Error("Failed to fetch control plane deployments for sync",
			slog.Any("error", err),
		)
		return
	}

	c.logger.Info("Fetched remote deployments for sync",
		slog.Int("count", len(remoteDeployments)),
	)

	// 2. Get local control-plane-originated configs from DB (gateway-API configs are excluded)
	localConfigs, err := c.db.GetAllConfigsByOrigin(models.OriginControlPlane)
	if err != nil {
		c.logger.Error("Failed to get local configs for sync diff",
			slog.Any("error", err),
		)
		return
	}

	// 3. Compute diff
	diff := computeSyncDiff(remoteDeployments, localConfigs)

	c.logger.Info("Computed sync diff",
		slog.Int("to_fetch", len(diff.toFetch)),
		slog.Int("to_update_status", len(diff.toUpdateStatus)),
		slog.Int("to_delete", len(diff.toDelete)),
	)

	// 4. Process deletions for orphaned artifacts first (reverse dependency order: REST APIs → proxies → providers)
	//    Deleting before fetching frees resources and avoids transient route conflicts.
	//    Before deleting, check with the platform which artifacts still exist — artifacts
	//    that exist but have no deployment (e.g., deployment was deleted) should be retained
	//    to preserve their API keys and subscriptions on the gateway.
	if len(diff.toDelete) > 0 {
		// Check artifact existence only for non-on-prem control planes.
		// On-prem CP does not support the /artifacts/exists endpoint.
		if c.apiUtilsService != nil && !c.isOnPrem() {
			existingIDs, err := c.apiUtilsService.CheckArtifactsExist(diff.toDelete)
			if err != nil {
				c.logger.Warn("Failed to check artifact existence, skipping deletions to avoid data loss",
					slog.Any("error", err),
					slog.Int("orphan_count", len(diff.toDelete)),
				)
				// Clear toDelete so no orphans are removed — a transient error
				// (network, CP restart) must not cause destructive deletion of
				// artifacts that may still exist on the platform.
				diff.toDelete = nil
			} else if len(existingIDs) > 0 {
				// Filter out artifacts that still exist on the platform
				existingSet := make(map[string]struct{}, len(existingIDs))
				for _, id := range existingIDs {
					existingSet[id] = struct{}{}
				}
				var actualOrphans []string
				for _, id := range diff.toDelete {
					if _, exists := existingSet[id]; !exists {
						actualOrphans = append(actualOrphans, id)
					} else {
						c.logger.Info("Retaining artifact (exists on platform but has no deployment)",
							slog.String("artifact_id", id),
						)
					}
				}
				diff.toDelete = actualOrphans
			}
		}

		if len(diff.toDelete) > 0 {
			c.processSyncDeletions(diff.toDelete, gatewayID)
		}
	}

	// 5. Process fetches in chunked batches (dependency order: providers → proxies → REST APIs)
	if len(diff.toFetch) > 0 {
		c.processSyncFetches(diff.toFetch, gatewayID)
	}

	// 6. Process status-only updates
	if len(diff.toUpdateStatus) > 0 {
		c.processSyncStatusUpdates(diff.toUpdateStatus, gatewayID)
	}

	c.logger.Info("Deployment sync completed",
		slog.String("gateway_id", gatewayID),
	)
}

// computeSyncDiff compares remote deployments from the platform-API against
// local configs from the database and categorises them into fetch, status-update,
// and delete buckets.
func computeSyncDiff(remote []models.ControlPlaneDeployment, local []*models.StoredConfig) syncDiffResult {
	// Build a map of local configs by artifact ID (UUID) for O(1) lookup.
	// Caller is expected to pass only control-plane-originated configs
	// (e.g. via GetAllConfigsByOrigin).
	localMap := make(map[string]*models.StoredConfig, len(local))
	for _, cfg := range local {
		localMap[cfg.UUID] = cfg
	}

	var result syncDiffResult

	// Track which remote artifact IDs we've seen (for orphan detection)
	remoteIDs := make(map[string]struct{}, len(remote))

	for _, dep := range remote {
		remoteIDs[dep.ArtifactID] = struct{}{}

		localCfg, exists := localMap[dep.ArtifactID]
		if !exists {
			// Not in local DB — only fetch if the remote state is deployed.
			// There is no point creating an artifact locally when the control
			// plane already considers it undeployed (e.g. fresh gateway DB).
			if remoteState, valid := models.ParseDesiredState(dep.State); valid && remoteState == models.StateDeployed {
				result.toFetch = append(result.toFetch, dep)
			}
			continue
		}

		// 1. Deployment ID mismatch — entirely different deployment version,
		//    need to fetch new content regardless of state.
		if localCfg.DeploymentID != dep.DeploymentID {
			result.toFetch = append(result.toFetch, dep)
			continue
		}

		// 2. Same deployment ID but state differs (either direction):
		//    remote undeployed / local deployed, or remote deployed / local undeployed.
		remoteState, valid := models.ParseDesiredState(dep.State)
		if !valid {
			// Unrecognised state — skip rather than persisting an invalid value.
			continue
		}
		if remoteState != localCfg.DesiredState {
			result.toUpdateStatus = append(result.toUpdateStatus, dep)
			continue
		}

		// 3. Same deployment ID, same state, but deployed_at is different or
		//    null locally — re-fetch to ensure consistency with platform-API.
		if localCfg.DeployedAt == nil || !dep.DeployedAt.Equal(*localCfg.DeployedAt) {
			result.toFetch = append(result.toFetch, dep)
			continue
		}
	}

	// Find orphans: local control-plane configs not in remote list
	for id := range localMap {
		if _, exists := remoteIDs[id]; !exists {
			result.toDelete = append(result.toDelete, id)
		}
	}

	return result
}

// processSyncFetches fetches deployment artifacts in chunked batches, ordered by
// dependency: LLM Providers first, then LLM Proxies, then REST APIs.
func (c *Client) processSyncFetches(deployments []models.ControlPlaneDeployment, gatewayID string) {
	// Sort by dependency order: providers → proxies → REST APIs/MCP proxies
	var providers, proxies, restAPIs, mcpProxies []models.ControlPlaneDeployment
	for _, dep := range deployments {
		switch dep.Kind {
		case models.KindLlmProvider:
			providers = append(providers, dep)
		case models.KindLlmProxy:
			proxies = append(proxies, dep)
		case models.KindRestApi:
			restAPIs = append(restAPIs, dep)
		case models.KindMcp:
			mcpProxies = append(mcpProxies, dep)
		}
	}

	// Process in dependency order
	ordered := make([]models.ControlPlaneDeployment, 0, len(deployments))
	ordered = append(ordered, providers...)
	ordered = append(ordered, proxies...)
	ordered = append(ordered, restAPIs...)
	ordered = append(ordered, mcpProxies...)

	batchSize := c.config.SyncBatchSize
	if batchSize <= 0 {
		batchSize = 50
	}

	for i := 0; i < len(ordered); i += batchSize {
		end := i + batchSize
		if end > len(ordered) {
			end = len(ordered)
		}
		batch := ordered[i:end]
		c.processSyncFetchBatch(batch, gatewayID)
	}
}

// processSyncFetchBatch fetches and processes a single batch of deployment artifacts.
func (c *Client) processSyncFetchBatch(batch []models.ControlPlaneDeployment, gatewayID string) {
	// Collect deployment IDs for batch fetch
	deploymentIDs := make([]string, len(batch))
	depMap := make(map[string]models.ControlPlaneDeployment, len(batch))
	for i, dep := range batch {
		deploymentIDs[i] = dep.DeploymentID
		depMap[dep.DeploymentID] = dep
	}

	c.logger.Info("Fetching sync batch",
		slog.Int("batch_size", len(batch)),
	)

	// Batch fetch zip from platform-API
	zipData, err := c.apiUtilsService.BatchFetchDeployments(deploymentIDs)
	if err != nil {
		c.logger.Error("Failed to batch fetch deployments during sync",
			slog.Any("error", err),
			slog.Int("batch_size", len(batch)),
		)
		return
	}

	// Extract YAML content from zip
	yamlMap, err := c.apiUtilsService.ExtractDeploymentsFromBatchZip(zipData)
	if err != nil {
		c.logger.Error("Failed to extract deployments from batch zip during sync",
			slog.Any("error", err),
		)
		return
	}

	// Process each deployment in the batch
	for _, dep := range batch {
		yamlData, ok := yamlMap[dep.DeploymentID]
		if !ok {
			c.logger.Warn("Deployment not found in batch zip response",
				slog.String("deployment_id", dep.DeploymentID),
				slog.String("artifact_id", dep.ArtifactID),
			)
			continue
		}

		correlationID := syncCorrelationID(dep)
		deployedAt := dep.DeployedAt

		switch dep.Kind {
		case models.KindLlmProvider:
			if c.llmDeploymentService == nil {
				c.logger.Warn("Skipping LLM provider sync: llmDeploymentService is nil",
					slog.String("artifact_id", dep.ArtifactID),
				)
				continue
			}
			_, err = c.apiUtilsService.CreateLLMProviderFromYAML(yamlData, dep.ArtifactID,
				dep.DeploymentID, &deployedAt, correlationID, c.llmDeploymentService)

		case models.KindLlmProxy:
			if c.llmDeploymentService == nil {
				c.logger.Warn("Skipping LLM proxy sync: llmDeploymentService is nil",
					slog.String("artifact_id", dep.ArtifactID),
				)
				continue
			}
			_, err = c.apiUtilsService.CreateLLMProxyFromYAML(yamlData, dep.ArtifactID,
				dep.DeploymentID, &deployedAt, correlationID, c.llmDeploymentService)

		case models.KindRestApi:
			_, err = c.apiUtilsService.CreateAPIFromYAML(yamlData, dep.ArtifactID,
				dep.DeploymentID, &deployedAt, correlationID, c.deploymentService)

		case models.KindMcp:
			if c.mcpDeploymentService == nil {
				c.logger.Warn("Skipping MCP proxy sync: mcpDeploymentService is nil",
					slog.String("artifact_id", dep.ArtifactID),
				)
				continue
			}
			_, err = c.apiUtilsService.CreateMCPProxyFromYAML(yamlData, dep.ArtifactID,
				dep.DeploymentID, &deployedAt, correlationID, c.mcpDeploymentService)
		}

		if err != nil {
			c.logger.Error("Failed to process deployment during sync",
				slog.String("artifact_id", dep.ArtifactID),
				slog.String("deployment_id", dep.DeploymentID),
				slog.String("kind", dep.Kind),
				slog.Any("error", err),
			)
			continue
		}

		c.logger.Info("Successfully synced deployment",
			slog.String("artifact_id", dep.ArtifactID),
			slog.String("kind", dep.Kind),
			slog.String("correlation_id", correlationID),
		)
	}
}

// processSyncStatusUpdates handles deployments where only the desired state has
// changed (e.g. undeploy or redeploy) while the deployment ID remains the same.
func (c *Client) processSyncStatusUpdates(deployments []models.ControlPlaneDeployment, gatewayID string) {
	for _, dep := range deployments {
		correlationID := syncCorrelationID(dep)

		cfg, err := c.db.GetConfig(dep.ArtifactID)
		if err != nil {
			c.logger.Error("Failed to get config for sync status update",
				slog.String("artifact_id", dep.ArtifactID),
				slog.Any("error", err),
			)
			continue
		}

		// Map remote state to local desired state
		desiredState, ok := models.ParseDesiredState(dep.State)
		if !ok {
			c.logger.Error("Skipping sync status update: unrecognised remote state",
				slog.String("artifact_id", dep.ArtifactID),
				slog.String("remote_state", dep.State),
			)
			continue
		}
		deployedAt := dep.DeployedAt
		cfg.DesiredState = desiredState
		cfg.DeploymentID = dep.DeploymentID
		cfg.DeployedAt = &deployedAt
		cfg.UpdatedAt = time.Now()

		affected, err := c.db.UpsertConfig(cfg)
		if err != nil {
			c.logger.Error("Failed to upsert config for sync status update",
				slog.String("artifact_id", dep.ArtifactID),
				slog.Any("error", err),
			)
			continue
		}

		if !affected {
			c.logger.Debug("Skipped stale sync status update (newer version in DB)",
				slog.String("artifact_id", dep.ArtifactID),
			)
			continue
		}

		// Publish event or update in-memory store
		if c.eventHub != nil {
			evtType := syncEventType(cfg.Kind)
			evt := eventhub.Event{
				EventType: evtType,
				Action:    "UPDATE",
				EntityID:  dep.ArtifactID,
				EventID:   correlationID,
			}
			if err := c.eventHub.PublishEvent(c.gatewayID, evt); err != nil {
				c.logger.Error("Failed to publish sync status update event",
					slog.String("artifact_id", dep.ArtifactID),
					slog.Any("error", err),
				)
			}
		} else {
			if err := c.store.Update(cfg); err != nil {
				c.logger.Error("Failed to update config in memory store during sync",
					slog.String("artifact_id", dep.ArtifactID),
					slog.Any("error", err),
				)
				continue
			}
			c.updateXDSSnapshotAsync(dep.ArtifactID, correlationID, false, true)
		}

		c.logger.Info("Synced status update",
			slog.String("artifact_id", dep.ArtifactID),
			slog.String("new_state", dep.State),
			slog.String("correlation_id", correlationID),
		)
	}
}

// processSyncDeletions removes orphaned artifacts that exist locally but are
// no longer present in the remote deployment list. Processes in reverse
// dependency order: REST APIs first, then LLM Proxies, then LLM Providers.
func (c *Client) processSyncDeletions(artifactIDs []string, gatewayID string) {
	// Look up configs to determine kind for ordering
	type deletionEntry struct {
		id   string
		kind string
	}

	var restAPIs, proxies, providers, mcpProxies, unknown []deletionEntry

	for _, id := range artifactIDs {
		cfg, err := c.db.GetConfig(id)
		if err != nil {
			if storage.IsNotFoundError(err) {
				continue // Already deleted
			}
			c.logger.Error("Failed to get config for sync deletion",
				slog.String("artifact_id", id),
				slog.Any("error", err),
			)
			continue
		}

		entry := deletionEntry{id: id, kind: cfg.Kind}
		switch cfg.Kind {
		case models.KindLlmProvider:
			providers = append(providers, entry)
		case models.KindLlmProxy:
			proxies = append(proxies, entry)
		case models.KindRestApi:
			restAPIs = append(restAPIs, entry)
		case models.KindMcp:
			mcpProxies = append(mcpProxies, entry)
		}
	}

	// Reverse dependency order: MCP proxies/REST APIs → proxies → providers
	ordered := make([]deletionEntry, 0, len(artifactIDs))
	ordered = append(ordered, mcpProxies...)
	ordered = append(ordered, restAPIs...)
	ordered = append(ordered, unknown...)
	ordered = append(ordered, proxies...)
	ordered = append(ordered, providers...)

	for _, entry := range ordered {
		c.processSyncDeletion(entry.id, entry.kind, gatewayID)
	}
}

// processSyncDeletion deletes a single orphaned artifact during sync.
func (c *Client) processSyncDeletion(artifactID, kind, gatewayID string) {
	correlationID := utils.GenerateDeterministicUUIDv7(artifactID, time.Now())

	c.logger.Info("Deleting orphaned artifact during sync",
		slog.String("artifact_id", artifactID),
		slog.String("kind", kind),
	)

	switch kind {
	case models.KindLlmProvider:
		if c.llmDeploymentService != nil {
			cfg, err := c.findAPIConfig(artifactID)
			if err != nil {
				c.logger.Error("Failed to find LLM provider config for sync deletion",
					slog.String("artifact_id", artifactID),
					slog.Any("error", err),
				)
				return
			}
			_, err = c.llmDeploymentService.DeleteLLMProvider(cfg.Handle, correlationID, c.logger)
			if err != nil {
				c.logger.Error("Failed to delete LLM provider during sync",
					slog.String("artifact_id", artifactID),
					slog.Any("error", err),
				)
				return
			}
		}

	case models.KindLlmProxy:
		if c.llmDeploymentService != nil {
			cfg, err := c.findAPIConfig(artifactID)
			if err != nil {
				c.logger.Error("Failed to find LLM proxy config for sync deletion",
					slog.String("artifact_id", artifactID),
					slog.Any("error", err),
				)
				return
			}
			_, err = c.llmDeploymentService.DeleteLLMProxy(cfg.Handle, correlationID, c.logger)
			if err != nil {
				c.logger.Error("Failed to delete LLM proxy during sync",
					slog.String("artifact_id", artifactID),
					slog.Any("error", err),
				)
				return
			}
		}

	case models.KindRestApi:
		// REST API / WebSub — follow the performFullAPIDeletion pattern
		apiConfig, err := c.findAPIConfig(artifactID)
		if err != nil {
			if storage.IsNotFoundError(err) {
				c.cleanupOrphanedResources(artifactID, correlationID)
				return
			}
			c.logger.Error("Failed to find API config for sync deletion",
				slog.String("artifact_id", artifactID),
				slog.Any("error", err),
			)
			return
		}
		c.performFullAPIDeletion(artifactID, apiConfig, correlationID)

	case models.KindMcp:
		if c.mcpDeploymentService != nil {
			cfg, err := c.findAPIConfig(artifactID)
			if err != nil {
				c.logger.Error("Failed to find MCP proxy config for sync deletion",
					slog.String("artifact_id", artifactID),
					slog.Any("error", err),
				)
				return
			}
			_, err = c.mcpDeploymentService.DeleteMCPProxy(cfg.Handle, correlationID, c.logger)
			if err != nil {
				c.logger.Error("Failed to delete MCP proxy during sync",
					slog.String("artifact_id", artifactID),
					slog.Any("error", err),
				)
				return
			}
		}
	}

	c.logger.Info("Successfully deleted orphaned artifact during sync",
		slog.String("artifact_id", artifactID),
		slog.String("kind", kind),
	)
}

// syncCorrelationID returns the correlation ID for a sync deployment.
// Uses the Etag if provided, otherwise generates a deterministic UUID
// from the artifact ID and deployed-at timestamp.
func syncCorrelationID(dep models.ControlPlaneDeployment) string {
	if dep.Etag != "" {
		return dep.Etag
	}
	return utils.GenerateDeterministicUUIDv7(dep.ArtifactID, dep.DeployedAt)
}

// parseCPSyncInfo extracts the APIM API ID and revision ID from a CPSyncInfo JSON string.
// Returns empty strings if the reason is empty, not valid JSON, or missing fields.
func parseCPSyncInfo(cpSyncInfo string) (apiID, revisionID string) {
	if cpSyncInfo == "" {
		return "", ""
	}
	var data map[string]string
	if err := json.Unmarshal([]byte(cpSyncInfo), &data); err != nil {
		return "", ""
	}
	return data["id"], data["revision"]
}

// SyncBottomUpAPIs pushes all pending gateway-created APIs to the on-prem control plane.
// It is called on connect/reconnect (when IsOnPrem() is true) and immediately after a
// gateway-initiated create/update/undeploy when the gateway is already connected.
//
// For each API with cp_sync_status IN ('pending', 'failed'):
//   - Calls PushAPIDeployment (with initiatedFromGateway=true) up to maxRetries times.
//   - On success: records cp_sync_status=success and stores any returned deploymentId.
//   - On all retries exhausted: records cp_sync_status=failed with the error reason.
func (c *Client) SyncArtifactsToOnPremAPIM(apimConfig *utils.APIMConfig) error {
	if !c.isOnPrem() {
		return fmt.Errorf("bottom-up API sync skipped: on-prem control plane mode is not enabled")
	}

	if c.db == nil {
		return fmt.Errorf("cannot run bottom-up sync: database is not available")
	}

	if apimConfig == nil {
		return fmt.Errorf("cannot run bottom-up sync: APIM configuration is nil")
	}

	apis, err := c.db.GetPendingBottomUpAPIs()
	if err != nil {
		c.logger.Error("Bottom-up sync: failed to query pending APIs", slog.Any("error", err))
		return fmt.Errorf("failed to query pending bottom-up APIs: %w", err)
	}

	if len(apis) == 0 {
		c.logger.Info("Bottom-up sync: no pending APIs to process")
		return nil
	}

	c.logger.Info("Bottom-up sync: starting", slog.Int("pending", len(apis)))

	const maxRetries = 3

	for _, api := range apis {
		var lastErr error

		// --- Undeploy flow ---
		if api.DesiredState == models.StateUndeployed {
			apimAPIID, revisionID := parseCPSyncInfo(api.CPSyncInfo)

			if apimAPIID == "" {
				// Never synced to APIM — nothing to undeploy
				c.logger.Info("Bottom-up sync: API was never synced to APIM, skipping undeploy",
					slog.String("uuid", api.UUID),
					slog.String("display_name", api.DisplayName),
				)
				if dbErr := c.db.UpdateCPSyncStatus(api.UUID, "", models.CPSyncStatusSuccess, ""); dbErr != nil {
					c.logger.Error("Bottom-up sync: failed to record success status",
						slog.String("uuid", api.UUID),
						slog.String("display_name", api.DisplayName),
						slog.Any("error", dbErr),
					)
				}
				continue
			}

			for attempt := 1; attempt <= maxRetries; attempt++ {
				lastErr = utils.UndeployRevisionFromAPIM(*apimConfig, apimAPIID, revisionID, c.logger)
				if lastErr == nil {
					break
				}
				c.logger.Warn("Bottom-up sync: undeploy attempt failed",
					slog.String("uuid", api.UUID),
					slog.String("display_name", api.DisplayName),
					slog.String("apim_api_id", apimAPIID),
					slog.Int("attempt", attempt),
					slog.Any("error", lastErr),
				)
			}

			if lastErr != nil {
				c.logger.Error("Bottom-up sync: undeploy all retries exhausted",
					slog.String("uuid", api.UUID),
					slog.String("display_name", api.DisplayName),
					slog.Any("error", lastErr),
				)
				if dbErr := c.db.UpdateCPSyncStatus(api.UUID, "", models.CPSyncStatusFailed, lastErr.Error()); dbErr != nil {
					c.logger.Error("Bottom-up sync: failed to record failed status",
						slog.String("uuid", api.UUID),
						slog.String("display_name", api.DisplayName),
						slog.Any("error", dbErr),
					)
				}
				continue
			}

			// Success — clear cp_sync_reason
			if dbErr := c.db.UpdateCPSyncStatus(api.UUID, "", models.CPSyncStatusSuccess, ""); dbErr != nil {
				c.logger.Error("Bottom-up sync: failed to record success status",
					slog.String("uuid", api.UUID),
					slog.String("display_name", api.DisplayName),
					slog.Any("error", dbErr),
				)
			}
			c.logger.Info("Bottom-up sync: API undeployed successfully",
				slog.String("uuid", api.UUID),
				slog.String("display_name", api.DisplayName),
			)
			continue
		}

		// --- Deploy / update flow ---

		// For updates (previously synced APIs), fetch swagger from APIM instead of generating locally.
		// CPSyncInfo contains {"id": "<apim-api-id>", "revision": "..."} from the last successful sync.
		swaggerOverride := ""
		if apimAPIID, _ := parseCPSyncInfo(api.CPSyncInfo); apimAPIID != "" {
			swagger, err := utils.FetchSwaggerFromAPIM(*apimConfig, apimAPIID, c.logger)
			if err != nil {
				c.logger.Warn("Bottom-up sync: failed to fetch swagger from APIM, falling back to local generation",
					slog.String("uuid", api.UUID),
					slog.String("apim_api_id", apimAPIID),
					slog.Any("error", err),
				)
			} else {
				swaggerOverride = swagger
			}
		}

		// Export the API as a zip
		zipBuffer, err := utils.ExportAPIAsZip(api, apimConfig.GatewayName, swaggerOverride)
		if err != nil {
			c.logger.Error("Bottom-up sync: failed to export API as zip",
				slog.String("uuid", api.UUID),
				slog.Any("error", err),
			)
			if dbErr := c.db.UpdateCPSyncStatus(api.UUID, "", models.CPSyncStatusFailed, err.Error()); dbErr != nil {
				c.logger.Error("Bottom-up sync: failed to record failed status",
					slog.String("uuid", api.UUID),
					slog.Any("error", dbErr),
				)
			}
			continue
		}

		// Store the exported bytes to reuse in retries
		zipBytes := zipBuffer.Bytes()

		var importResp *utils.OnPremAPIMImportResponse

		for attempt := 1; attempt <= maxRetries; attempt++ {
			currentBuffer := bytes.NewBuffer(zipBytes)

			importResp, lastErr = utils.ImportAPIToAPIMWithConfig(
				*apimConfig, c.logger, api.UUID+".zip", currentBuffer,
			)
			if lastErr == nil {
				break
			}
			c.logger.Warn("Bottom-up sync: attempt failed",
				slog.String("uuid", api.UUID),
				slog.String("desired_state", string(api.DesiredState)),
				slog.Int("attempt", attempt),
				slog.Any("error", lastErr),
			)
		}

		if lastErr != nil {
			c.logger.Error("Bottom-up sync: all retries exhausted",
				slog.String("uuid", api.UUID),
				slog.String("display_name", api.DisplayName),
				slog.String("desired_state", string(api.DesiredState)),
				slog.Any("error", lastErr),
			)
			if dbErr := c.db.UpdateCPSyncStatus(api.UUID, "", models.CPSyncStatusFailed, lastErr.Error()); dbErr != nil {
				c.logger.Error("Bottom-up sync: failed to record failed status",
					slog.String("uuid", api.UUID),
					slog.String("display_name", api.DisplayName),
					slog.Any("error", dbErr),
				)
			}
			continue
		}

		// Success — persist sync outcome with import response data
		importReasonJSON := "{}"
		if importResp != nil {
			reasonData := map[string]string{
				"id":       importResp.ID,
				"revision": importResp.Revision,
			}
			if reasonBytes, err := json.Marshal(reasonData); err == nil {
				importReasonJSON = string(reasonBytes)
			}
		}

		var cpArtifactID string
		if importResp != nil {
			cpArtifactID = importResp.ID
		}
		if dbErr := c.db.UpdateCPSyncStatus(api.UUID, cpArtifactID, models.CPSyncStatusSuccess, importReasonJSON); dbErr != nil {
			c.logger.Error("Bottom-up sync: failed to record success status",
				slog.String("uuid", api.UUID),
				slog.String("display_name", api.DisplayName),
				slog.Any("error", dbErr),
			)
		}

		c.logger.Info("Bottom-up sync: API synced successfully",
			slog.String("uuid", api.UUID),
			slog.String("display_name", api.DisplayName),
			slog.String("desired_state", string(api.DesiredState)),
			slog.String("api_id", importResp.ID),
			slog.String("revision", importResp.Revision),
		)
	}

	c.logger.Info("Bottom-up API sync completed successfully", slog.Int("processed", len(apis)))
	return nil
}

// syncEventType maps an artifact kind to the corresponding eventhub.EventType.
func syncEventType(kind string) eventhub.EventType {
	switch kind {
	case models.KindLlmProvider:
		return eventhub.EventTypeLLMProvider
	case models.KindLlmProxy:
		return eventhub.EventTypeLLMProxy
	case models.KindMcp:
		return eventhub.EventTypeMCPProxy
	default:
		return eventhub.EventTypeAPI
	}
}
