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

// Steps for llm_provider.feature — exercises the end-to-end path:
//
//  1. Create a secret in platform-api (POST /api/v0.9/secrets, multipart/form-data).
//  2. Create an LLM provider whose upstream auth value is a {{ secret "handle" }}
//     placeholder (POST /api/v0.9/llm-providers).
//  3. Deploy the provider to the gateway (POST /api/v0.9/llm-providers/{id}/deployments).
//     The platform-api broadcasts a llmprovider.deployed WebSocket event; the controller
//     resolves {{ secret "..." }} references on demand (no restart required).
//  4. Poll the gateway management API until the provider appears, confirming that
//     the controller successfully resolved secret references at deploy time.

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// aSecretWithAPIKey creates a GENERIC secret in platform-api via multipart/form-data.
// The handle is persisted on w.secretHandle for the next step.
func (w *world) aSecretWithAPIKey() error {
	w.secretHandle = "e2e-secret-" + randHex()

	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	for _, kv := range [][2]string{
		{"id", w.secretHandle},
		{"displayName", "E2E Provider API Key"},
		{"value", "e2e-test-key-value-" + randHex()},
		{"type", "GENERIC"},
	} {
		if err := mw.WriteField(kv[0], kv[1]); err != nil {
			return err
		}
	}
	mw.Close()

	req, err := http.NewRequest(http.MethodPost, platformAPI+"/api/v0.9/secrets", buf)
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
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("create secret failed (%d): %s", resp.StatusCode, body)
	}
	return nil
}

// anLLMProviderReferencingSecret creates an LLM provider whose upstream auth value
// embeds a {{ secret "handle" }} placeholder pointing at the secret created above.
// The provider id is persisted on w.llmProviderID.
func (w *world) anLLMProviderReferencingSecret() error {
	if w.secretHandle == "" {
		return fmt.Errorf("no secret handle — run 'a secret containing an LLM provider API key' first")
	}

	suffix := randHex()
	w.llmProviderID = "e2e-llm-provider-" + suffix

	// The gateway controller resolves {{ secret "handle" }} at deploy time by
	// calling FetchPlatformSecretValue on demand — that is the behaviour under test.
	secretPlaceholder := `{{ secret "` + w.secretHandle + `" }}`

	st, body, err := apiCall(http.MethodPost, "/api/v0.9/llm-providers", suite.token, map[string]any{
		"id":          w.llmProviderID,
		"displayName": "e2e-llm-provider-" + suffix,
		"description": "E2E test LLM provider",
		"version":     "v1.0",
		"template":    "openai",
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
		"accessControl": map[string]any{
			"mode": "allow_all",
		},
	})
	if err != nil {
		return err
	}
	if st >= 300 {
		return fmt.Errorf("create LLM provider failed (%d): %s", st, body)
	}
	return nil
}

// deployLLMProviderToGateway deploys the LLM provider to gateway 1. The platform-api
// broadcasts a llmprovider.deployed WebSocket event to the connected controller. The
// controller's handleLLMProviderDeployedEvent fetches the provider definition, calls
// syncSecretRefsFromYAML to resolve any {{ secret "..." }} references on demand, then
// creates the LLM provider configuration — no restart required.
func (w *world) deployLLMProviderToGateway() error {
	if w.llmProviderID == "" {
		return fmt.Errorf("no LLM provider id — run 'an LLM provider that references the secret' first")
	}

	st, body, err := apiCall(
		http.MethodPost,
		"/api/v0.9/llm-providers/"+w.llmProviderID+"/deployments",
		suite.token,
		map[string]any{
			"name":      "dep-" + randHex(),
			"base":      "current",
			"gatewayId": suite.gw1ID,
		},
	)
	if err != nil {
		return err
	}
	w.llmDepID = jsonField(body, "deploymentId")
	if st >= 300 || w.llmDepID == "" {
		return fmt.Errorf("deploy LLM provider failed (%d): %s", st, body)
	}
	return nil
}

// gatewayHasLLMProviderConfigured polls the gateway management API until the LLM
// provider appears, confirming that the on-demand secret fetch succeeded and the
// provider was created in the gateway without a "configuration not found" error.
func (w *world) gatewayHasLLMProviderConfigured() error {
	return waitGatewayLLMProvider(w.llmProviderID)
}

// waitGatewayLLMProvider polls GET /api/management/v1/llm-providers/{id} on the
// gateway management API until it returns 200 or the poll timeout expires.
func waitGatewayLLMProvider(providerID string) error {
	url := gwMgmtAPI + "/api/management/v1/llm-providers/" + providerID
	deadline := time.Now().Add(pollTimeout)
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
	return fmt.Errorf("gateway did not configure LLM provider %q within timeout: last status %d",
		providerID, lastStatus)
}
