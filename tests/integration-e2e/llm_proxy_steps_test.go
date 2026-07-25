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

// Steps for llm_proxy.feature — exercises the on-demand secret fetch path for
// LLM proxies. An LLM proxy fronts an already-deployed LLM provider (referenced
// by id) and may override the auth used to call it:
//
//  1. Create and deploy a plain LLM provider (POST /llm-providers,
//     POST /llm-providers/{id}/deployments) for the proxy to reference.
//  2. Create a secret (POST /secrets, multipart/form-data).
//  3. Create an LLM proxy referencing the provider by id, with its auth
//     override set to a {{ secret "handle" }} placeholder (POST /llm-proxies).
//  4. Deploy the proxy (POST /llm-proxies/{id}/deployments). The platform-api
//     broadcasts an llmproxy.deployed WebSocket event to the already-connected
//     controller, which resolves the {{ secret "..." }} reference on demand —
//     no restart required.
//  5. Poll the gateway management API until the proxy appears, confirming the
//     controller resolved the secret reference at deploy time.

import (
	"fmt"
	"net/http"
)

// aBaseLLMProviderForProxy creates and deploys a plain LLM provider (no secret
// involved) that the proxy under test will reference by id.
func (w *world) aBaseLLMProviderForProxy() error {
	suffix := randHex()
	w.llmProxyBaseProviderID = "e2e-llm-proxy-base-" + suffix

	st, body, err := apiCall(http.MethodPost, "/llm-providers", suite.token, map[string]any{
		"id":          w.llmProxyBaseProviderID,
		"displayName": "e2e-llm-proxy-base-" + suffix,
		"description": "E2E base provider for LLM proxy test",
		"version":     "v1.0",
		"template":    "openai",
		"upstream": map[string]any{
			"main": map[string]any{
				"url": "http://sample-backend:9080",
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
		return fmt.Errorf("create base LLM provider failed (%d): %s", st, body)
	}

	st, body, err = apiCall(http.MethodPost, "/llm-providers/"+w.llmProxyBaseProviderID+"/deployments", suite.token,
		map[string]any{"name": "dep-" + randHex(), "base": "current", "gatewayId": suite.gw1ID})
	if err != nil {
		return err
	}
	if st >= 300 || jsonField(body, "deploymentId") == "" {
		return fmt.Errorf("deploy base LLM provider failed (%d): %s", st, body)
	}
	return nil
}

// aSecretForLLMProxy creates the secret backing the proxy's auth override.
func (w *world) aSecretForLLMProxy() error {
	handle, err := createSecret("E2E LLM Proxy Auth Override", "e2e-test-proxy-value-"+randHex())
	if err != nil {
		return err
	}
	w.llmProxySecretHandle = handle
	return nil
}

// anLLMProxyReferencingProviderAndSecret creates an LLM proxy whose provider
// reference is the base provider above, and whose auth override embeds a
// {{ secret "handle" }} placeholder pointing at the secret created above.
func (w *world) anLLMProxyReferencingProviderAndSecret() error {
	if w.llmProxyBaseProviderID == "" {
		return fmt.Errorf("no base provider — run 'an LLM provider deployed to the gateway for the proxy to reference' first")
	}
	if w.llmProxySecretHandle == "" {
		return fmt.Errorf("no secret handle — run 'a secret containing an LLM proxy API key' first")
	}

	suffix := randHex()
	w.llmProxyID = "e2e-llm-proxy-" + suffix
	secretPlaceholder := `{{ secret "` + w.llmProxySecretHandle + `" }}`

	st, body, err := apiCall(http.MethodPost, "/llm-proxies", suite.token, map[string]any{
		"id":          w.llmProxyID,
		"displayName": "e2e-llm-proxy-" + suffix,
		"version":     "v1.0",
		"projectId":   suite.projectID,
		"provider": map[string]any{
			"id": w.llmProxyBaseProviderID,
			"auth": map[string]any{
				"type":   "api-key",
				"header": "Authorization",
				"value":  secretPlaceholder,
			},
		},
	})
	if err != nil {
		return err
	}
	if st >= 300 {
		return fmt.Errorf("create LLM proxy failed (%d): %s", st, body)
	}
	return nil
}

// deployLLMProxyToGateway deploys the LLM proxy to gateway 1. The platform-api
// broadcasts an llmproxy.deployed WebSocket event to the already-connected
// controller, whose handleLLMProxyDeployedEvent fetches the proxy definition,
// calls syncSecretRefsFromYAML to resolve the {{ secret "..." }} reference on
// demand, then creates the proxy configuration — no restart required.
func (w *world) deployLLMProxyToGateway() error {
	if w.llmProxyID == "" {
		return fmt.Errorf("no LLM proxy id — run 'an LLM proxy that references the provider and the secret' first")
	}

	st, body, err := apiCall(http.MethodPost, "/llm-proxies/"+w.llmProxyID+"/deployments", suite.token,
		map[string]any{"name": "dep-" + randHex(), "base": "current", "gatewayId": suite.gw1ID})
	if err != nil {
		return err
	}
	if st >= 300 || jsonField(body, "deploymentId") == "" {
		return fmt.Errorf("deploy LLM proxy failed (%d): %s", st, body)
	}
	return nil
}

// gatewayHasLLMProxyConfigured polls the gateway management API until the LLM
// proxy appears, confirming the on-demand secret fetch succeeded.
func (w *world) gatewayHasLLMProxyConfigured() error {
	return waitGatewayResource("llm-proxies/"+w.llmProxyID, llmProviderPollTimeout)
}
