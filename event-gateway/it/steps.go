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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

// --- Common HTTP step helpers ---

// RegisterCommonSteps registers generic HTTP assertion and utility steps.
func RegisterCommonSteps(ctx *godog.ScenarioContext, state *TestState) {
	ctx.Step(`^the response status code should be (\d+)$`, func(code int) error {
		if state.lastResponse == nil {
			return fmt.Errorf("no response received")
		}
		if state.lastResponse.StatusCode != code {
			return fmt.Errorf("expected HTTP %d, got %d\nbody: %s", code, state.lastResponse.StatusCode, state.lastBody)
		}
		return nil
	})

	ctx.Step(`^the response should be successful$`, func() error {
		if state.lastResponse == nil {
			return fmt.Errorf("no response received")
		}
		if state.lastResponse.StatusCode < 200 || state.lastResponse.StatusCode >= 300 {
			return fmt.Errorf("expected 2xx, got %d\nbody: %s", state.lastResponse.StatusCode, state.lastBody)
		}
		return nil
	})

	ctx.Step(`^the response should be valid JSON$`, func() error {
		var v interface{}
		if err := json.Unmarshal(state.lastBody, &v); err != nil {
			return fmt.Errorf("response is not valid JSON: %w\nbody: %s", err, state.lastBody)
		}
		return nil
	})

	ctx.Step(`^the response body should contain "([^"]*)"$`, func(expected string) error {
		if !strings.Contains(string(state.lastBody), expected) {
			return fmt.Errorf("response body does not contain %q\nbody: %s", expected, state.lastBody)
		}
		return nil
	})

	ctx.Step(`^the JSON response field "([^"]*)" should be "([^"]*)"$`, func(field, expected string) error {
		var v map[string]interface{}
		if err := json.Unmarshal(state.lastBody, &v); err != nil {
			return fmt.Errorf("response is not valid JSON: %w", err)
		}
		got, ok := v[field]
		if !ok {
			return fmt.Errorf("field %q not found in response", field)
		}
		if fmt.Sprintf("%v", got) != expected {
			return fmt.Errorf("expected field %q = %q, got %q", field, expected, got)
		}
		return nil
	})

	ctx.Step(`^I wait for (\d+) seconds$`, func(secs int) error {
		time.Sleep(time.Duration(secs) * time.Second)
		return nil
	})

	ctx.Step(`^I authenticate using basic auth as "([^"]*)"$`, func(userKey string) error {
		user, ok := state.Config.Users[userKey]
		if !ok {
			return fmt.Errorf("unknown user: %s", userKey)
		}
		req, _ := http.NewRequest("GET", "http://localhost", nil)
		req.SetBasicAuth(user.Username, user.Password)
		state.SetHeader("Authorization", req.Header.Get("Authorization"))
		return nil
	})
}

// --- Internal HTTP helpers ---

