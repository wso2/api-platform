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

// Shared helpers for the on-demand secret fetch scenarios: llm_provider.feature,
// llm_proxy.feature, mcp_proxy.feature, rest_api_secret.feature and
// policy_secret.feature. Each scenario creates its own secret and its own
// secret-backed artifact, then polls the gateway-controller's management API
// until the artifact appears — confirming the controller resolved the
// {{ secret "..." }} reference at deploy time.

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// createSecret creates a GENERIC secret in platform-api via multipart/form-data
// and returns its handle.
func createSecret(displayName, value string) (string, error) {
	handle := "e2e-secret-" + randHex()

	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	for _, kv := range [][2]string{
		{"id", handle},
		{"displayName", displayName},
		{"value", value},
		{"type", "GENERIC"},
	} {
		if err := mw.WriteField(kv[0], kv[1]); err != nil {
			return "", err
		}
	}
	mw.Close()

	req, err := http.NewRequest(http.MethodPost, platformAPI+platformAPIBase+"/secrets", buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+suite.token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("create secret failed (%d): %s", resp.StatusCode, body)
	}
	return handle, nil
}

// deployRestAPIWithoutRestart attaches the gateway to a REST API and creates a
// deployment, WITHOUT restarting the controller — unlike the shared deploy()
// helper in steps_test.go. platform-api broadcasts an api.deployed WebSocket
// event to the already-connected controller, whose handleAPIDeployedEvent
// resolves any {{ secret "..." }} reference in the rendered YAML on demand
// before creating the API configuration, so a restart is not needed to
// exercise (or verify) that on-demand path. Returns the deployment id.
func deployRestAPIWithoutRestart(apiID, gatewayID string) (string, error) {
	if st, body, err := apiCall(http.MethodPost, "/rest-apis/"+apiID+"/gateways", suite.token,
		[]map[string]string{{"gatewayId": gatewayID}}); err != nil {
		return "", err
	} else if st >= 300 {
		return "", fmt.Errorf("attach gateway failed (%d): %s", st, body)
	}
	st, body, err := apiCall(http.MethodPost, "/rest-apis/"+apiID+"/deployments", suite.token,
		map[string]any{"base": "current", "gatewayId": gatewayID, "name": "dep-" + randHex()})
	if err != nil {
		return "", err
	}
	id := jsonField(body, "deploymentId")
	if st >= 300 || id == "" {
		return "", fmt.Errorf("deploy failed (%d): %s", st, body)
	}
	return id, nil
}

// waitGatewayResource polls GET <gwMgmtAPI>/api/management/v1/<resourcePath> on the
// gateway-controller's management API (basic auth admin:admin) until it returns 200
// or timeout expires. resourcePath is e.g. "llm-proxies/e2e-llm-proxy-abcd1234".
func waitGatewayResource(resourcePath string, timeout time.Duration) error {
	url := gwMgmtAPI + "/api/management/v1/" + resourcePath
	deadline := time.Now().Add(timeout)
	var lastStatus int
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		req.SetBasicAuth("admin", "admin")
		resp, err := httpClient.Do(req)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		resp.Body.Close()
		lastStatus = resp.StatusCode
		if lastStatus == http.StatusOK {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("gateway did not configure resource %q within timeout: last status %d",
		resourcePath, lastStatus)
}
