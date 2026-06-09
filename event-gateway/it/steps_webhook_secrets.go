// --------------------------------------------------------------------
// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
// --------------------------------------------------------------------

package it

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cucumber/godog"
)

// WebhookSecretSteps holds per-scenario state for webhook secret step definitions.
type WebhookSecretSteps struct {
	lastSecretName  string
	lastSecretValue string
}

// Reset clears per-scenario webhook secret state.
func (w *WebhookSecretSteps) Reset() {
	w.lastSecretName = ""
	w.lastSecretValue = ""
}

// RegisterWebhookSecretSteps registers all webhook secret management step definitions.
func RegisterWebhookSecretSteps(ctx *godog.ScenarioContext, state *TestState, ws *WebhookSecretSteps) {
	ctx.Step(`^I create a webhook secret with display name "([^"]*)" for WebSub API "([^"]*)"$`,
		func(displayName, apiName string) error {
			body := fmt.Sprintf(`{"displayName":%q}`, displayName)
			err := doRequest(state, http.MethodPost,
				state.Config.GatewayControllerURL+"/websub-apis/"+apiName+"/secrets",
				strings.NewReader(body), "application/json")
			if err != nil {
				return err
			}
			if state.lastResponse != nil && state.lastResponse.StatusCode == http.StatusCreated {
				ws.lastSecretName, ws.lastSecretValue = parseWebhookSecretResponse(state.lastBody)
			}
			return nil
		})

	ctx.Step(`^I list webhook secrets for WebSub API "([^"]*)"$`,
		func(apiName string) error {
			return doRequest(state, http.MethodGet,
				state.Config.GatewayControllerURL+"/websub-apis/"+apiName+"/secrets",
				nil, "")
		})

	// Uses the secret name captured from the most recent create call.
	ctx.Step(`^I regenerate the saved webhook secret for WebSub API "([^"]*)"$`,
		func(apiName string) error {
			if ws.lastSecretName == "" {
				return fmt.Errorf("no saved webhook secret name; call 'create a webhook secret' first")
			}
			err := doRequest(state, http.MethodPost,
				state.Config.GatewayControllerURL+"/websub-apis/"+apiName+"/secrets/"+ws.lastSecretName+"/regenerate",
				nil, "")
			if err != nil {
				return err
			}
			if state.lastResponse != nil && state.lastResponse.StatusCode == http.StatusOK {
				_, ws.lastSecretValue = parseWebhookSecretResponse(state.lastBody)
			}
			return nil
		})

	// Uses an explicitly named secret (for error-case tests).
	ctx.Step(`^I regenerate webhook secret "([^"]*)" for WebSub API "([^"]*)"$`,
		func(secretName, apiName string) error {
			return doRequest(state, http.MethodPost,
				state.Config.GatewayControllerURL+"/websub-apis/"+apiName+"/secrets/"+secretName+"/regenerate",
				nil, "")
		})

	// Uses the secret name captured from the most recent create call.
	ctx.Step(`^I delete the saved webhook secret from WebSub API "([^"]*)"$`,
		func(apiName string) error {
			if ws.lastSecretName == "" {
				return fmt.Errorf("no saved webhook secret name; call 'create a webhook secret' first")
			}
			return doRequest(state, http.MethodDelete,
				state.Config.GatewayControllerURL+"/websub-apis/"+apiName+"/secrets/"+ws.lastSecretName,
				nil, "")
		})

	// Uses an explicitly named secret (for error-case tests).
	ctx.Step(`^I delete webhook secret "([^"]*)" from WebSub API "([^"]*)"$`,
		func(secretName, apiName string) error {
			return doRequest(state, http.MethodDelete,
				state.Config.GatewayControllerURL+"/websub-apis/"+apiName+"/secrets/"+secretName,
				nil, "")
		})

	ctx.Step(`^the webhook secret value should start with "([^"]*)"$`,
		func(prefix string) error {
			if ws.lastSecretValue == "" {
				return fmt.Errorf("no webhook secret value captured from response")
			}
			if !strings.HasPrefix(ws.lastSecretValue, prefix) {
				return fmt.Errorf("expected secret value to start with %q, got %q", prefix, ws.lastSecretValue)
			}
			return nil
		})

	// Publishes a webhook event signed with the HMAC-SHA256 of the captured secret value.
	ctx.Step(`^I publish event "([^"]*)" to topic "([^"]*)" on API "([^"]*)" version "([^"]*)" with the saved HMAC signature$`,
		func(payload, topic, apiCtx, version string) error {
			if ws.lastSecretValue == "" {
				return fmt.Errorf("no saved webhook secret value; call 'create a webhook secret' first")
			}
			return publishWithHMAC(state, ws.lastSecretValue, payload, topic, apiCtx, version)
		})
}

// parseWebhookSecretResponse extracts (name, plaintext) from a WebhookSecretCreationResponse body.
// Returns empty strings on parse failure.
func parseWebhookSecretResponse(body []byte) (name, value string) {
	var resp struct {
		Secret        string `json:"secret"`
		WebhookSecret *struct {
			Name *string `json:"name"`
		} `json:"webhookSecret"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", ""
	}
	if resp.WebhookSecret != nil && resp.WebhookSecret.Name != nil {
		name = *resp.WebhookSecret.Name
	}
	return name, resp.Secret
}

// publishWithHMAC sends a webhook event to the event gateway's webhook-receiver endpoint,
// adding an x-hub-signature header containing an HMAC-SHA256 of the payload body.
func publishWithHMAC(state *TestState, secret, payload, topic, apiCtx, version string) error {
	publishURL := fmt.Sprintf("%s/%s/%s/webhook-receiver?topic=%s",
		state.Config.WebSubURL, apiCtx, version, topic)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequest(http.MethodPost, publishURL, bytes.NewBufferString(payload))
	if err != nil {
		return fmt.Errorf("failed to build publish request: %w", err)
	}
	for k, v := range state.headers {
		if strings.EqualFold(k, "Content-Type") {
			continue
		}
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("X-Hub-Signature-256", sig)

	resp, err := state.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("publish request failed: %w", err)
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	state.lastResponse = resp
	state.lastBody = bodyBytes
	return nil
}
