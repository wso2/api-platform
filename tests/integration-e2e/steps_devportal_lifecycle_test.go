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
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cucumber/godog"
)

// This file extends the @devportal scenario with credential-lifecycle changes made
// in the developer portal, each verified either at the gateway (invocation) or on
// the platform-api (control-plane) side, all via the signed-webhook propagation.
// The scenario runs them in two groups:
//
//   API key lifecycle (the subscription stays ACTIVE throughout):
//     - change expiry  -> gateway rejects the (now-expired) key, then serves after restore
//     - revoke         -> gateway returns 401, then a new key is issued and serves again
//
//   Subscription lifecycle (the API key stays valid throughout):
//     - change plan             -> verify the new plan via platform-api REST
//     - regenerate token        -> new token works at the gateway, old is rejected
//     - pause (INACTIVE)/resume -> gateway rejects then serves
//     - remove                  -> gateway returns 403 (terminal)
//
// Isolation: the gateway distinguishes a bad/revoked/expired key (401, api-key-auth)
// from a bad/absent/inactive subscription (403, subscription-validation). Grouping
// the checks — all key checks while the subscription is active, all subscription
// checks with a valid key — means each rejection's status code identifies the cause.
//
// Expiry note: platform-api exposes NO REST endpoint that returns a webhook-created
// key's expiry — /me/api-keys is filtered to the caller's own keys (webhook keys have
// no owner) and /applications/{appId}/api-keys requires a key→app mapping that neither
// the apikey.application_updated webhook nor the direct AddApplicationAPIKeys REST can
// establish (both fail to resolve the key). So the expiry change is verified at the
// gateway instead: setting a past expiry must make the data plane reject the key.

const (
	keyExpiryPast   = "2020-01-01T00:00:00Z" // past → the gateway must reject the key
	keyExpiryFuture = "2035-01-01T00:00:00Z" // future → restores validity for later checks
)

func (w *world) registerDevportalLifecycleSteps(sc *godog.ScenarioContext) {
	sc.Step(`^a second subscription plan is synced to the developer portal$`, w.syncSecondPlan)
	sc.Step(`^the API key is expired in the developer portal$`, w.expireKey)
	sc.Step(`^invoking with the expired API key is rejected$`, w.invokeExpiredKeyRejected)
	sc.Step(`^the API key expiry is restored in the developer portal$`, w.restoreKeyExpiry)
	sc.Step(`^the applied subscription plan of the API is switched in the developer portal$`, w.changeSubPlan)
	sc.Step(`^platform-api receives the new subscription plan update of the API$`, w.verifySubPlan)
	// The token-regeneration triple: the current token works, then after regen the new
	// token works and the old one is rejected. The two 200-checks reuse the shared
	// invokeWithCredentialsSucceeds (invokes with the current w.apiKey + w.subToken).
	sc.Step(`^invoking with the current subscription token returns 200$`, w.invokeWithCredentialsSucceeds)
	sc.Step(`^the subscription token is regenerated in the developer portal$`, w.regenerateSubToken)
	sc.Step(`^invoking with the new subscription token returns 200$`, w.invokeWithCredentialsSucceeds)
	sc.Step(`^invoking with the old subscription token is rejected$`, w.invokeOldTokenRejected)
	sc.Step(`^the subscription is paused in the developer portal$`, w.pauseSubscription)
	sc.Step(`^the subscription is resumed in the developer portal$`, w.resumeSubscription)
	sc.Step(`^invoking the secured API through the gateway is rejected$`, w.invokeRejected)
	sc.Step(`^the API key is revoked in the developer portal$`, w.revokeKey)
	sc.Step(`^invoking with the revoked API key is unauthorized$`, w.invokeRevokedKeyUnauthorized)
	sc.Step(`^a new API key is generated in the developer portal$`, w.generateKeyInDevportal)
	sc.Step(`^the subscription is removed in the developer portal$`, w.removeSubscription)
}

// --- 2. API key expiry --------------------------------------------------------

