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

	req, err := http.NewRequest(http.MethodPost, platformAPI+"/api/v0.9/secrets", buf)
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
