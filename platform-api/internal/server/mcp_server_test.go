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

package server

import (
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/mcpserver"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
)

// TestStartMCPServer_RefusesAPluginOverrideOnAToolRoute: a tool calls the
// services, so a plugin's decorator on the route it stands for would not run.
func TestStartMCPServer_RefusesAPluginOverrideOnAToolRoute(t *testing.T) {
	const specPath = "../../resources/openapi.yaml"
	scopes, err := middleware.LoadScopeRegistry(specPath)
	require.NoError(t, err)
	cfg := &config.Server{}
	cfg.MCPServer = config.MCPServer{Enabled: true}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	start := func(overrides map[string]routeOverride) error {
		_, _, err := startMCPServer(logger, http.NewServeMux(), overrides, mcpserver.Deps{Config: cfg, Scopes: scopes})
		return err
	}
	wrap := func(next http.Handler) http.Handler { return next }

	assert.NoError(t, start(nil))
	assert.NoError(t, start(map[string]routeOverride{
		"GET /api/v0.9/projects/{projectId}": {plugin: "cloud", wrap: wrap},
	}), "no tool uses this route")

	err = start(map[string]routeOverride{"POST /api/v0.9/projects": {plugin: "cloud", wrap: wrap}})
	assert.ErrorContains(t, err, `tool create_project uses POST /api/v0.9/projects, which plugin "cloud" overrides`)

	err = start(map[string]routeOverride{"DELETE /api/v0.9/projects/{projectId}": {plugin: "cloud", wrap: wrap}})
	assert.ErrorContains(t, err, "tool delete_project uses DELETE /api/v0.9/projects/{id}", "wildcard names do not matter")

	err = start(map[string]routeOverride{"PUT /api/v0.9/llm-proxies/{proxyId}": {plugin: "billing", wrap: wrap}})
	assert.ErrorContains(t, err, "tool update_api uses PUT /api/v0.9/llm-proxies/{id}", "every API type's routes are checked")
}
