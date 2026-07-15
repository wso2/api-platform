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
 *  KIND, either express or implied. See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

// This file adds the "secured API" scenario: a PUBLISHED REST API protected by
// the api-key-auth and subscription-validation policies is deployed to the
// gateway and then invoked through the data plane, proving that the full chain
// (publish -> deploy -> subscription plan -> application -> subscription ->
// API key -> authenticated invocation) works end to end against the real
// platform-api and the real gateway.
//
// Credential value flow (verified from source):
//   - api-key-auth validates the plaintext value of the `API-Key` header by
//     hashing it and matching the hash stored for the API. The caller CHOOSES
//     the plaintext when creating the key (POST /rest-apis/{id}/api-keys with
//     {displayName, apiKey}); platform-api stores only the hash, so the test
//     keeps the plaintext to present at the ingress.
//   - subscription-validation validates the raw `Subscription-Key` header
//     against the subscriptionToken minted by POST /subscriptions.
//
// Ordering rule (verified): the API must be DEPLOYED to the gateway before the
// subscription / API key are created, because platform-api only broadcasts the
// subscription.created / apikey.created events to gateways where the artifact is
// already deployed (and POST .../api-keys returns 503 when no gateway is
// connected). Unlike deployments, these events are pushed live over the
// control-plane WebSocket and applied immediately, so NO controller restart is
// needed after them — we just poll the ingress until they propagate.

const (
	apiKeyHeader = "API-Key"          // api-key-auth policy default header
	subKeyHeader = "Subscription-Key" // subscription-validation policy default header
)

func (w *world) registerSecuredSteps(sc *godog.ScenarioContext) {
	sc.Step(`^a subscription plan "([^"]*)" allowing (\d+) requests per (minute|hour|day|month)$`, w.aSubscriptionPlan)
	sc.Step(`^a published REST API secured with API key and subscription validation offering that plan$`, w.aSecuredRestAPI)
	sc.Step(`^I deploy the secured API to the gateway$`, w.deployToGateway)
	sc.Step(`^an unauthenticated request to the secured API is rejected$`, w.unauthenticatedRequestRejected)
	sc.Step(`^an application is subscribed to the API under that plan$`, w.applicationSubscribed)
	sc.Step(`^an API key is issued for the API$`, w.apiKeyIssued)
	sc.Step(`^invoking the secured API through the gateway with valid credentials returns 200$`, w.invokeWithCredentialsSucceeds)
	sc.Step(`^invoking the secured API through the gateway without credentials is rejected$`, w.invokeWithoutCredentialsRejected)
}

// --- Given -----------------------------------------------------------------

// aSubscriptionPlan creates an ACTIVE, org-scoped subscription plan. The handle
// is made unique per run so a kept stack (E2E_KEEP=1) can be re-run without a
// 409; the quoted name in the feature is used as the display name.
func (w *world) aSubscriptionPlan(name string, count int, unit string) error {
	handle := strings.ToLower(name) + "-" + randHex()
	// displayName must be unique too: the gateway stores plans keyed by
	// (gateway_id, plan_name) where plan_name is this displayName, so two plans with
	// the same display name on one gateway collide (and the later one — with its
	// subscription — silently fails to sync). Use the unique handle as the name.
	st, body, err := apiCall(http.MethodPost, "/subscription-plans", suite.token, map[string]any{
		"id":          handle,
		"displayName": handle,
		"status":      "ACTIVE",
		"limits": []map[string]any{
			{"limitType": "REQUEST_COUNT", "timeUnit": strings.ToUpper(unit), "limitCount": count},
		},
	})
	if err != nil {
		return err
	}
	if st >= 300 {
		return fmt.Errorf("create subscription plan failed (%d): %s", st, body)
	}
	// The response echoes the plan handle as "id".
	if id := jsonField(body, "id"); id != "" {
		handle = id
	}
	w.planID = handle
	return nil
}

// aSecuredRestAPI creates a PUBLISHED REST API that offers the plan created
// above and is guarded by api-key-auth + subscription-validation. Publishing is
// done inline via lifeCycleStatus (there is no separate lifecycle endpoint).
func (w *world) aSecuredRestAPI() error {
	if w.planID == "" {
		return fmt.Errorf("subscription plan was not created before the secured API")
	}
	suffix := randHex()
	w.apiContext = "/e2e-sec-" + suffix
	st, body, err := apiCall(http.MethodPost, "/rest-apis", suite.token, map[string]any{
		"displayName":       "e2e-secured-" + suffix,
		"context":           w.apiContext,
		"version":           "v1",
		"projectId":         suite.projectID,
		"lifeCycleStatus":   "PUBLISHED",
		"subscriptionPlans": []string{w.planID},
		"upstream":          map[string]any{"main": map[string]any{"url": "http://sample-backend:9080"}},
		"policies": []map[string]any{
			{"name": "api-key-auth", "version": "v1", "params": map[string]any{"key": apiKeyHeader, "in": "header"}},
			{"name": "subscription-validation", "version": "v1", "params": map[string]any{"subscriptionKeyHeader": subKeyHeader}},
		},
	})
	if err != nil {
		return err
	}
	w.apiID = jsonField(body, "id")
	if st >= 300 || w.apiID == "" {
		return fmt.Errorf("create secured API failed (%d): %s", st, body)
	}
	// Keep the plaintext key for the ingress call; platform-api stores only its hash.
	w.apiKey = "e2e" + randHex() + randHex() + randHex() + randHex()
	return nil
}

