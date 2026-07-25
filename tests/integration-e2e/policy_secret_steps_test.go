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

// Steps for policy_secret.feature — exercises secret resolution INSIDE a
// policy's configuration parameters, as opposed to an artifact's upstream auth
// block (already covered by rest_api_secret.feature). The gateway-controller's
// secret resolution (syncSecretRefsFromYAML + the template renderer) operates
// generically over an artifact's entire rendered YAML, so a
// {{ secret "handle" }} placeholder nested under
// operations[].request.policies[].params is resolved the same way:
//
//  1. Create a secret (POST /secrets, multipart/form-data).
//  2. Create a REST API with one operation carrying a "set-headers" policy
//     whose request.headers[0].value is a {{ secret "handle" }} placeholder
//     (POST /rest-apis).
//  3. Deploy the API — attach the gateway and create the deployment WITHOUT
//     restarting the controller (deployRestAPIWithoutRestart in
//     secret_helpers_test.go). The platform-api broadcasts an api.deployed
//     WebSocket event to the already-connected controller, which resolves the
//     placeholder on demand — no restart required.
//  4. Poll the gateway management API until the API appears, confirming the
//     controller resolved the secret reference inside the policy's params.

import (
	"fmt"
	"net/http"
)

// aSecretForPolicy creates the secret backing the policy's header value.
func (w *world) aSecretForPolicy() error {
	handle, err := createSecret("E2E Policy Header Value", "e2e-test-policy-value-"+randHex())
	if err != nil {
		return err
	}
	w.policySecretHandle = handle
	return nil
}

// aRestAPIWithPolicyReferencingSecret creates a REST API with one operation
// carrying a set-headers policy whose header value embeds a
// {{ secret "handle" }} placeholder pointing at the secret above.
func (w *world) aRestAPIWithPolicyReferencingSecret() error {
	if w.policySecretHandle == "" {
		return fmt.Errorf("no secret handle — run 'a secret containing a header value' first")
	}

	suffix := randHex()
	w.policySecretContext = "/e2e-policy-" + suffix
	secretPlaceholder := `{{ secret "` + w.policySecretHandle + `" }}`

	st, body, err := apiCall(http.MethodPost, "/rest-apis", suite.token, map[string]any{
		"displayName": "e2e-policy-api-" + suffix,
		"context":     w.policySecretContext,
		"version":     "v1",
		"projectId":   suite.projectID,
		"upstream": map[string]any{
			"main": map[string]any{
				"url": "http://sample-backend:9080",
			},
		},
		"operations": []map[string]any{
			{
				"request": map[string]any{
					"method": "GET",
					"path":   "/",
					"policies": []map[string]any{
						{
							"name":    "set-headers",
							"version": "v1",
							"params": map[string]any{
								"request": map[string]any{
									"headers": []map[string]any{
										{"name": "X-Api-Token", "value": secretPlaceholder},
									},
								},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		return err
	}
	w.policySecretApiID = jsonField(body, "id", "handle", "uuid")
	if st >= 300 || w.policySecretApiID == "" {
		return fmt.Errorf("create policy-secret REST API failed (%d): %s", st, body)
	}
	return nil
}

// deployPolicySecretRestAPI deploys the REST API to gateway 1 without
// restarting the controller, so the assertion exercises the on-demand
// api.deployed event path rather than the startup bulk-sync path.
func (w *world) deployPolicySecretRestAPI() error {
	id, err := deployRestAPIWithoutRestart(w.policySecretApiID, suite.gw1ID)
	if err != nil {
		return err
	}
	w.policySecretDepID = id
	return nil
}

// gatewayHasPolicySecretRestAPIConfigured polls the gateway management API
// until the REST API appears, confirming the secret reference nested inside
// the policy's params was resolved successfully.
func (w *world) gatewayHasPolicySecretRestAPIConfigured() error {
	return waitGatewayResource("rest-apis/"+w.policySecretApiID, llmProviderPollTimeout)
}
