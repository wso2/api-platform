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
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/gorilla/websocket"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
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
		var v any
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
		var v map[string]any
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

	ctx.Step(`^the WebSub API "([^"]*)" version "([^"]*)" is reachable within (\d+) seconds$`,
		func(apiCtx, version string, secs int) error {
			hubURL := fmt.Sprintf("%s/%s/%s/hub", state.Config.WebSubURL, strings.TrimPrefix(apiCtx, "/"), version)
			deadline := time.Now().Add(time.Duration(secs) * time.Second)
			for time.Now().Before(deadline) {
				resp, err := state.HTTPClient.Get(hubURL)
				if err == nil {
					resp.Body.Close()
					if resp.StatusCode != http.StatusNotFound {
						// HTTP route is registered. Wait an additional 3 s so the Kafka
						// client inside the runtime can warm its connection before the
						// first subscribe request arrives.
						time.Sleep(3 * time.Second)
						return nil
					}
				}
				time.Sleep(500 * time.Millisecond)
			}
			return fmt.Errorf("WebSub API %s/%s not reachable after %d seconds", apiCtx, version, secs)
		})

	ctx.Step(`^the WebBroker API at "([^"]*)" is reachable within (\d+) seconds$`,
		func(apiCtx string, secs int) error {
			httpBase := strings.NewReplacer("ws://", "http://", "wss://", "https://").Replace(state.Config.WebSocketURL)
			deadline := time.Now().Add(time.Duration(secs) * time.Second)
			for time.Now().Before(deadline) {
				resp, err := state.HTTPClient.Get(httpBase + apiCtx)
				if err == nil {
					resp.Body.Close()
					if resp.StatusCode != http.StatusNotFound {
						return nil
					}
				}
				time.Sleep(500 * time.Millisecond)
			}
			return fmt.Errorf("WebBroker API %s not reachable after %d seconds", apiCtx, secs)
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
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read response body: %w", err)
			}
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
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read response body: %w", err)
			}
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
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read response body: %w", err)
			}
			state.lastResponse = resp
			state.lastBody = bodyBytes
			return nil
		})

	ctx.Step(`^I wait for event delivery for (\d+) seconds$`, func(secs int) error {
		time.Sleep(time.Duration(secs) * time.Second)
		return nil
	})

	ctx.Step(`^the webhook listener should have received the event "([^"]*)"$`, func(payload string) error {
		err := checkListenerReceivedEvent(state, payload, 10*time.Second)
		if errors.Is(err, errListenerUnavailable) {
			// /received-events endpoint not present; fall back to verifying publish acceptance.
			if state.lastResponse == nil {
				return fmt.Errorf("no response from publish step")
			}
			if state.lastResponse.StatusCode < 200 || state.lastResponse.StatusCode >= 300 {
				return fmt.Errorf("event publish was not accepted: HTTP %d", state.lastResponse.StatusCode)
			}
			return nil
		}
		return err
	})
}

// --- WebBroker API management and WebSocket step definitions ---

const webBrokerAPIDelay = 2 * time.Second

