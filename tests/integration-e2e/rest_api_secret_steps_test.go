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

// Steps for rest_api_secret.feature — exercises the on-demand secret fetch path
// for plain REST APIs. A REST API's upstream.auth uses the same shared
// UpstreamAuth schema as an LLM provider's, so a secret reference works
// identically:
//
//  1. Create a secret (POST /secrets, multipart/form-data).
//  2. Create a REST API whose upstream.main.auth.value is a
//     {{ secret "handle" }} placeholder (POST /rest-apis).
//  3. Deploy the API — attach the gateway and create the deployment WITHOUT
//     restarting the controller (deployRestAPIWithoutRestart in
//     secret_helpers_test.go, unlike the plain api-deployment.feature scenario's
//     shared deploy() helper). The platform-api broadcasts an api.deployed
//     WebSocket event to the already-connected controller, which resolves the
//     {{ secret "..." }} reference on demand — no restart required.
//  4. Poll the gateway management API until the API appears, confirming the
//     controller resolved the secret reference at deploy time.

import (
	"fmt"
	"net/http"
)

// aSecretForRestAPI creates the secret backing the REST API's upstream auth.
func (w *world) aSecretForRestAPI() error {
	handle, err := createSecret("E2E REST API Upstream Credential", "e2e-test-restapi-value-"+randHex())
	if err != nil {
		return err
	}
	w.restAPISecretHandle = handle
	return nil
}

// aRestAPIReferencingSecret creates a REST API whose upstream auth value
// embeds a {{ secret "handle" }} placeholder pointing at the secret above.
func (w *world) aRestAPIReferencingSecret() error {
	if w.restAPISecretHandle == "" {
		return fmt.Errorf("no secret handle — run 'a secret containing a REST API upstream credential' first")
	}

	suffix := randHex()
	w.restAPISecretContext = "/e2e-secret-" + suffix
	secretPlaceholder := `{{ secret "` + w.restAPISecretHandle + `" }}`

	st, body, err := apiCall(http.MethodPost, "/rest-apis", suite.token, map[string]any{
		"displayName": "e2e-secret-api-" + suffix,
		"context":     w.restAPISecretContext,
		"version":     "v1",
		"projectId":   suite.projectID,
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
	w.restAPISecretApiID = jsonField(body, "id", "handle", "uuid")
	if st >= 300 || w.restAPISecretApiID == "" {
		return fmt.Errorf("create secret-backed REST API failed (%d): %s", st, body)
	}
	return nil
}

// deploySecretBackedRestAPI deploys the REST API to gateway 1 without
// restarting the controller, so the assertion exercises the on-demand
// api.deployed event path rather than the startup bulk-sync path.
func (w *world) deploySecretBackedRestAPI() error {
	id, err := deployRestAPIWithoutRestart(w.restAPISecretApiID, suite.gw1ID)
	if err != nil {
		return err
	}
	w.restAPISecretDepID = id
	return nil
}

// gatewayHasSecretBackedRestAPIConfigured polls the gateway management API
// until the REST API appears, confirming the on-demand secret fetch succeeded.
func (w *world) gatewayHasSecretBackedRestAPIConfigured() error {
	return waitGatewayResource("rest-apis/"+w.restAPISecretApiID, llmProviderPollTimeout)
}
