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

// TEMP-READ-ONLY-MODE: this whole file is part of the temporary organization-scoped
// read-only mode used while Bijira migrates from Platform API v1 to v2. Delete it
// when the mode is removed — see README.md "Read-only (maintenance) mode — temporary"
// for the removal checklist.

package handler

import (
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/model"
	ws "github.com/wso2/api-platform/platform-api/internal/websocket"
)

// SetReadOnly wires the read-only (maintenance) mode configuration into the
// WebSocket handler. A gateway of a read-only organization may still connect —
// it keeps syncing (its reads still work) and keeps retrying its writes — but
// the connection's own side-effect writes are skipped (see
// skipActiveStatusUpdate and dropDeploymentAck).
func (h *WebSocketHandler) SetReadOnly(ro *config.ReadOnly) {
	h.readOnly = ro
}

// skipActiveStatusUpdate reports whether the gateways.is_active update on
// connect/disconnect must be skipped because the gateway's organization is
// read-only, logging the skip. Since the mode is a restart-time setting and a
// restart drops every connection, connect and disconnect are always evaluated
// under the same value, so is_active is never left half-updated.
func (h *WebSocketHandler) skipActiveStatusUpdate(gateway *model.Gateway) bool {
	if h.readOnly == nil || !h.readOnly.IsReadOnlyOrg(gateway.OrganizationID) {
		return false
	}
	h.slogger.Warn("Read-only mode: skipping gateway active-status update",
		"gatewayID", gateway.ID, "orgID", gateway.OrganizationID)
	return true
}

// dropDeploymentAck reports whether a deployment.ack from the connection must be
// dropped because its organization is read-only: the deployment status rows
// must not change. An in-flight deployment of such an organization therefore
// stays in its transitional status until the mode is lifted (the timeout job
// skips it too) — the rows not changing is the point.
func (h *WebSocketHandler) dropDeploymentAck(conn *ws.Connection, ack *model.DeploymentAckPayload) bool {
	if h.readOnly == nil || !h.readOnly.IsReadOnlyOrg(conn.OrganizationID) {
		return false
	}
	h.slogger.Warn("Read-only mode: dropping deployment.ack",
		"gatewayID", conn.GatewayID, "orgID", conn.OrganizationID,
		"artifactID", ack.ArtifactID, "deploymentID", ack.DeploymentID, "status", ack.Status)
	return true
}
