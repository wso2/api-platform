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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"time"

	"github.com/cucumber/godog"
)

// This file adds the full-product-suite scenario: an API key and a subscription
// are created in the DEVELOPER PORTAL, which fires signed webhooks to
// platform-api; platform-api decrypts them, persists the credentials, and
// propagates them to the gateway over its control-plane WebSocket. The API is
// then invoked through the gateway using those developer-portal-issued
// credentials.
//
// Trust/transport model (verified from source):
//   - The devportal signs each webhook "t=<unix>,v1=<hmac>" over "<t>.<body>"
//     (X-Devportal-Signature) with the shared secret, and encrypts the key/token
//     with the platform-api webhook RSA public key (RSA-OAEP-SHA256 + AES-256-GCM).
//   - platform-api resolves the event's org by HANDLE (org.ref_id) and the API /
//     plan by HANDLE (data.api.ref_id / data.subscription_plan.ref_id). So the
//     devportal org's cpRefId must equal the platform-api org handle ("default"),
//     the devportal API's referenceId must equal the platform-api API handle, and
//     the devportal plan's refId must equal the platform-api plan handle.
//   - Delivery is fire-once on a ~2s poll, and the plaintext key/token the portal
//     returns to the user are exactly what the gateway validates.
//
// The devportal accepts the platform-api admin JWT directly (it verifies it with
// the shared APIP_DP_PLATFORMAPI_JWTSECRET and takes the org from the token's
// org_handle claim), so suite.token is reused for every call here.

// webhookReceiverURL is the platform-api webhook receiver at its container-internal
// host (the devportal reaches platform-api by service name on the compose network,
// not via the host-published port). Path from webhookReceiverPath (a var, so this
// is a var too).
var webhookReceiverURL = "https://platform-api:9243" + webhookReceiverPath

// The devportal org handle seeded via APIP_DP_ORGANIZATION_DEFAULTNAME; must match the platform-api
// org handle so org.ref_id resolves.
const devportalOrgHandle = "default"

