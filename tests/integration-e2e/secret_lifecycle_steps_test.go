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

// Steps for secret_lifecycle.feature — exercises the LIVE push-event path for an
// already-referenced, already-connected secret, as opposed to rest_api_secret.feature
// (and siblings) which exercise the on-demand fetch that happens at artifact-deploy
// time. The Background/Given steps here are shared with rest_api_secret.feature
// (aSecretForRestAPI, aRestAPIReferencingSecret, deploySecretBackedRestAPI,
// gatewayHasSecretBackedRestAPIConfigured in rest_api_secret_steps_test.go) — this
// feature picks up from an already-deployed, already-resolved secret-backed REST API
// and then rotates or replaces the secret:
//
//  1. Rotation: PUT /secrets/:handle with a new value (rotateSecret in
//     secret_helpers_test.go). platform-api broadcasts secret.updated to every
//     gateway in the org; the already-connected controller's handleSecretUpdatedEvent
//     re-fetches the plaintext and upserts it, without a restart. The assertion polls
//     the gateway-controller's own GET /api/management/v1/secrets/:handle until its
//     decrypted value matches the rotated one.
//  2. Deletion: PUT /rest-apis/:id to swap the upstream auth to a brand-new secret,
//     then redeploy (without this, the gateway's already-deployed snapshot still
//     references the original handle — artifact_secret_refs keeps a gateway-scoped
//     row for it independently of the artifact's current config — so the explicit
//     DELETE below would 409 as still-referenced). Once the redeploy clears that
//     gateway-scoped row, the original secret is referenced by nothing and
//     DELETE /secrets/:handle succeeds, broadcasting secret.deleted; the
//     controller's handleSecretDeletedEvent evicts its local copy. The assertion
//     polls the same management endpoint until it 404s.

import (
	"fmt"
	"net/http"
	"strings"
)

// iRotateTheSecretToANewValue rotates w.restAPISecretHandle to a fresh value,
// exercising the live secret.updated push-event path.
func (w *world) iRotateTheSecretToANewValue() error {
	if w.restAPISecretHandle == "" {
		return fmt.Errorf("no secret handle — run the REST API secret background steps first")
	}
	w.restAPISecretRotatedValue = "e2e-test-restapi-rotated-" + randHex()
	return rotateSecret(w.restAPISecretHandle, w.restAPISecretRotatedValue)
}

// theGatewaysLocalSecretHasTheRotatedValue polls the gateway-controller's own
// secret store until it reflects the rotated value, confirming secret.updated
// triggered an immediate re-fetch rather than waiting for the next reconnect.
func (w *world) theGatewaysLocalSecretHasTheRotatedValue() error {
	if w.restAPISecretRotatedValue == "" {
		return fmt.Errorf("no rotated value recorded — run 'I rotate the secret to a new value' first")
	}
	return waitGatewaySecretValue(w.restAPISecretHandle, w.restAPISecretRotatedValue, pollTimeout)
}

// iUpdateTheRestAPIToReferenceADifferentSecret creates a second secret, updates the
// REST API's upstream auth to reference it instead of the original one, redeploys so
// the gateway's deployed snapshot stops referencing the original handle too, then
// explicitly deletes the now-fully-unreferenced original secret.
func (w *world) iUpdateTheRestAPIToReferenceADifferentSecret() error {
	if w.restAPISecretApiID == "" {
		return fmt.Errorf("no REST API — run the REST API secret background steps first")
	}

	replacementHandle, err := createSecret("E2E REST API Replacement Credential", "e2e-test-restapi-value-"+randHex())
	if err != nil {
		return err
	}

	// Reconstruct the exact displayName aRestAPIReferencingSecret used, so this
	// full-replace PUT doesn't incidentally change anything but the auth value.
	suffix := strings.TrimPrefix(w.restAPISecretContext, "/e2e-secret-")
	displayName := "e2e-secret-api-" + suffix

	st, body, err := apiCall(http.MethodPut, "/rest-apis/"+w.restAPISecretApiID, suite.token, map[string]any{
		"displayName": displayName,
		"context":     w.restAPISecretContext,
		"version":     "v1",
		"projectId":   suite.projectID,
		"upstream": map[string]any{
			"main": map[string]any{
				"url": "http://sample-backend:9080",
				"auth": map[string]any{
					"type":   "api-key",
					"header": "Authorization",
					"value":  `{{ secret "` + replacementHandle + `" }}`,
				},
			},
		},
	})
	if err != nil {
		return err
	}
	if st >= 300 {
		return fmt.Errorf("update REST API to swap secret failed (%d): %s", st, body)
	}

	// Redeploy the updated config so the gateway's deployed snapshot (and therefore
	// its gateway-scoped artifact_secret_refs row) picks up the replacement handle,
	// freeing the original one from every reference — current config and deployed
	// snapshot alike — before we try to delete it.
	if _, err := deployRestAPIWithoutRestart(w.restAPISecretApiID, suite.gw1ID); err != nil {
		return fmt.Errorf("redeploy after secret swap failed: %w", err)
	}

	return deleteSecret(w.restAPISecretHandle)
}

// theGatewayEvictsTheOriginalSecretFromItsLocalStore polls the gateway-controller's
// secret store until the original (now-unreferenced, permanently deleted) secret is
// gone, confirming the secret.deleted push event triggered eviction.
func (w *world) theGatewayEvictsTheOriginalSecretFromItsLocalStore() error {
	return waitGatewaySecretGone(w.restAPISecretHandle, pollTimeout)
}