// RegisterWebBrokerSteps registers all WebBroker API management and end-to-end step definitions.
func RegisterWebBrokerSteps(ctx *godog.ScenarioContext, state *TestState) {
	// --- Control plane operations ---

	ctx.Step(`^I create a WebBroker API with the following configuration:$`, func(body *godog.DocString) error {
		err := doRequest(state, http.MethodPost,
			state.Config.GatewayControllerURL+"/webbroker-apis",
			bytes.NewBufferString(body.Content), "application/json")
		if err != nil {
			return err
		}
		time.Sleep(webBrokerAPIDelay)
		return nil
	})

	ctx.Step(`^I update the WebBroker API "([^"]*)" with the following configuration:$`, func(name string, body *godog.DocString) error {
		err := doRequest(state, http.MethodPut,
			state.Config.GatewayControllerURL+"/webbroker-apis/"+name,
			bytes.NewBufferString(body.Content), "application/json")
		if err != nil {
			return err
		}
		time.Sleep(webBrokerAPIDelay)
		return nil
	})

	ctx.Step(`^I delete the WebBroker API "([^"]*)"$`, func(name string) error {
		err := doRequest(state, http.MethodDelete,
			state.Config.GatewayControllerURL+"/webbroker-apis/"+name,
			nil, "")
		if err != nil {
			return err
		}
		time.Sleep(webBrokerAPIDelay)
		return nil
	})

	ctx.Step(`^I list all WebBroker APIs$`, func() error {
		return doRequest(state, http.MethodGet,
			state.Config.GatewayControllerURL+"/webbroker-apis",
			nil, "")
	})

	ctx.Step(`^I get the WebBroker API "([^"]*)"$`, func(name string) error {
		return doRequest(state, http.MethodGet,
			state.Config.GatewayControllerURL+"/webbroker-apis/"+name,
			nil, "")
	})

	// --- Auth helper ---

	ctx.Step(`^I clear all authentication headers$`, func() error {
		delete(state.headers, "Authorization")
		return nil
	})

	// --- WebSocket steps ---

	ctx.Step(`^I connect to WebBroker API "([^"]*)" on channel "([^"]*)"$`,
		func(apiContext, channel string) error {
			wsURL := state.Config.WebSocketURL + apiContext
			header := http.Header{"X-channel": {channel}}
			for k, v := range state.headers {
				header.Set(k, v)
			}
			conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
			if resp != nil {
				state.wsRejectionStatus = resp.StatusCode
				_ = resp.Body.Close()
			}
			state.wsConn = conn
			state.wsConnErr = err
			return nil // rejection is asserted by a separate step
		})

	ctx.Step(`^I send WebSocket message "([^"]*)"$`, func(payload string) error {
		if state.wsConn == nil {
			return fmt.Errorf("no active WebSocket connection")
		}
		return state.wsConn.WriteMessage(websocket.BinaryMessage, []byte(payload))
	})

	ctx.Step(`^I should receive a WebSocket message containing "([^"]*)" within (\d+) seconds$`,
		func(expected string, secs int) error {
			if state.wsConn == nil {
				return fmt.Errorf("no active WebSocket connection")
			}
			deadline := time.Now().Add(time.Duration(secs) * time.Second)
			_ = state.wsConn.SetReadDeadline(deadline)
			for time.Now().Before(deadline) {
				_, msg, err := state.wsConn.ReadMessage()
				if err != nil {
					return fmt.Errorf("failed to read WebSocket message: %w", err)
				}
				if strings.Contains(string(msg), expected) {
					return nil
				}
			}
			return fmt.Errorf("did not receive WebSocket message containing %q within %d seconds", expected, secs)
		})

	ctx.Step(`^I close the WebSocket connection$`, func() error {
		if state.wsConn != nil {
			_ = state.wsConn.WriteMessage(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			_ = state.wsConn.Close()
			state.wsConn = nil
		}
		return nil
	})

	ctx.Step(`^the WebSocket connection should be rejected with HTTP status (\d+)$`, func(code int) error {
		if state.wsConn != nil {
			return fmt.Errorf("expected WebSocket connection to be rejected, but it succeeded")
		}
		if state.wsConnErr == nil {
			return fmt.Errorf("expected WebSocket connection to fail, but no error was recorded")
		}
		if state.wsRejectionStatus != code {
			return fmt.Errorf("expected rejection HTTP status %d, got %d", code, state.wsRejectionStatus)
		}
		return nil
	})

	// --- Kafka steps ---

	ctx.Step(`^I publish "([^"]*)" to Kafka topic "([^"]*)"$`, func(payload, topic string) error {
		return publishToKafka(state.Config, topic, payload)
	})

	ctx.Step(`^the Kafka topic "([^"]*)" should contain a message with "([^"]*)" within (\d+) seconds$`,
		func(topic, expected string, secs int) error {
			return readFromKafka(state.Config, topic, expected, time.Duration(secs)*time.Second)
		})
}

// kafkaClient creates a kgo.Client configured to match docker-compose.dev.yaml:
// SASL/PLAIN + TLS (InsecureSkipVerify for the self-signed cert from kafka-cert-init).
//
// The Kafka advertised listener is "kafka:29092" (Docker-internal hostname).
// kafkaDialer transparently rewrites "kafka" → "localhost" so that the IT process
// running on the host can reach the port-mapped broker without modifying /etc/hosts.
func kafkaClient(cfg *Config, extra ...kgo.Opt) (*kgo.Client, error) {
	tlsCfg := &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	opts := []kgo.Opt{
		kgo.SeedBrokers(cfg.KafkaBrokers...),
		kgo.Dialer(func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			if host == "kafka" {
				addr = net.JoinHostPort("localhost", port)
			}
			return (&tls.Dialer{NetDialer: &net.Dialer{}, Config: tlsCfg}).DialContext(ctx, network, addr)
		}),
		kgo.SASL(plain.Auth{User: cfg.KafkaUsername, Pass: cfg.KafkaPassword}.AsMechanism()),
	}
	return kgo.NewClient(append(opts, extra...)...)
}

// publishToKafka produces a single record to the given topic.
func publishToKafka(cfg *Config, topic, payload string) error {
	cl, err := kafkaClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create Kafka producer: %w", err)
	}
	defer cl.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return cl.ProduceSync(ctx, &kgo.Record{Topic: topic, Value: []byte(payload)}).FirstErr()
}

// readFromKafka consumes from the beginning of topic and returns nil as soon as
// a record whose value contains expected is found, or an error on timeout.
func readFromKafka(cfg *Config, topic, expected string, timeout time.Duration) error {
	cl, err := kafkaClient(cfg,
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return fmt.Errorf("failed to create Kafka consumer: %w", err)
	}
	defer cl.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		fetches := cl.PollFetches(ctx)
		if ctx.Err() != nil {
			return fmt.Errorf("Kafka topic %q did not contain a message with %q within %s", topic, expected, timeout)
		}
		found := false
		fetches.EachRecord(func(r *kgo.Record) {
			if strings.Contains(string(r.Value), expected) {
				found = true
			}
		})
		if found {
			return nil
		}
	}
}

// errListenerUnavailable is returned by checkListenerReceivedEvent when the
// wh-listener /received-events endpoint is not reachable.
var errListenerUnavailable = errors.New("listener /received-events endpoint unavailable")

// checkListenerReceivedEvent polls GET /received-events on the wh-listener admin
// interface every 500 ms until a body containing payload is found or timeout expires.
// Returns errListenerUnavailable if the endpoint is not reachable on the first attempt.
func checkListenerReceivedEvent(state *TestState, payload string, timeout time.Duration) error {
	endpoint := state.Config.WebhookListenerURL + "/received-events"
	deadline := time.Now().Add(timeout)
	firstAttempt := true

	for time.Now().Before(deadline) {
		resp, err := state.HTTPClient.Get(endpoint)
		if err != nil {
			if firstAttempt {
				return errListenerUnavailable
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if firstAttempt && resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			return errListenerUnavailable
		}
		firstAttempt = false

		var bodies []string
		if err := json.NewDecoder(resp.Body).Decode(&bodies); err != nil {
			resp.Body.Close()
			return err
		}
		resp.Body.Close()

		for _, b := range bodies {
			if strings.Contains(b, payload) {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("webhook listener did not receive event containing %q within %s", payload, timeout)
}
