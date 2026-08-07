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
//
// rotateSecret / waitGatewaySecretValue / waitGatewaySecretGone below back a
// different scenario family — secret_lifecycle.feature — which exercises the
// live secret.updated/secret.deleted WebSocket push path (handleSecretUpdatedEvent /
// handleSecretDeletedEvent) against an already-connected gateway, rather than the
// on-demand fetch that happens at artifact-deploy time.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"time"
)

// defaultMaxSecretHelperRespBytes bounds how much of a platform-api response body
// these secret helpers will buffer into memory — a safety ceiling against a
// misbehaving server returning an unbounded stream, not an expected size.
// Override via E2E_MAX_RESP_BYTES for scenarios that legitimately need more.
const defaultMaxSecretHelperRespBytes = 10 << 20 // 10 MiB

// maxSecretHelperRespBytes returns the configured response-body size ceiling
// (E2E_MAX_RESP_BYTES), falling back to defaultMaxSecretHelperRespBytes when unset
// or invalid.
func maxSecretHelperRespBytes() int64 {
	if v := os.Getenv("E2E_MAX_RESP_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return defaultMaxSecretHelperRespBytes
}

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
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxSecretHelperRespBytes()))
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

// rotateSecret rotates an existing secret's value via PUT /secrets/:handle
// (multipart/form-data) — the same call the AI Workspace UI's "Rotate secret"
// action makes. platform-api broadcasts a secret.updated event to every gateway
// in the org as part of this call.
func rotateSecret(handle, newValue string) error {
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	if err := mw.WriteField("value", newValue); err != nil {
		return err
	}
	mw.Close()

	req, err := http.NewRequest(http.MethodPut, platformAPI+platformAPIBase+"/secrets/"+handle, buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+suite.token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxSecretHelperRespBytes()))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("rotate secret failed (%d): %s", resp.StatusCode, body)
	}
	return nil
}

// deleteSecret permanently deletes a secret via DELETE /secrets/:handle — the
// same call the AI Workspace UI's "Delete secret" action makes. platform-api broadcasts
// a secret.deleted event to every gateway in the org once the delete actually
// succeeds (it 409s instead if the handle is still referenced by any artifact, current
// config or deployed snapshot, on any gateway).
func deleteSecret(handle string) error {
	req, err := http.NewRequest(http.MethodDelete, platformAPI+platformAPIBase+"/secrets/"+handle, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+suite.token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxSecretHelperRespBytes()))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("delete secret failed (%d): %s", resp.StatusCode, body)
	}
	return nil
}

// gatewaySecretValue extracts spec.value from a gateway-controller GetSecret
// response body (GET /api/management/v1/secrets/:handle — see
// buildSecretResourceResponse in the gateway-controller source).
func gatewaySecretValue(body []byte) string {
	var r struct {
		Spec struct {
			Value string `json:"value"`
		} `json:"spec"`
	}
	if json.Unmarshal(body, &r) != nil {
		return ""
	}
	return r.Spec.Value
}

// waitGatewaySecretValue polls the gateway-controller's own secret store
// (GET /api/management/v1/secrets/:handle, basic auth admin:admin) until its
// decrypted value equals expectedValue or timeout expires. Confirms a
// secret.updated push event caused an immediate re-fetch — not merely that the
// gateway will eventually catch up on its next reconnect-triggered poll.
func waitGatewaySecretValue(handle, expectedValue string, timeout time.Duration) error {
	url := gwMgmtAPI + "/api/management/v1/secrets/" + handle
	deadline := time.Now().Add(timeout)
	var lastValue string
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
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			lastValue = gatewaySecretValue(body)
			if lastValue == expectedValue {
				return nil
			}
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("gateway secret %q did not reach the rotated value within timeout (last seen: %q)",
		handle, lastValue)
}

// waitGatewaySecretGone polls the gateway-controller's own secret store until
// GET /api/management/v1/secrets/:handle returns 404 or timeout expires.
// Confirms a secret.deleted push event caused the gateway to evict its local
// copy of a secret that is no longer referenced by any artifact.
func waitGatewaySecretGone(handle string, timeout time.Duration) error {
	url := gwMgmtAPI + "/api/management/v1/secrets/" + handle
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
		if lastStatus == http.StatusNotFound {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("gateway did not evict secret %q within timeout: last status %d", handle, lastStatus)
}