func (w *world) registerDevportalSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the subscription plan is synced to the developer portal$`, w.syncPlanToDevportal)
	sc.Step(`^the API is published to the developer portal linked to the platform API$`, w.publishAPIToDevportal)
	sc.Step(`^an application subscribed to the API is created in the developer portal$`, w.subscribeInDevportal)
	sc.Step(`^an API key is generated in the developer portal$`, w.generateKeyInDevportal)
	// The invocation assertions are shared with the platform-api-driven scenario.
	sc.Step(`^invoking the secured API through the gateway with the developer portal credentials returns 200$`, w.invokeWithCredentialsSucceeds)

	// Credential-lifecycle steps (revoke / expiry / plan / pause / delete / token regen).
	w.registerDevportalLifecycleSteps(sc)
}

// --- suite bootstrap (called from bringUpStack) ----------------------------

// bootstrapDevportal prepares the running developer portal so its webhooks are
// accepted by platform-api: it links the portal org to the control-plane org
// handle and registers the platform-api webhook subscriber.
func bootstrapDevportal() error {
	if err := waitDevportalHealthy(); err != nil {
		return err
	}
	if err := linkDevportalOrg(); err != nil {
		return err
	}
	return registerWebhookSubscriber()
}

func waitDevportalHealthy() error {
	deadline := time.Now().Add(pollTimeout * 2)
	var lastStatus int
	var lastErr error
	for time.Now().Before(deadline) {
		st, _, err := dpCall(http.MethodGet, "/organizations", nil)
		if err == nil && st == http.StatusOK {
			return nil
		}
		lastStatus, lastErr = st, err
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("developer portal did not become healthy (last status %d, err %v)", lastStatus, lastErr)
}

// linkDevportalOrg sets the portal org's cpRefId to the platform-api org handle
// so outbound events carry a resolvable org.ref_id. displayName/idpRefId are read
// back first so the update does not clobber them.
func linkDevportalOrg() error {
	st, body, err := dpCall(http.MethodGet, "/organizations/"+devportalOrgHandle, nil)
	if err != nil {
		return err
	}
	if st >= 300 {
		return fmt.Errorf("get devportal org failed (%d): %s", st, body)
	}
	displayName := jsonField(body, "displayName")
	if displayName == "" {
		displayName = "Default"
	}
	update := map[string]any{
		"id":          devportalOrgHandle,
		"displayName": displayName,
		"cpRefId":     devportalOrgHandle, // == platform-api org handle
	}
	if idp := jsonField(body, "idpRefId"); idp != "" {
		update["idpRefId"] = idp
	}
	st, body, err = dpCall(http.MethodPut, "/organizations/"+devportalOrgHandle, update)
	if err != nil {
		return err
	}
	if st >= 300 {
		return fmt.Errorf("link devportal org failed (%d): %s", st, body)
	}
	return nil
}

// registerWebhookSubscriber points the portal at the platform-api receiver with
// the shared HMAC secret and the run's generated RSA public key (see
// prepareWebhookKey). Idempotent: a repeat registration (E2E_KEEP reruns) that
// conflicts is treated as success.
func registerWebhookSubscriber() error {
	if webhookPublicKeyPEM == "" {
		return fmt.Errorf("webhook public key not generated (prepareWebhookKey must run first)")
	}
	st, body, err := dpCall(http.MethodPost, "/webhook-subscribers", map[string]any{
		"id":          "platform-api",
		"displayName": "Platform API",
		"targetUrl":   webhookReceiverURL,
		"secret":      webhookSecret,
		"publicKey":   webhookPublicKeyPEM,
		"events":      []string{"apikey.*", "subscription.*"},
		"enabled":     true,
	})
	if err != nil {
		return err
	}
	if st == http.StatusConflict {
		return nil // already registered on a kept stack
	}
	if st >= 300 {
		return fmt.Errorf("register webhook subscriber failed (%d): %s", st, body)
	}
	return nil
}

// --- scenario steps --------------------------------------------------------

// syncPlanToDevportal upserts a devportal plan whose refId equals the platform-api
// plan handle, so subscription.created carries a resolvable subscription_plan.ref_id.
func (w *world) syncPlanToDevportal() error {
	if w.planID == "" {
		return fmt.Errorf("no platform-api plan to sync")
	}
	st, body, err := dpCall(http.MethodPut, "/subscription-plans", []map[string]any{
		{
			"id":          w.planID, // devportal plan handle
			"displayName": w.planID,
			"refId":       w.planID, // link to the platform-api plan handle
			"limits": []map[string]any{
				{"limitType": "REQUEST_COUNT", "limitCount": 10000, "timeUnit": "HOUR", "timeAmount": 1},
			},
		},
	})
	if err != nil {
		return err
	}
	if st >= 300 {
		return fmt.Errorf("sync devportal plan failed (%d): %s", st, body)
	}
	return nil
}

// publishAPIToDevportal publishes the API into the portal with referenceId set to
// the platform-api API handle (lowercase `referenceId` — the parser ignores the
// `referenceID` spelling used by the shipped samples) so credential webhooks map
// back to the deployed API.
func (w *world) publishAPIToDevportal() error {
	if w.apiID == "" {
		return fmt.Errorf("no platform-api API to publish")
	}
	name := "dp-" + w.apiID // devportal API handle (metadata.name)
	// Offer the second plan too when the lifecycle scenario has pre-created it, so a
	// later change-plan (which only allows plans the API offers) succeeds.
	plans := []string{w.planID}
	if w.plan2ID != "" {
		plans = append(plans, w.plan2ID)
	}
	ct, payload, err := devportalAPIMultipart(name, w.apiID, plans)
	if err != nil {
		return err
	}
	st, body, err := dpDo(http.MethodPost, "/apis", ct, payload)
	if err != nil {
		return err
	}
	w.dpApiID = jsonField(body, "id")
	if st >= 300 || w.dpApiID == "" {
		return fmt.Errorf("publish API to devportal failed (%d): %s", st, body)
	}
	return nil
}

// devportalAPIMultipart builds the multipart body (api.yaml + a minimal OpenAPI
// definition) used to create or update a developer-portal API. referenceId is set
// to the platform-api API handle so credential webhooks map back to the deployed
// API, and subscriptionPlans lists the plan handles the API offers.
func devportalAPIMultipart(name, refID string, plans []string) (string, []byte, error) {
	plansYAML := ""
	for _, p := range plans {
		plansYAML += "\n    - " + p
	}
	apiYAML := fmt.Sprintf(`apiVersion: devportal.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: %s
