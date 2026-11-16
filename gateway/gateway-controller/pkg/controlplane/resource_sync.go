/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// resourceSyncTimeout bounds one push request.
const resourceSyncTimeout = 30 * time.Second

// Gateway-created resources that are not deployable artifacts (models.SyncedResourceKinds)
// are pushed to the control plane the way artifacts are (import-gateway-artifacts): one
// item after each create or update (PushResource, like PushArtifact), a deletion when one
// is deleted (PushResourceDeletion, like PushArtifactUndeploy), and every one on connect
// (PushGatewayResourcesToControlPlane, like PushGatewayArtifactsToControlPlane). The push on
// connect is marked complete, so the control plane also removes the ones deleted while the
// gateway was disconnected. The control plane matches a resource by kind and name and
// returns its own ID, stored as cp_artifact_id.

// gatewayResource is one resource in a push: the document as the gateway stores it (its
// kind is configuration.kind), keyed by the gateway's ID for it.
type gatewayResource struct {
	DPID          string    `json:"dpid"`
	Configuration any       `json:"configuration"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// gatewayResourcesRequest is the body of POST {base}/gateways/{gatewayID}/resources.
// Complete marks Resources as everything the gateway created.
type gatewayResourcesRequest struct {
	Complete  bool              `json:"complete"`
	Resources []gatewayResource `json:"resources"`
	Deleted   []gatewayResource `json:"deleted"`
}

// gatewayResourceResult is the control plane's outcome for one resource. ID is its ID
// for the resource.
type gatewayResourceResult struct {
	ID     string `json:"id"`
	Status string `json:"status"` // "synced" or "failed"
	Error  string `json:"error,omitempty"`
}

// gatewayResourcesResponse holds a result per resource, keyed by dpid, with counts.
type gatewayResourcesResponse struct {
	Total     int                              `json:"total"`
	Success   int                              `json:"success"`
	Failed    int                              `json:"failed"`
	Resources map[string]gatewayResourceResult `json:"resources"`
}

// canPushResources reports whether a DP->CP push should be attempted now.
// The pushed resources are budgets, so nothing is pushed when budgets are not enabled
// on this gateway (no budget applier is set).
func (c *Client) canPushResources() bool {
	return c.IsConnected() && c.config.DeploymentSyncEnabled && !c.IsOnPrem() && c.getBudgetApplier() != nil
}

// errResourcesNotSupported is a control plane that does not take gateway resources: it
// answers 404 (AI Workspace). Retrying cannot help.
var errResourcesNotSupported = errors.New("the control plane does not support gateway resources")

// PushResource pushes a gateway-created resource after it is created or updated.
func (c *Client) PushResource(cfg *models.StoredConfig) {
	if !isPushedResource(cfg) || !c.canPushResources() {
		return
	}
	item := toGatewayResource(cfg)
	c.SubmitArtifactPush(func() {
		c.pushResources(gatewayResourcesRequest{Resources: []gatewayResource{item}}, map[string]*models.StoredConfig{cfg.UUID: cfg})
	})
}

// PushResourceDeletion tells the control plane that a gateway-created resource was deleted.
func (c *Client) PushResourceDeletion(cfg *models.StoredConfig) {
	if !isPushedResource(cfg) || !c.canPushResources() {
		return
	}
	item := toGatewayResource(cfg)
	c.SubmitArtifactPush(func() {
		c.pushResources(gatewayResourcesRequest{Deleted: []gatewayResource{item}}, nil)
	})
}

// PushGatewayResourcesToControlPlane pushes every gateway-created resource, as a complete
// list. It runs on connect and reconnect.
func (c *Client) PushGatewayResourcesToControlPlane() {
	if c.canPushResources() {
		c.pushGatewayResources()
	}
}

// pushGatewayResources lists and pushes every gateway-created resource. It is separated
// from the connection gating so it can be tested in isolation.
func (c *Client) pushGatewayResources() {
	if c.db == nil {
		c.logger.Warn("Cannot push gateway resources to control plane: database is not available")
		return
	}
	req := gatewayResourcesRequest{Complete: true, Resources: []gatewayResource{}}
	sent := map[string]*models.StoredConfig{}
	for _, kind := range models.SyncedResourceKinds() {
		configs, err := c.db.GetAllConfigsByKind(kind)
		if err != nil {
			// Not complete without this kind: pushing the rest as complete would delete it.
			c.logger.Error("Failed to list gateway resources for control plane push",
				slog.String("kind", kind), slog.Any("error", err))
			return
		}
		for _, cfg := range configs {
			if isPushedResource(cfg) {
				req.Resources = append(req.Resources, toGatewayResource(cfg))
				sent[cfg.UUID] = cfg
			}
		}
	}
	c.logger.Info("Pushing gateway resources to control plane", slog.Int("count", len(req.Resources)))
	c.pushResources(req, sent)
}

// isPushedResource reports whether cfg is a gateway-created resource of a pushed kind;
// control-plane resources are never sent back.
func isPushedResource(cfg *models.StoredConfig) bool {
	return cfg != nil && cfg.Origin == models.OriginGatewayAPI && models.IsSyncedResourceKind(cfg.Kind)
}

func toGatewayResource(cfg *models.StoredConfig) gatewayResource {
	return gatewayResource{
		DPID: cfg.UUID, Configuration: cfg.SourceConfiguration,
		CreatedAt: cfg.CreatedAt.UTC(), UpdatedAt: cfg.UpdatedAt.UTC(),
	}
}

// pushResources sends a push, retrying it like an artifact push (retryPush), and records
// the outcome of each resource in sent.
func (c *Client) pushResources(req gatewayResourcesRequest, sent map[string]*models.StoredConfig) {
	gatewayID := c.connectedGatewayID()
	if gatewayID == "" {
		c.logger.Warn("Cannot push gateway resources to control plane: gateway ID is not known yet")
		return
	}
	var resp *gatewayResourcesResponse
	unsupported := false
	err := c.retryPush("resource push", func() error {
		var err error
		resp, err = c.postGatewayResources(gatewayID, req)
		if errors.Is(err, errResourcesNotSupported) {
			unsupported = true
			return nil
		}
		return err
	}, slog.Int("count", len(req.Resources)+len(req.Deleted)))
	if unsupported {
		c.logger.Debug("The control plane does not support gateway resources; nothing was pushed")
		return
	}
	if err != nil {
		// The resources keep their pending/failed cp_sync_status and are pushed again on
		// the next (re)connect.
		c.logger.Error("Failed to push gateway resources to control plane", slog.Any("error", err))
		return
	}
	for id, cfg := range sent {
		result, ok := resp.Resources[id]
		if !ok {
			c.logger.Warn("Control plane returned no result for pushed resource; leaving it pending",
				slog.String("id", id))
			continue
		}
		c.recordResourceSyncStatus(cfg, result)
	}
	for _, item := range req.Deleted {
		if result := resp.Resources[item.DPID]; result.Error != "" {
			c.logger.Warn("Control plane failed to delete resource", slog.String("id", item.DPID), slog.String("error", result.Error))
		}
	}
	c.logger.Info("Completed pushing gateway resources to control plane",
		slog.Int("total", resp.Total), slog.Int("success", resp.Success), slog.Int("failed", resp.Failed))
}

// recordResourceSyncStatus stores a push outcome on the resource's row, like
// recordArtifactSyncStatus: the control plane's ID on success, the error on failure.
func (c *Client) recordResourceSyncStatus(cfg *models.StoredConfig, result gatewayResourceResult) {
	status, cpID, info := models.CPSyncStatusSuccess, result.ID, ""
	if result.Error != "" || result.Status != "synced" {
		status, cpID, info = models.CPSyncStatusFailed, cfg.CPArtifactID, result.Error
	}
	if err := c.db.UpdateCPSyncStatus(cfg.UUID, cpID, status, info); err != nil {
		c.logger.Warn("Failed to record resource CP sync status",
			slog.String("id", cfg.UUID), slog.Any("error", err))
	}
}

// connectedGatewayID is the control plane's ID for this gateway, from connection.ack.
func (c *Client) connectedGatewayID() string {
	c.state.mu.RLock()
	defer c.state.mu.RUnlock()
	return c.state.GatewayID
}

// postGatewayResources sends the request to POST {base}/gateways/{gatewayID}/resources.
func (c *Client) postGatewayResources(gatewayID string, body gatewayResourcesRequest) (*gatewayResourcesResponse, error) {
	endpoint := c.getRestAPIBaseURL() + "/gateways/" + url.PathEscape(gatewayID) + "/resources"
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal gateway resources: %w", err)
	}
	ctx, cancel := context.WithTimeout(c.ctx, resourceSyncTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gateway resources request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", c.config.Token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send gateway resources: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read gateway resources response: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, errResourcesNotSupported
	}
	if resp.StatusCode != http.StatusOK {
		if isPermanentControlPlaneStatus(resp.StatusCode) {
			return nil, fmt.Errorf("%w: gateway resources sync returned %d: %s",
				errPermanentControlPlaneFailure, resp.StatusCode, string(respBody))
		}
		return nil, fmt.Errorf("gateway resources sync failed with status %d: %s", resp.StatusCode, string(respBody))
	}
	var out gatewayResourcesResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("failed to parse gateway resources response: %w", err)
	}
	return &out, nil
}
