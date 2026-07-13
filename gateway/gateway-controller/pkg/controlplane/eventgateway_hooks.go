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
	"log/slog"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/common/webhooksecret"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// ControlPlaneEventGatewayHooks is the extension point through which an
// external event-gateway-controller binary supplies WebSub/WebBroker
// control-plane WebSocket event handling (deploy/undeploy/delete and HMAC
// secret sync). Core never implements this interface itself; it is only ever
// satisfied by code living outside this module. See
// SetControlPlaneEventGatewayHooks.
type ControlPlaneEventGatewayHooks interface {
	HandleWebSubAPIDeployed(c *Client, event map[string]any)
	HandleWebSubAPIUndeployed(c *Client, event map[string]any)
	HandleWebSubAPIDeleted(c *Client, event map[string]any)
	HandleWebSubAPIHmacSecretEvent(c *Client, event map[string]any)
	HandleWebBrokerAPIDeployed(c *Client, event map[string]any)
	HandleWebBrokerAPIUndeployed(c *Client, event map[string]any)
	HandleWebBrokerAPIDeleted(c *Client, event map[string]any)
}

// SetControlPlaneEventGatewayHooks registers the event-gateway control-plane
// extension. Passing nil (the default) means this binary has no event-gateway
// support compiled in — incoming websub.*/webbroker.* control-plane events
// are logged and dropped.
func (c *Client) SetControlPlaneEventGatewayHooks(h ControlPlaneEventGatewayHooks) {
	c.eventGatewayHooks = h
}

// The following exported accessors/wrappers exist solely so that a
// ControlPlaneEventGatewayHooks implementation living outside this module can
// reuse the same generic control-plane sync primitives every other kind uses,
// without duplicating them.

// Logger returns the client's logger.
func (c *Client) Logger() *slog.Logger {
	return c.logger
}

// DB returns the client's storage handle.
func (c *Client) DB() storage.Storage {
	return c.db
}

// EventHub returns the client's EventHub instance.
func (c *Client) EventHub() eventhub.EventHub {
	return c.eventHub
}

// GatewayID returns the gateway ID this client is running for.
func (c *Client) GatewayID() string {
	return c.gatewayID
}

// APIUtilsService returns the client's platform-API HTTP helper.
func (c *Client) APIUtilsService() *utils.APIUtilsService {
	return c.apiUtilsService
}

// DeploymentService returns the client's generic API deployment service.
func (c *Client) DeploymentService() *utils.APIDeploymentService {
	return c.deploymentService
}

// WebhookSecretStore returns the client's in-memory webhook-secret store, or
// nil if none was configured.
func (c *Client) WebhookSecretStore() *webhooksecret.WebhookSecretStore {
	return c.webhookSecretStore
}

// RefreshWebhookSecretSnapshot refreshes the webhook-secret xDS snapshot via
// the configured WebhookSecretSnapshotRefresher. No-op if none was configured.
func (c *Client) RefreshWebhookSecretSnapshot() error {
	if c.webhookSecretSnapshotManager == nil {
		return nil
	}
	return c.webhookSecretSnapshotManager.RefreshSnapshot()
}

// FindAPIConfig exposes findAPIConfig for use by ControlPlaneEventGatewayHooks implementations.
func (c *Client) FindAPIConfig(apiID string) (*models.StoredConfig, error) {
	return c.findAPIConfig(apiID)
}

// ResolveLocalArtifactID exposes resolveLocalArtifactID for use by ControlPlaneEventGatewayHooks implementations.
func (c *Client) ResolveLocalArtifactID(id string) string {
	return c.resolveLocalArtifactID(id)
}

// dispatchEventGatewayHook invokes fn with the registered hooks, or logs and
// drops the event if this binary has no event-gateway support compiled in.
func (c *Client) dispatchEventGatewayHook(eventType any, fn func(ControlPlaneEventGatewayHooks)) {
	if c.eventGatewayHooks == nil {
		c.logger.Warn("Received event-gateway control-plane event but no event-gateway support is compiled into this binary",
			slog.Any("type", eventType))
		return
	}
	fn(c.eventGatewayHooks)
}

// SendDeploymentAck exposes sendDeploymentAck for use by ControlPlaneEventGatewayHooks implementations.
func (c *Client) SendDeploymentAck(deploymentID, artifactID, resourceType, action, status string, performedAt time.Time, errorCode string) {
	c.sendDeploymentAck(deploymentID, artifactID, resourceType, action, status, performedAt, errorCode)
}

// PerformFullAPIDeletion exposes performFullAPIDeletion for use by ControlPlaneEventGatewayHooks implementations.
func (c *Client) PerformFullAPIDeletion(apiID string, apiConfig *models.StoredConfig, correlationID string) {
	c.performFullAPIDeletion(apiID, apiConfig, correlationID)
}

// CleanupOrphanedResources exposes cleanupOrphanedResources for use by ControlPlaneEventGatewayHooks implementations.
func (c *Client) CleanupOrphanedResources(apiID, correlationID string) {
	c.cleanupOrphanedResources(apiID, correlationID)
}

// SyncSecretRefsFromYAML exposes syncSecretRefsFromYAML for use by ControlPlaneEventGatewayHooks implementations.
func (c *Client) SyncSecretRefsFromYAML(yamlData []byte, correlationID string) {
	c.syncSecretRefsFromYAML(yamlData, correlationID)
}
