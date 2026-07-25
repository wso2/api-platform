/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied.  See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

package e2e

// Steps for mcp_proxy.feature — exercises the on-demand secret fetch path for
// MCP proxies. An MCP proxy's upstream.auth uses the same shared UpstreamAuth
// schema as an LLM provider's, so a secret reference works identically:
//
//  1. Create a secret (POST /secrets, multipart/form-data).
//  2. Create an MCP proxy whose upstream.main.auth.value is a
//     {{ secret "handle" }} placeholder (POST /mcp-proxies).
//  3. Deploy the proxy (POST /mcp-proxies/{id}/deployments). The platform-api
//     broadcasts an mcpproxy.deployed WebSocket event to the already-connected
//     controller, which resolves the {{ secret "..." }} reference on demand —
//     no restart required.
//  4. Poll the gateway management API until the proxy appears, confirming the
//     controller resolved the secret reference at deploy time.

import (
	"fmt"
	"net/http"
)

// aSecretForMCPProxy creates the secret backing the MCP proxy's upstream auth.
func (w *world) aSecretForMCPProxy() error {
	handle, err := createSecret("E2E MCP Proxy Upstream Key", "e2e-test-mcp-value-"+randHex())
	if err != nil {
		return err
	}
	w.mcpProxySecretHandle = handle
	return nil
}

// anMCPProxyReferencingSecret creates an MCP proxy whose upstream auth value
// embeds a {{ secret "handle" }} placeholder pointing at the secret above.
func (w *world) anMCPProxyReferencingSecret() error {
	if w.mcpProxySecretHandle == "" {
		return fmt.Errorf("no secret handle — run 'a secret containing an MCP proxy upstream API key' first")
	}

	suffix := randHex()
	w.mcpProxyID = "e2e-mcp-proxy-" + suffix
	secretPlaceholder := `{{ secret "` + w.mcpProxySecretHandle + `" }}`

	st, body, err := apiCall(http.MethodPost, "/mcp-proxies", suite.token, map[string]any{
		"id":             w.mcpProxyID,
		"displayName":    "e2e-mcp-proxy-" + suffix,
		"version":        "v1.0",
		"mcpSpecVersion": "2025-06-18",
		"upstream": map[string]any{
			"main": map[string]any{
				"url": "http://sample-backend:9080",
				"auth": map[string]any{
					"type":   "api-key",
					"header": "Authorization",
					"value":  secretPlaceholder,
				},
			},
		},
	})
	if err != nil {
		return err
	}
	if st >= 300 {
		return fmt.Errorf("create MCP proxy failed (%d): %s", st, body)
	}
	return nil
}

// deployMCPProxyToGateway deploys the MCP proxy to gateway 1. The platform-api
// broadcasts an mcpproxy.deployed WebSocket event to the already-connected
// controller, whose handleMCPProxyDeploymentEvent resolves the secret
// reference on demand and creates the proxy configuration — no restart
// required.
func (w *world) deployMCPProxyToGateway() error {
	if w.mcpProxyID == "" {
		return fmt.Errorf("no MCP proxy id — run 'an MCP proxy that references the secret' first")
	}

	st, body, err := apiCall(http.MethodPost, "/mcp-proxies/"+w.mcpProxyID+"/deployments", suite.token,
		map[string]any{"name": "dep-" + randHex(), "base": "current", "gatewayId": suite.gw1ID})
	if err != nil {
		return err
	}
	if st >= 300 || jsonField(body, "deploymentId") == "" {
		return fmt.Errorf("deploy MCP proxy failed (%d): %s", st, body)
	}
	return nil
}

// gatewayHasMCPProxyConfigured polls the gateway management API until the MCP
// proxy appears, confirming the on-demand secret fetch succeeded.
func (w *world) gatewayHasMCPProxyConfigured() error {
	return waitGatewayResource("mcp-proxies/"+w.mcpProxyID, llmProviderPollTimeout)
}
