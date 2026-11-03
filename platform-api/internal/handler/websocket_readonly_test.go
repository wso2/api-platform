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
// when the mode is removed.

package handler

import (
	"io"
	"log/slog"
	"testing"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/model"
	ws "github.com/wso2/api-platform/platform-api/internal/websocket"
)

func TestWebSocketHandler_readOnlyHelpers(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewWebSocketHandler(nil, nil, nil, 10, logger)
	frozenGateway := &model.Gateway{ID: "gw-a", OrganizationID: readOnlyTestOrgA}
	writableGateway := &model.Gateway{ID: "gw-b", OrganizationID: readOnlyTestOrgB}
	frozenConn := &ws.Connection{GatewayID: "gw-a", OrganizationID: readOnlyTestOrgA}
	writableConn := &ws.Connection{GatewayID: "gw-b", OrganizationID: readOnlyTestOrgB}
	ack := &model.DeploymentAckPayload{DeploymentID: "d1", ArtifactID: "a1", Action: "deploy", Status: "success"}

	// Never wired: nothing is skipped or dropped.
	if h.skipActiveStatusUpdate(frozenGateway) || h.dropDeploymentAck(frozenConn, ack) {
		t.Fatal("unwired handler must not skip or drop anything")
	}

	h.SetReadOnly(&config.ReadOnly{Enabled: true, WritableOrganizations: []string{readOnlyTestOrgB}})

	if !h.skipActiveStatusUpdate(frozenGateway) {
		t.Error("active-status update for a frozen organization's gateway must be skipped")
	}
	if h.skipActiveStatusUpdate(writableGateway) {
		t.Error("active-status update for a writable organization's gateway must run")
	}
	if !h.dropDeploymentAck(frozenConn, ack) {
		t.Error("deployment.ack from a frozen organization's gateway must be dropped")
	}
	if h.dropDeploymentAck(writableConn, ack) {
		t.Error("deployment.ack from a writable organization's gateway must be processed")
	}

	h.SetReadOnly(&config.ReadOnly{Enabled: false})
	if h.skipActiveStatusUpdate(frozenGateway) || h.dropDeploymentAck(frozenConn, ack) {
		t.Fatal("disabled mode must not skip or drop anything")
	}
}