// regenerateKeyExpiry regenerates the key with a new expiry. Regenerate mints a new
// plaintext secret too, so the current key value is updated. Fires apikey.regenerated,
// which platform-api applies (new expiry + hash) and broadcasts to the gateway.
func (w *world) regenerateKeyExpiry(expiresAt string) error {
	st, body, err := dpCall(http.MethodPost, "/apis/"+w.dpApiID+"/api-keys/regenerate", map[string]any{
		"keyId":     w.dpKeyHandle,
		"expiresAt": expiresAt,
	})
	if err != nil {
		return err
	}
	newKey := jsonField(body, "key")
	if st >= 300 || newKey == "" {
		return fmt.Errorf("regenerate devportal key failed (%d): %s", st, body)
	}
	w.apiKey = newKey // the previous secret is now invalid
	return nil
}

func (w *world) expireKey() error        { return w.regenerateKeyExpiry(keyExpiryPast) }
func (w *world) restoreKeyExpiry() error { return w.regenerateKeyExpiry(keyExpiryFuture) }

// invokeExpiredKeyRejected polls until the gateway rejects the expired key with 401.
// The subscription is still active, so the failure isolates the key's expiry.
func (w *world) invokeExpiredKeyRejected() error {
	headers := map[string]string{apiKeyHeader: w.apiKey, subKeyHeader: w.subToken}
	return waitIngressWithHeaders(ingressGw1, w.apiContext, headers, 401)
}

// --- 3. subscription plan change ---------------------------------------------

// syncSecondPlan creates a second plan in platform-api (ACTIVE) and mirrors it into
// the developer portal. It must run before the API is published so the API can offer
// it (publishAPIToDevportal includes plan2ID), which the change-plan below requires.
func (w *world) syncSecondPlan() error {
	w.plan2ID = "e2e-silver-" + randHex()
	// displayName is unique (= handle) to avoid the gateway's (gateway_id, plan_name)
	// collision; verifySubPlan asserts against this same value.
	if st, body, err := apiCall(http.MethodPost, "/subscription-plans", suite.token, map[string]any{
		"id":          w.plan2ID,
		"displayName": w.plan2ID,
		"status":      "ACTIVE",
		"limits":      []map[string]any{{"limitType": "REQUEST_COUNT", "timeUnit": "HOUR", "limitCount": 5000}},
	}); err != nil {
		return err
	} else if st >= 300 {
		return fmt.Errorf("create second plan failed (%d): %s", st, body)
	}
	return syncDevportalPlan(w.plan2ID) // refId = platform-api plan handle
}

// changeSubPlan switches the subscription to the second plan (already offered by the
// API). The devportal field is planId, not subscriptionPlanId.
func (w *world) changeSubPlan() error {
	st, body, err := dpCall(http.MethodPost, "/subscriptions/"+w.dpSubID+"/change-plan", map[string]any{
		"planId": w.plan2ID,
	})
	if err != nil {
		return err
	}
	if st >= 300 {
		return fmt.Errorf("change devportal plan failed (%d): %s", st, body)
	}
	return nil
}

