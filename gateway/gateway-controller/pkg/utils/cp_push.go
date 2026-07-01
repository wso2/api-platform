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

package utils

import (
	"log/slog"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// ArtifactPusher is the minimal control-plane client surface the deployment services need
// to push a gateway-created artifact to the control plane (DP->CP). It is implemented by
// *controlplane.Client. It is declared here rather than reusing
// controlplane.ControlPlaneClient because pkg/controlplane imports pkg/utils, so utils
// cannot import controlplane without creating an import cycle.
type ArtifactPusher interface {
	IsConnected() bool
	PushArtifact(artifactID string, artifact *models.StoredConfig, deploymentID string) error
}

const (
	cpPushDeploymentTimeout = 30 * time.Second
	cpPushPollInterval      = 500 * time.Millisecond
)

// cpSyncStatusForOrigin returns the initial cp_sync_status for a newly created/updated
// artifact row in the gateway DB: "pending" for gateway-originated artifacts (those created
// on the data plane — including via the immutable loader — which still need to be synced up to
// the control plane) and "" (NULL) for control-plane-originated artifacts, which the control
// plane already owns and the gateway must not push back. The DP->CP push later flips a pending
// row to success/failed (see Client.recordArtifactSyncStatus).
func cpSyncStatusForOrigin(origin models.Origin) models.CPSyncStatus {
	if origin == models.OriginGatewayAPI {
		return models.CPSyncStatusPending
	}
	return ""
}

// waitForDeploymentAndPush waits for the artifact identified by configID to finish deploying
// (its DeployedAt is set in the store) and then pushes it to the control plane (DP->CP). It is
// the gateway-create counterpart of the control plane's deployment callback and mirrors the
// REST API push path; it is meant to be run in a goroutine. The caller is responsible for
// gating on connectivity/push-enabled and gateway origin before invoking it.
func waitForDeploymentAndPush(store *storage.ConfigStore, pusher ArtifactPusher, configID, correlationID string, log *slog.Logger) {
	if log == nil {
		log = slog.Default()
	}
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	timeout := time.NewTimer(cpPushDeploymentTimeout)
	ticker := time.NewTicker(cpPushPollInterval)
	defer timeout.Stop()
	defer ticker.Stop()

	for {
		select {
		case <-timeout.C:
			log.Warn("Timeout waiting for artifact deployment to complete before pushing to control plane",
				slog.String("config_id", configID))
			return

		case <-ticker.C:
			cfg, err := store.Get(configID)
			if err != nil {
				log.Warn("Config not found while waiting for deployment completion",
					slog.String("config_id", configID))
				continue
			}

			if cfg.DeployedAt != nil {
				log.Info("Artifact deployed successfully, pushing to control plane",
					slog.String("config_id", configID),
					slog.String("displayName", cfg.DisplayName))

				if err := pusher.PushArtifact(configID, cfg, cfg.DeploymentID); err != nil {
					log.Error("Failed to push deployment to control plane",
						slog.String("artifact_id", configID),
						slog.Any("error", err))
				} else {
					log.Info("Successfully pushed deployment to control plane",
						slog.String("artifact_id", configID))
				}
				return
			}
		}
	}
}