// --- Then / When: readiness, subscription, key, invocation -----------------

// unauthenticatedRequestRejected doubles as the deployment-readiness gate: it
// waits until the ingress stops returning 404 (route not yet programmed) and
// starts returning 401/403 (route active and enforcing the auth policies).
func (w *world) unauthenticatedRequestRejected() error {
	return waitIngressRejected(ingressGw1, w.apiContext)
}

func (w *world) applicationSubscribed() error {
	appID, err := createApplication("e2e-app-"+randHex(), suite.projectID)
	if err != nil {
		return err
	}
	w.appID = appID
	token, err := createSubscription(w.apiID, w.appID, w.planID)
	if err != nil {
		return err
	}
	w.subToken = token
	return nil
}

func (w *world) apiKeyIssued() error {
	return createAPIKey(w.apiID, w.apiKey)
}

func (w *world) invokeWithCredentialsSucceeds() error {
	headers := map[string]string{apiKeyHeader: w.apiKey, subKeyHeader: w.subToken}
	return waitIngressWithHeaders(ingressGw1, w.apiContext, headers, 200)
}

func (w *world) invokeWithoutCredentialsRejected() error {
	if code := ingressStatusWithHeaders(ingressGw1, w.apiContext, nil); code != 401 && code != 403 {
		return fmt.Errorf("unauthenticated request should be rejected (401/403), got %d", code)
	}
	return nil
}

// --- platform-api REST helpers ---------------------------------------------

func createApplication(name, projectID string) (string, error) {
	st, body, err := apiCall(http.MethodPost, "/applications", suite.token, map[string]any{
		"id":          name,
		"displayName": name,
		"projectId":   projectID,
		"type":        "genai", // the only ApplicationType value
	})
	if err != nil {
		return "", err
	}
	id := jsonField(body, "id", "handle", "uuid")
	if st >= 300 || id == "" {
		return "", fmt.Errorf("create application failed (%d): %s", st, body)
	}
	return id, nil
}

func createSubscription(apiID, appID, planID string) (string, error) {
	st, body, err := apiCall(http.MethodPost, "/subscriptions", suite.token, map[string]any{
		"artifactId":         apiID,
		"kind":               "RestApi",
		"subscriberId":       "e2e-subscriber",
		"applicationId":      appID,
		"subscriptionPlanId": planID,
	})
	if err != nil {
		return "", err
	}
	// The raw token is returned only on creation; it is the Subscription-Key value.
	token := jsonField(body, "subscriptionToken")
	if st >= 300 || token == "" {
		return "", fmt.Errorf("create subscription failed (%d): %s", st, body)
	}
	return token, nil
}

// createAPIKey supplies the plaintext key value to platform-api, which hashes
// and broadcasts it to the gateways where the API is deployed. The response
// carries only status/keyId, never the secret, so the caller must retain the
// value it supplied.
func createAPIKey(apiID, keyValue string) error {
	st, body, err := apiCall(http.MethodPost, "/rest-apis/"+apiID+"/api-keys", suite.token, map[string]any{
		"displayName": "e2e-key",
		"apiKey":      keyValue,
	})
	if err != nil {
		return err
	}
	if st >= 300 {
		return fmt.Errorf("create API key failed (%d): %s", st, body)
	}
	return nil
}

// --- ingress helpers -------------------------------------------------------

// ingressStatusWithHeaders is ingressStatus with optional request headers (the
// API-Key / Subscription-Key credentials).
func ingressStatusWithHeaders(base, context string, headers map[string]string) int {
	req, err := http.NewRequest(http.MethodGet, base+context+"/", nil)
	if err != nil {
		return -1
	}
	req.Host = ingressHost // gateway routes by vhost
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0
	}
	resp.Body.Close()
	return resp.StatusCode
}

// waitIngressWithHeaders polls the ingress with the given headers until it
// returns want (credentials propagate to the data plane asynchronously).
func waitIngressWithHeaders(base, context string, headers map[string]string, want int) error {
	deadline := time.Now().Add(pollTimeout)
	var last int
	for time.Now().Before(deadline) {
		if last = ingressStatusWithHeaders(base, context, headers); last == want {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("ingress %s%s with credentials: wanted %d, last observed %d", base, context, want, last)
}

// waitIngressRejected polls until the route is active and enforcing auth, i.e.
// it returns 401 or 403 (not 404, which means the route is not programmed yet).
func waitIngressRejected(base, context string) error {
	deadline := time.Now().Add(pollTimeout)
	var last int
	for time.Now().Before(deadline) {
		if last = ingressStatus(base, context); last == 401 || last == 403 {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("ingress %s%s: wanted 401/403 (route active, unauthenticated), last observed %d", base, context, last)
}