// verifySubPlan polls platform-api until the subscription reports the new plan.
// The subscriptionPlanId in the response is the plan's UUID, so the plan is
// identified by its display name instead.
func (w *world) verifySubPlan() error {
	deadline := time.Now().Add(pollTimeout)
	var last string
	for time.Now().Before(deadline) {
		st, body, err := apiCall(http.MethodGet, "/subscriptions?artifactId="+w.apiID, suite.token, nil)
		if err == nil && st == http.StatusOK {
			var env struct {
				List []map[string]any `json:"list"`
			}
			if json.Unmarshal(body, &env) == nil && len(env.List) > 0 {
				last = stringField(env.List[0], "subscriptionPlanName")
				if last == w.plan2ID {
					return nil
				}
			}
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("platform-api did not report the new subscription plan %q (last observed %q)", w.plan2ID, last)
}

// --- 6. subscription token regeneration --------------------------------------

func (w *world) regenerateSubToken() error {
	w.prevSubToken = w.subToken
	st, body, err := dpCall(http.MethodPost, "/subscriptions/"+w.dpSubID+"/regenerate-token", nil)
	if err != nil {
		return err
	}
	w.subToken = jsonField(body, "subscriptionToken")
	if st >= 300 || w.subToken == "" || w.subToken == w.prevSubToken {
		return fmt.Errorf("regenerate devportal token failed (%d): %s", st, body)
	}
	return nil
}

func (w *world) invokeOldTokenRejected() error {
	headers := map[string]string{apiKeyHeader: w.apiKey, subKeyHeader: w.prevSubToken}
	// The new token must have propagated (asserted by the preceding 200 step), so the
	// old token should now be rejected immediately.
	if code := ingressStatusWithHeaders(ingressGw1, w.apiContext, headers); code != 401 && code != 403 {
		return fmt.Errorf("old subscription token should be rejected, got %d", code)
	}
	return nil
}

// --- 4. pause / resume subscription ------------------------------------------

func (w *world) pauseSubscription() error  { return w.setSubStatus("INACTIVE") }
func (w *world) resumeSubscription() error { return w.setSubStatus("ACTIVE") }

func (w *world) setSubStatus(status string) error {
	st, body, err := dpCall(http.MethodPut, "/subscriptions/"+w.dpSubID, map[string]any{
		"status": status,
	})
	if err != nil {
		return err
	}
	if st >= 300 {
		return fmt.Errorf("set devportal subscription status %s failed (%d): %s", status, st, body)
	}
	return nil
}

// invokeRejected polls until the gateway rejects the current credentials with 403
// (subscription-level authorization failure — used for pause and delete).
func (w *world) invokeRejected() error {
	headers := map[string]string{apiKeyHeader: w.apiKey, subKeyHeader: w.subToken}
	return waitIngressWithHeaders(ingressGw1, w.apiContext, headers, 403)
}

// --- 1. API key revocation ---------------------------------------------------

func (w *world) revokeKey() error {
	st, body, err := dpCall(http.MethodPost, "/apis/"+w.dpApiID+"/api-keys/revoke", map[string]any{
		"keyId": w.dpKeyHandle,
	})
	if err != nil {
		return err
	}
	if st >= 300 {
		return fmt.Errorf("revoke devportal key failed (%d): %s", st, body)
	}
	return nil
}

// invokeRevokedKeyUnauthorized polls until the gateway rejects the revoked key with
// 401. The subscription is still active, so the failure isolates the key.
func (w *world) invokeRevokedKeyUnauthorized() error {
	headers := map[string]string{apiKeyHeader: w.apiKey, subKeyHeader: w.subToken}
	return waitIngressWithHeaders(ingressGw1, w.apiContext, headers, 401)
}

// --- 5. subscription removal -------------------------------------------------

func (w *world) removeSubscription() error {
	st, body, err := dpCall(http.MethodDelete, "/subscriptions/"+w.dpSubID, nil)
	if err != nil {
		return err
	}
	if st >= 300 {
		return fmt.Errorf("delete devportal subscription failed (%d): %s", st, body)
	}
	return nil
}

// --- helpers -----------------------------------------------------------------

// syncDevportalPlan upserts a devportal plan whose refId equals the platform-api
// plan handle (so subscription events carry a resolvable subscription_plan.ref_id).
func syncDevportalPlan(handle string) error {
	st, body, err := dpCall(http.MethodPut, "/subscription-plans", []map[string]any{
		{
			"id":          handle,
			"displayName": handle,
			"refId":       handle,
			"limits": []map[string]any{
				{"limitType": "REQUEST_COUNT", "limitCount": 5000, "timeUnit": "HOUR", "timeAmount": 1},
			},
		},
	})
	if err != nil {
		return err
	}
	if st >= 300 {
		return fmt.Errorf("sync devportal plan %q failed (%d): %s", handle, st, body)
	}
	return nil
}

// stringField reads a string value from a decoded JSON object.
func stringField(m map[string]any, key string) string {
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}