spec:
  type: REST
  displayName: %s
  version: v1
  status: PUBLISHED
  referenceId: %s
  subscriptionPlans:%s
  endpoints:
    sandboxUrl: http://sample-backend:9080
    productionUrl: http://sample-backend:9080
`, name, name, refID, plansYAML)

	defYAML := fmt.Sprintf(`openapi: 3.0.1
info:
  title: %s
  version: v1
paths:
  /:
    get:
      responses:
        '200':
          description: OK
`, name)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := addYAMLPart(mw, "metadata", "metadata.yaml", apiYAML); err != nil {
		return "", nil, err
	}
	if err := addYAMLPart(mw, "definition", "definition.yaml", defYAML); err != nil {
		return "", nil, err
	}
	if err := mw.Close(); err != nil {
		return "", nil, err
	}
	return mw.FormDataContentType(), buf.Bytes(), nil
}

// subscribeInDevportal creates an application and subscribes it to the API under
// the synced plan. The returned subscriptionToken (plaintext) is the gateway
// Subscription-Key value; creating the subscription fires subscription.created.
func (w *world) subscribeInDevportal() error {
	appName := "e2e-app-" + randHex()
	st, body, err := dpCall(http.MethodPost, "/applications", map[string]any{
		"id":          appName,
		"displayName": appName,
		"description": "e2e application",
	})
	if err != nil {
		return err
	}
	w.appID = jsonField(body, "id")
	if st >= 300 || w.appID == "" {
		return fmt.Errorf("create devportal application failed (%d): %s", st, body)
	}

	st, body, err = dpCall(http.MethodPost, "/subscriptions", map[string]any{
		"artifactId":         w.dpApiID, // the devportal API handle, not the referenceId
		"subscriptionPlanId": w.planID,
	})
	if err != nil {
		return err
	}
	w.subToken = jsonField(body, "subscriptionToken")
	w.dpSubID = jsonField(body, "subscriptionId")
	if st >= 300 || w.subToken == "" || w.dpSubID == "" {
		return fmt.Errorf("create devportal subscription failed (%d): %s", st, body)
	}
	return nil
}

// generateKeyInDevportal generates an API key in the portal; the one-time plaintext
// `key` is the gateway API-Key value, and generation fires apikey.generated. The
// key handle is captured for later lifecycle operations (revoke/regenerate/associate).
func (w *world) generateKeyInDevportal() error {
	handle := "e2ekey-" + randHex()
	st, body, err := dpCall(http.MethodPost, "/apis/"+w.dpApiID+"/api-keys/generate", map[string]any{
		"id":          handle,
		"displayName": "e2e key",
	})
	if err != nil {
		return err
	}
	w.apiKey = jsonField(body, "key")
	w.dpKeyHandle = jsonField(body, "id")
	if w.dpKeyHandle == "" {
		w.dpKeyHandle = handle
	}
	if st >= 300 || w.apiKey == "" {
		return fmt.Errorf("generate devportal API key failed (%d): %s", st, body)
	}
	return nil
}

// --- devportal HTTP helpers ------------------------------------------------

// dpCall performs a JSON request against the developer portal with the admin
// bearer token.
func dpCall(method, path string, body any) (int, []byte, error) {
	var payload []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		payload = b
	}
	return dpDo(method, path, "application/json", payload)
}

// dpDo performs a request against the developer portal with the admin bearer token
// and an explicit content type (used for both JSON and multipart bodies). path is
// the resource path relative to devportalBase (e.g. "/apis"), which is prepended.
func dpDo(method, path, contentType string, body []byte) (int, []byte, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, devportalAPI+devportalBase+path, rdr)
	if err != nil {
		return 0, nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	// Bearer auth with the platform-api admin JWT: the devportal verifies it with
	// the shared secret, takes the org from its org_handle claim, and (unlike
	// API-key mode) resolves a user identity — required to persist created_by on
	// applications/subscriptions/keys.
	if suite.token != "" {
		req.Header.Set("Authorization", "Bearer "+suite.token)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out, nil
}

// addYAMLPart writes a named YAML file part into a multipart writer.
func addYAMLPart(mw *multipart.Writer, field, filename, content string) error {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, field, filename))
	h.Set("Content-Type", "application/yaml")
	part, err := mw.CreatePart(h)
	if err != nil {
		return err
	}
	_, err = part.Write([]byte(content))
	return err
}
