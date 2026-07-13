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

package controlplanehooks

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/controlplane"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// Hooks implements controlplane.ControlPlaneEventGatewayHooks, supplying
// WebSub/WebBroker control-plane WebSocket event handling (deploy/undeploy/
// delete and HMAC secret sync). Built entirely on top of *controlplane.Client's
// exported accessors — see gateway/gateway-controller/pkg/controlplane/eventgateway_hooks.go.
type Hooks struct{}

// HandleWebSubAPIDeployed handles websub.deployed events.
func (Hooks) HandleWebSubAPIDeployed(c *controlplane.Client, event map[string]any) {
	logger := c.Logger()
	logger.Debug("WebSub API Deployment Event",
		slog.Any("payload", event["payload"]),
		slog.Any("timestamp", event["timestamp"]),
		slog.Any("correlationId", event["correlationId"]),
	)

	eventBytes, err := json.Marshal(event)
	if err != nil {
		logger.Error("Failed to marshal WebSub API deployment event for parsing", slog.Any("error", err))
		return
	}

	var deployedEvent WebSubAPIDeployedEvent
	if err := json.Unmarshal(eventBytes, &deployedEvent); err != nil {
		logger.Error("Failed to parse WebSub API deployment event", slog.Any("error", err))
		return
	}

	apiID := deployedEvent.Payload.APIID
	if apiID == "" {
		logger.Error("API ID is empty in WebSub API deployment event")
		return
	}

	logger.Info("Processing WebSub API deployment",
		slog.String("api_id", apiID),
		slog.String("deployment_id", deployedEvent.Payload.DeploymentID),
		slog.String("correlation_id", deployedEvent.CorrelationID),
	)

	// Fetch WebSub API definition from control plane
	zipData, err := c.APIUtilsService().FetchResourceZip("/websub-apis/"+apiID, "WebSub API definition")
	if err != nil {
		logger.Error("Failed to fetch WebSub API definition",
			slog.String("api_id", apiID),
			slog.Any("error", err),
		)
		c.SendDeploymentAck(deployedEvent.Payload.DeploymentID, apiID, "websub", "deploy", "failed",
			deployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	yamlData, err := c.APIUtilsService().ExtractYAMLFromZip(zipData)
	if err != nil {
		logger.Error("Failed to extract YAML from WebSub API ZIP",
			slog.String("api_id", apiID),
			slog.Any("error", err),
		)
		c.SendDeploymentAck(deployedEvent.Payload.DeploymentID, apiID, "websub", "deploy", "failed",
			deployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	// Ensure any {{ secret "handle" }} references in the YAML are in local
	// storage before rendering.
	c.SyncSecretRefsFromYAML(yamlData, deployedEvent.CorrelationID)

	performedAt := deployedEvent.Payload.PerformedAt.Truncate(time.Millisecond)
	if performedAt.IsZero() {
		performedAt = time.Now().Truncate(time.Millisecond)
	}
	// Reuse the existing local UUID for a bottom-up (DP->CP) synced API so the
	// control-plane deploy is an in-place update.
	result, err := c.APIUtilsService().CreateAPIFromYAML(yamlData, c.ResolveLocalArtifactID(apiID), deployedEvent.Payload.DeploymentID, &performedAt, deployedEvent.CorrelationID, c.DeploymentService())
	if err != nil {
		logger.Error("Failed to create WebSub API from YAML",
			slog.String("api_id", apiID),
			slog.Any("error", err),
		)
		c.SendDeploymentAck(deployedEvent.Payload.DeploymentID, apiID, "websub", "deploy", "failed",
			deployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	if result.IsStale {
		logger.Debug("Skipped stale WebSub API deploy event (newer version exists in DB)",
			slog.String("api_id", apiID),
			slog.String("deployment_id", deployedEvent.Payload.DeploymentID),
		)
		return
	}

	// Load platform-managed HMAC secrets into the webhook secret store.
	if result.StoredConfig != nil {
		syncHmacSecretsForArtifact(c, result.StoredConfig.UUID)
	}

	c.SendDeploymentAck(deployedEvent.Payload.DeploymentID, apiID, "websub", "deploy", "success",
		deployedEvent.Payload.PerformedAt, "")

	logger.Info("Successfully processed WebSub API deployment event",
		slog.String("api_id", apiID),
		slog.String("correlation_id", deployedEvent.CorrelationID),
	)
}

// HandleWebSubAPIUndeployed handles websub.undeployed events.
func (Hooks) HandleWebSubAPIUndeployed(c *controlplane.Client, event map[string]any) {
	logger := c.Logger()
	logger.Debug("WebSub API Undeployment Event",
		slog.Any("payload", event["payload"]),
		slog.Any("timestamp", event["timestamp"]),
		slog.Any("correlationId", event["correlationId"]),
	)

	eventBytes, err := json.Marshal(event)
	if err != nil {
		logger.Error("Failed to marshal WebSub API undeployment event for parsing", slog.Any("error", err))
		return
	}

	var undeployedEvent WebSubAPIUndeployedEvent
	if err := json.Unmarshal(eventBytes, &undeployedEvent); err != nil {
		logger.Error("Failed to parse WebSub API undeployment event", slog.Any("error", err))
		return
	}

	apiID := undeployedEvent.Payload.APIID
	if apiID == "" {
		logger.Error("API ID is empty in WebSub API undeployment event")
		return
	}

	apiConfig, err := c.FindAPIConfig(apiID)
	if err != nil {
		if storage.IsNotFoundError(err) {
			logger.Warn("WebSub API configuration not found for undeployment",
				slog.String("api_id", apiID),
			)
			c.SendDeploymentAck(undeployedEvent.Payload.DeploymentID, apiID, "websub", "undeploy", "success",
				undeployedEvent.Payload.PerformedAt, "")
			return
		}
		logger.Error("Failed to fetch WebSub API configuration for undeployment",
			slog.String("api_id", apiID),
			slog.String("correlation_id", undeployedEvent.CorrelationID),
			slog.Any("error", err),
		)
		c.SendDeploymentAck(undeployedEvent.Payload.DeploymentID, apiID, "websub", "undeploy", "failed",
			undeployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	if apiConfig.DeploymentID != "" && undeployedEvent.Payload.DeploymentID != "" &&
		apiConfig.DeploymentID != undeployedEvent.Payload.DeploymentID {
		logger.Warn("Ignoring stale WebSub API undeploy event: deployment ID mismatch",
			slog.String("api_id", apiID),
			slog.String("event_deployment_id", undeployedEvent.Payload.DeploymentID),
			slog.String("current_deployment_id", apiConfig.DeploymentID),
		)
		c.SendDeploymentAck(undeployedEvent.Payload.DeploymentID, apiID, "websub", "undeploy", "failed",
			undeployedEvent.Payload.PerformedAt, "DEPLOYMENT_ID_MISMATCH")
		return
	}

	performedAt := undeployedEvent.Payload.PerformedAt.Truncate(time.Millisecond)
	if performedAt.IsZero() {
		performedAt = time.Now().Truncate(time.Millisecond)
	}
	apiConfig.DesiredState = models.StateUndeployed
	apiConfig.DeploymentID = undeployedEvent.Payload.DeploymentID
	apiConfig.DeployedAt = &performedAt
	apiConfig.UpdatedAt = time.Now()

	affected, err := c.DB().UpsertConfig(apiConfig)
	if err != nil {
		logger.Error("Failed to upsert config for WebSub API undeployment",
			slog.String("api_id", apiID),
			slog.Any("error", err),
		)
		c.SendDeploymentAck(undeployedEvent.Payload.DeploymentID, apiID, "websub", "undeploy", "failed",
			undeployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}
	if !affected {
		logger.Debug("Skipped stale WebSub API undeploy event (newer version exists in DB)",
			slog.String("api_id", apiID),
			slog.String("deployment_id", undeployedEvent.Payload.DeploymentID),
		)
		return
	}

	evt := eventhub.Event{
		EventType: eventhub.EventTypeAPI,
		Action:    "UPDATE",
		EntityID:  apiID,
		EventID:   undeployedEvent.CorrelationID,
	}
	if err := c.EventHub().PublishEvent(c.GatewayID(), evt); err != nil {
		logger.Error("Failed to publish WebSub API undeployment event", slog.Any("error", err))
	}

	c.SendDeploymentAck(undeployedEvent.Payload.DeploymentID, apiID, "websub", "undeploy", "success",
		undeployedEvent.Payload.PerformedAt, "")

	logger.Info("Successfully processed WebSub API undeployment event",
		slog.String("api_id", apiID),
		slog.String("correlation_id", undeployedEvent.CorrelationID),
	)
}

// HandleWebSubAPIDeleted handles websub.deleted events.
func (Hooks) HandleWebSubAPIDeleted(c *controlplane.Client, event map[string]any) {
	logger := c.Logger()
	logger.Debug("WebSub API Deleted Event",
		slog.Any("payload", event["payload"]),
		slog.Any("timestamp", event["timestamp"]),
		slog.Any("correlationId", event["correlationId"]),
	)

	eventBytes, err := json.Marshal(event)
	if err != nil {
		logger.Error("Failed to marshal WebSub API deleted event for parsing", slog.Any("error", err))
		return
	}

	var deletedEvent WebSubAPIDeletedEvent
	if err := json.Unmarshal(eventBytes, &deletedEvent); err != nil {
		logger.Error("Failed to parse WebSub API deleted event", slog.Any("error", err))
		return
	}

	apiID := deletedEvent.Payload.APIID
	if apiID == "" {
		logger.Error("API ID is empty in WebSub API deleted event")
		return
	}

	apiConfig, err := c.FindAPIConfig(apiID)
	if err != nil {
		if storage.IsNotFoundError(err) {
			logger.Warn("WebSub API configuration not found for deletion",
				slog.String("api_id", apiID),
			)
			cleanupHmacSecretsForArtifact(c, apiID)
			return
		}
		logger.Error("Failed to fetch WebSub API configuration for deletion",
			slog.String("api_id", apiID),
			slog.String("correlation_id", deletedEvent.CorrelationID),
			slog.Any("error", err),
		)
		return
	}

	c.PerformFullAPIDeletion(apiID, apiConfig, deletedEvent.CorrelationID)
	cleanupHmacSecretsForArtifact(c, apiConfig.UUID)
}

// HandleWebBrokerAPIDeployed handles webbroker.deployed events.
func (Hooks) HandleWebBrokerAPIDeployed(c *controlplane.Client, event map[string]any) {
	logger := c.Logger()
	logger.Debug("WebBroker API Deployment Event",
		slog.Any("payload", event["payload"]),
		slog.Any("timestamp", event["timestamp"]),
		slog.Any("correlationId", event["correlationId"]),
	)

	eventBytes, err := json.Marshal(event)
	if err != nil {
		logger.Error("Failed to marshal WebBroker API deployment event for parsing", slog.Any("error", err))
		return
	}

	var deployedEvent WebBrokerAPIDeployedEvent
	if err := json.Unmarshal(eventBytes, &deployedEvent); err != nil {
		logger.Error("Failed to parse WebBroker API deployment event", slog.Any("error", err))
		return
	}

	apiID := deployedEvent.Payload.APIID
	if apiID == "" {
		logger.Error("API ID is empty in WebBroker API deployment event")
		return
	}

	logger.Info("Processing WebBroker API deployment",
		slog.String("api_id", apiID),
		slog.String("deployment_id", deployedEvent.Payload.DeploymentID),
		slog.String("correlation_id", deployedEvent.CorrelationID),
	)

	// Fetch WebBroker API definition from control plane
	zipData, err := c.APIUtilsService().FetchResourceZip("/webbroker-apis/"+apiID, "WebBroker API definition")
	if err != nil {
		logger.Error("Failed to fetch WebBroker API definition",
			slog.String("api_id", apiID),
			slog.Any("error", err),
		)
		c.SendDeploymentAck(deployedEvent.Payload.DeploymentID, apiID, "webbroker", "deploy", "failed",
			deployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	yamlData, err := c.APIUtilsService().ExtractYAMLFromZip(zipData)
	if err != nil {
		logger.Error("Failed to extract YAML from WebBroker API ZIP",
			slog.String("api_id", apiID),
			slog.Any("error", err),
		)
		c.SendDeploymentAck(deployedEvent.Payload.DeploymentID, apiID, "webbroker", "deploy", "failed",
			deployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	// Ensure any {{ secret "handle" }} references in the YAML are in local
	// storage before rendering.
	c.SyncSecretRefsFromYAML(yamlData, deployedEvent.CorrelationID)

	performedAt := deployedEvent.Payload.PerformedAt.Truncate(time.Millisecond)
	if performedAt.IsZero() {
		performedAt = time.Now().Truncate(time.Millisecond)
	}
	// Reuse the existing local UUID for a bottom-up (DP->CP) synced API so the
	// control-plane deploy is an in-place update.
	result, err := c.APIUtilsService().CreateAPIFromYAML(yamlData, c.ResolveLocalArtifactID(apiID), deployedEvent.Payload.DeploymentID, &performedAt, deployedEvent.CorrelationID, c.DeploymentService())
	if err != nil {
		logger.Error("Failed to create WebBroker API from YAML",
			slog.String("api_id", apiID),
			slog.Any("error", err),
		)
		c.SendDeploymentAck(deployedEvent.Payload.DeploymentID, apiID, "webbroker", "deploy", "failed",
			deployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	if result.IsStale {
		logger.Debug("Skipped stale WebBroker API deploy event (newer version exists in DB)",
			slog.String("api_id", apiID),
			slog.String("deployment_id", deployedEvent.Payload.DeploymentID),
		)
		return
	}

	c.SendDeploymentAck(deployedEvent.Payload.DeploymentID, apiID, "webbroker", "deploy", "success",
		deployedEvent.Payload.PerformedAt, "")

	logger.Info("Successfully processed WebBroker API deployment event",
		slog.String("api_id", apiID),
		slog.String("correlation_id", deployedEvent.CorrelationID),
	)
}

// HandleWebBrokerAPIUndeployed handles webbroker.undeployed events.
func (Hooks) HandleWebBrokerAPIUndeployed(c *controlplane.Client, event map[string]any) {
	logger := c.Logger()
	logger.Debug("WebBroker API Undeployment Event",
		slog.Any("payload", event["payload"]),
		slog.Any("timestamp", event["timestamp"]),
		slog.Any("correlationId", event["correlationId"]),
	)

	eventBytes, err := json.Marshal(event)
	if err != nil {
		logger.Error("Failed to marshal WebBroker API undeployment event for parsing", slog.Any("error", err))
		return
	}

	var undeployedEvent WebBrokerAPIUndeployedEvent
	if err := json.Unmarshal(eventBytes, &undeployedEvent); err != nil {
		logger.Error("Failed to parse WebBroker API undeployment event", slog.Any("error", err))
		return
	}

	apiID := undeployedEvent.Payload.APIID
	if apiID == "" {
		logger.Error("API ID is empty in WebBroker API undeployment event")
		return
	}

	apiConfig, err := c.FindAPIConfig(apiID)
	if err != nil {
		if storage.IsNotFoundError(err) {
			logger.Warn("WebBroker API configuration not found for undeployment",
				slog.String("api_id", apiID),
			)
			c.SendDeploymentAck(undeployedEvent.Payload.DeploymentID, apiID, "webbroker", "undeploy", "success",
				undeployedEvent.Payload.PerformedAt, "")
			return
		}
		logger.Error("Failed to fetch WebBroker API configuration for undeployment",
			slog.String("api_id", apiID),
			slog.String("correlation_id", undeployedEvent.CorrelationID),
			slog.Any("error", err),
		)
		c.SendDeploymentAck(undeployedEvent.Payload.DeploymentID, apiID, "webbroker", "undeploy", "failed",
			undeployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	if apiConfig.DeploymentID != "" && undeployedEvent.Payload.DeploymentID != "" &&
		apiConfig.DeploymentID != undeployedEvent.Payload.DeploymentID {
		logger.Warn("Ignoring stale WebBroker API undeploy event: deployment ID mismatch",
			slog.String("api_id", apiID),
			slog.String("event_deployment_id", undeployedEvent.Payload.DeploymentID),
			slog.String("current_deployment_id", apiConfig.DeploymentID),
		)
		c.SendDeploymentAck(undeployedEvent.Payload.DeploymentID, apiID, "webbroker", "undeploy", "failed",
			undeployedEvent.Payload.PerformedAt, "DEPLOYMENT_ID_MISMATCH")
		return
	}

	performedAt := undeployedEvent.Payload.PerformedAt.Truncate(time.Millisecond)
	if performedAt.IsZero() {
		performedAt = time.Now().Truncate(time.Millisecond)
	}
	apiConfig.DesiredState = models.StateUndeployed
	apiConfig.DeploymentID = undeployedEvent.Payload.DeploymentID
	apiConfig.DeployedAt = &performedAt
	apiConfig.UpdatedAt = time.Now()

	affected, err := c.DB().UpsertConfig(apiConfig)
	if err != nil {
		logger.Error("Failed to upsert config for WebBroker API undeployment",
			slog.String("api_id", apiID),
			slog.Any("error", err),
		)
		c.SendDeploymentAck(undeployedEvent.Payload.DeploymentID, apiID, "webbroker", "undeploy", "failed",
			undeployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}
	if !affected {
		logger.Debug("Skipped stale WebBroker API undeploy event (newer version exists in DB)",
			slog.String("api_id", apiID),
			slog.String("deployment_id", undeployedEvent.Payload.DeploymentID),
		)
		return
	}

	evt := eventhub.Event{
		EventType: eventhub.EventTypeAPI,
		Action:    "UPDATE",
		EntityID:  apiID,
		EventID:   undeployedEvent.CorrelationID,
	}
	if err := c.EventHub().PublishEvent(c.GatewayID(), evt); err != nil {
		logger.Error("Failed to publish WebBroker API undeployment event", slog.Any("error", err))
	}

	c.SendDeploymentAck(undeployedEvent.Payload.DeploymentID, apiID, "webbroker", "undeploy", "success",
		undeployedEvent.Payload.PerformedAt, "")

	logger.Info("Successfully processed WebBroker API undeployment event",
		slog.String("api_id", apiID),
		slog.String("correlation_id", undeployedEvent.CorrelationID),
	)
}

// HandleWebBrokerAPIDeleted handles webbroker.deleted events.
func (Hooks) HandleWebBrokerAPIDeleted(c *controlplane.Client, event map[string]any) {
	logger := c.Logger()
	logger.Debug("WebBroker API Deleted Event",
		slog.Any("payload", event["payload"]),
		slog.Any("timestamp", event["timestamp"]),
		slog.Any("correlationId", event["correlationId"]),
	)

	eventBytes, err := json.Marshal(event)
	if err != nil {
		logger.Error("Failed to marshal WebBroker API deleted event for parsing", slog.Any("error", err))
		return
	}

	var deletedEvent WebBrokerAPIDeletedEvent
	if err := json.Unmarshal(eventBytes, &deletedEvent); err != nil {
		logger.Error("Failed to parse WebBroker API deleted event", slog.Any("error", err))
		return
	}

	apiID := deletedEvent.Payload.APIID
	if apiID == "" {
		logger.Error("API ID is empty in WebBroker API deleted event")
		return
	}

	apiConfig, err := c.FindAPIConfig(apiID)
	if err != nil {
		if storage.IsNotFoundError(err) {
			logger.Warn("WebBroker API configuration not found for deletion; running orphan cleanup",
				slog.String("api_id", apiID),
			)
			c.CleanupOrphanedResources(apiID, deletedEvent.CorrelationID)
			return
		}
		logger.Error("Failed to fetch WebBroker API configuration for deletion",
			slog.String("api_id", apiID),
			slog.String("correlation_id", deletedEvent.CorrelationID),
			slog.Any("error", err),
		)
		return
	}

	c.PerformFullAPIDeletion(apiID, apiConfig, deletedEvent.CorrelationID)
}

// HandleWebSubAPIHmacSecretEvent handles websub.hmacsecret.created/updated/deleted
// events from platform-API. It re-syncs all platform-managed HMAC secrets for
// the affected artifact.
func (Hooks) HandleWebSubAPIHmacSecretEvent(c *controlplane.Client, event map[string]any) {
	logger := c.Logger()
	payloadBytes, err := json.Marshal(event["payload"])
	if err != nil {
		logger.Error("Failed to marshal HMAC secret event payload", slog.Any("error", err))
		return
	}
	var payload platformHmacSecretEventPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		logger.Error("Failed to parse HMAC secret event payload", slog.Any("error", err))
		return
	}
	if payload.ArtifactUUID == "" {
		logger.Warn("HMAC secret event missing artifactUuid, skipping")
		return
	}
	logger.Info("Processing platform HMAC secret event",
		slog.Any("type", event["type"]),
		slog.String("artifact_uuid", payload.ArtifactUUID),
		slog.String("secret_name", payload.SecretName),
	)
	syncHmacSecretsForArtifact(c, payload.ArtifactUUID)
}

// syncHmacSecretsForArtifact fetches all platform-managed HMAC secrets for a WebSub API
// artifact from platform-API and loads them into the in-memory webhook secret store.
// It replaces any previously loaded secrets for this artifact atomically (clear then re-add).
func syncHmacSecretsForArtifact(c *controlplane.Client, artifactID string) {
	store := c.WebhookSecretStore()
	if store == nil {
		return
	}
	logger := c.Logger()

	secrets, err := fetchWebSubAPIHmacSecrets(c, artifactID)
	if err != nil {
		logger.Warn("Failed to fetch platform HMAC secrets for WebSub API",
			slog.String("artifact_id", artifactID),
			slog.Any("error", err))
		return
	}

	if err := store.RemoveAllByAPI(artifactID); err != nil {
		logger.Warn("Failed to clear existing HMAC secrets for WebSub API",
			slog.String("artifact_id", artifactID),
			slog.Any("error", err))
		return
	}

	for _, s := range secrets {
		if err := store.Store(artifactID, s.Name, s.Plaintext); err != nil {
			logger.Warn("Failed to store platform HMAC secret in memory",
				slog.String("artifact_id", artifactID),
				slog.String("secret_name", s.Name),
				slog.Any("error", err))
		}
	}

	if err := c.RefreshWebhookSecretSnapshot(); err != nil {
		logger.Warn("Failed to refresh webhook secret xDS snapshot after platform sync",
			slog.String("artifact_id", artifactID),
			slog.Any("error", err))
	}

	logger.Info("Loaded platform HMAC secrets for WebSub API",
		slog.String("artifact_id", artifactID),
		slog.Int("count", len(secrets)))
}

// cleanupHmacSecretsForArtifact removes all in-memory HMAC secrets for an artifact and
// refreshes the xDS snapshot. Called on WebSub API deletion (found and not-found paths).
func cleanupHmacSecretsForArtifact(c *controlplane.Client, artifactID string) {
	store := c.WebhookSecretStore()
	if store == nil {
		return
	}
	logger := c.Logger()
	if err := store.RemoveAllByAPI(artifactID); err != nil {
		logger.Warn("Failed to remove HMAC secrets from store during WebSub API cleanup",
			slog.String("artifact_id", artifactID),
			slog.Any("error", err))
	}
	if err := c.RefreshWebhookSecretSnapshot(); err != nil {
		logger.Warn("Failed to refresh webhook secret xDS snapshot after WebSub API cleanup",
			slog.String("artifact_id", artifactID),
			slog.Any("error", err))
	}
}

// fetchWebSubAPIHmacSecrets fetches the plaintext HMAC secrets for a WebSub API artifact
// from the platform-API internal endpoint.
func fetchWebSubAPIHmacSecrets(c *controlplane.Client, artifactID string) ([]hmacSecretInfo, error) {
	var response platformHmacSecretsResponse
	if err := c.APIUtilsService().FetchResourceJSON("/websub-apis/"+artifactID+"/secrets", "WebSub API HMAC secrets", &response); err != nil {
		return nil, err
	}

	secrets := make([]hmacSecretInfo, 0, len(response.Secrets))
	for _, s := range response.Secrets {
		secrets = append(secrets, hmacSecretInfo{Name: s.Name, Plaintext: s.Secret})
	}
	return secrets, nil
}