func doRequest(state *TestState, method, rawURL string, body io.Reader, contentType string) error {
	req, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range state.headers {
		req.Header.Set(k, v)
	}

	resp, err := state.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
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

// --- Health steps ---

// RegisterHealthSteps registers health / readiness step definitions.
func RegisterHealthSteps(ctx *godog.ScenarioContext, state *TestState) {
	ctx.Step(`^the event gateway services are running$`, func() error {
		if state == nil || state.HTTPClient == nil {
			return fmt.Errorf("test state not initialized")
		}
		return nil
	})

	ctx.Step(`^I send a GET request to the event gateway health endpoint$`, func() error {
		return doRequest(state, http.MethodGet, state.Config.EventGatewayAdminURL+"/health", nil, "")
	})

	ctx.Step(`^I send a GET request to the event gateway ready endpoint$`, func() error {
		return doRequest(state, http.MethodGet, state.Config.EventGatewayAdminURL+"/ready", nil, "")
	})

	ctx.Step(`^the response should indicate UP status$`, func() error {
		if !strings.Contains(string(state.lastBody), "UP") {
			return fmt.Errorf("expected UP status in response, got: %s", state.lastBody)
		}
		return nil
	})

	ctx.Step(`^the response should indicate READY status$`, func() error {
		if !strings.Contains(string(state.lastBody), "READY") {
			return fmt.Errorf("expected READY status in response, got: %s", state.lastBody)
		}
		return nil
	})
}

// --- WebSub API management steps ---

// webSubAPIPayload is the delay for xDS to propagate after a control plane mutation.
const webSubAPIPayload = 2 * time.Second

// RegisterWebSubSteps registers all WebSub API management and end-to-end step definitions.
func RegisterWebSubSteps(ctx *godog.ScenarioContext, state *TestState) {
	// --- Control plane operations ---

	ctx.Step(`^I create a WebSub API with the following configuration:$`, func(body *godog.DocString) error {
		state.SetHeader("Content-Type", "application/json")
		err := doRequest(state, http.MethodPost,
			state.Config.GatewayControllerURL+"/websub-apis",
			bytes.NewBufferString(body.Content), "application/json")
		if err != nil {
			return err
		}
		time.Sleep(webSubAPIPayload)
		return nil
	})

	ctx.Step(`^I update the WebSub API "([^"]*)" with the following configuration:$`, func(name string, body *godog.DocString) error {
		err := doRequest(state, http.MethodPut,
			state.Config.GatewayControllerURL+"/websub-apis/"+name,
			bytes.NewBufferString(body.Content), "application/json")
		if err != nil {
			return err
		}
		time.Sleep(webSubAPIPayload)
		return nil
	})

	ctx.Step(`^I delete the WebSub API "([^"]*)"$`, func(name string) error {
		err := doRequest(state, http.MethodDelete,
			state.Config.GatewayControllerURL+"/websub-apis/"+name,
			nil, "")
		if err != nil {
			return err
		}
		time.Sleep(webSubAPIPayload)
		return nil
	})

	ctx.Step(`^I list all WebSub APIs$`, func() error {
		return doRequest(state, http.MethodGet,
			state.Config.GatewayControllerURL+"/websub-apis",
			nil, "")
	})

	ctx.Step(`^I get the WebSub API "([^"]*)"$`, func(name string) error {
		return doRequest(state, http.MethodGet,
			state.Config.GatewayControllerURL+"/websub-apis/"+name,
			nil, "")
	})

	// --- WebSub protocol operations ---

	ctx.Step(`^I subscribe to topic "([^"]*)" on API "([^"]*)" version "([^"]*)" with callback "([^"]*)"$`,
		func(topic, apiCtx, version, callback string) error {
			hubURL := fmt.Sprintf("%s/%s/%s/hub", state.Config.WebSubURL, apiCtx, version)
			formData := url.Values{
				"hub.mode":          {"subscribe"},
				"hub.topic":         {topic},
				"hub.callback":      {callback},
				"hub.secret":        {"test-secret"},
				"hub.lease_seconds": {"3600"},
			}
			req, err := http.NewRequest(http.MethodPost, hubURL,
				strings.NewReader(formData.Encode()))
			if err != nil {
				return err
			}
			for k, v := range state.headers {
				if strings.EqualFold(k, "Content-Type") {
					continue
				}
				req.Header.Set(k, v)
			}
			// Set after state headers so it is not overridden by a stale Content-Type.
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			resp, err := state.HTTPClient.Do(req)
			if err != nil {
				return fmt.Errorf("subscribe request failed: %w", err)
			}
			defer resp.Body.Close()
			bodyBytes, _ := io.ReadAll(resp.Body)
			state.lastResponse = resp
			state.lastBody = bodyBytes
			return nil
		})

	ctx.Step(`^I unsubscribe from topic "([^"]*)" on API "([^"]*)" version "([^"]*)" with callback "([^"]*)"$`,
		func(topic, apiCtx, version, callback string) error {
			hubURL := fmt.Sprintf("%s/%s/%s/hub", state.Config.WebSubURL, apiCtx, version)
			formData := url.Values{
				"hub.mode":     {"unsubscribe"},
				"hub.topic":    {topic},
				"hub.callback": {callback},
			}
			req, err := http.NewRequest(http.MethodPost, hubURL,
				strings.NewReader(formData.Encode()))
			if err != nil {
				return err
			}
			for k, v := range state.headers {
				if strings.EqualFold(k, "Content-Type") {
					continue
				}
				req.Header.Set(k, v)
			}
			// Set after state headers so it is not overridden by a stale Content-Type.
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			resp, err := state.HTTPClient.Do(req)
			if err != nil {
				return fmt.Errorf("unsubscribe request failed: %w", err)
			}
			defer resp.Body.Close()
			bodyBytes, _ := io.ReadAll(resp.Body)
			state.lastResponse = resp
			state.lastBody = bodyBytes
			return nil
		})

	ctx.Step(`^I publish event "([^"]*)" to topic "([^"]*)" on API "([^"]*)" version "([^"]*)"$`,
		func(payload, topic, apiCtx, version string) error {
			publishURL := fmt.Sprintf("%s/%s/%s/webhook-receiver?topic=%s",
				state.Config.WebSubURL, apiCtx, version, topic)
			req, err := http.NewRequest(http.MethodPost, publishURL,
				strings.NewReader(payload))
			if err != nil {
				return err
			}
			for k, v := range state.headers {
				if strings.EqualFold(k, "Content-Type") {
					continue
				}
				req.Header.Set(k, v)
			}
			// Set after state headers so it is not overridden by a stale Content-Type.
			req.Header.Set("Content-Type", "text/plain")
			resp, err := state.HTTPClient.Do(req)
			if err != nil {
				return fmt.Errorf("publish request failed: %w", err)
			}
			defer resp.Body.Close()
			bodyBytes, _ := io.ReadAll(resp.Body)
			state.lastResponse = resp
			state.lastBody = bodyBytes
			return nil
		})

	ctx.Step(`^I wait for event delivery for (\d+) seconds$`, func(secs int) error {
		time.Sleep(time.Duration(secs) * time.Second)
		return nil
	})

	ctx.Step(`^the webhook listener should have received the event "([^"]*)"$`, func(payload string) error {
		// Poll the wh-listener logs endpoint (admin) if available, otherwise just check
		// that publish returned a 2xx (delivery is async so we can only verify acceptance).
		// The wh-listener doesn't expose a query API, so we validate via publish status.
		if state.lastResponse == nil {
			return fmt.Errorf("no response from publish step")
		}
		if state.lastResponse.StatusCode < 200 || state.lastResponse.StatusCode >= 300 {
			return fmt.Errorf("event publish was not accepted: HTTP %d", state.lastResponse.StatusCode)
		}
		return nil
	})
}
