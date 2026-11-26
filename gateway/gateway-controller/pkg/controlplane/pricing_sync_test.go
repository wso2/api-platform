/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package controlplane

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The pricing catalog push: the price file, posted unchanged to the control plane after a
// connect, when cost tracking or budgets are on.

// A file as the real one looks: formatting, ordering and fields the control plane ignores
// are all part of what must arrive as it is.
const smallCatalog = `{
  "gpt-4o": {"input_cost_per_token": 2.5e-06, "output_cost_per_token": 1e-05, "max_input_tokens": 128000, "provider": "openai"},
  "azure/eu/gpt-4.1": {"input_cost_per_token": 2e-06, "output_cost_per_token": 8e-06, "provider": "azure", "extra": {"a": 1}},
  "dall-e-2": {"mode": "image"}
}
`

// pricingRequest is what a test server saw.
type pricingRequest struct {
	method, path, contentType, apiKey string
	body                              []byte
}

// pricingServer is a control plane that records the pricing posts and answers with the
// next of the given statuses (the last one repeats).
func pricingServer(t *testing.T, statuses ...int) (*Client, func() []pricingRequest) {
	t.Helper()
	var mu sync.Mutex
	var seen []pricingRequest
	client := newManifestTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, pricingRequest{r.Method, r.URL.Path, r.Header.Get("Content-Type"), r.Header.Get("api-key"), body})
		status := statuses[len(statuses)-1]
		if len(seen) <= len(statuses) {
			status = statuses[len(seen)-1]
		}
		w.WriteHeader(status)
	})
	// Fast retries: a few attempts a few milliseconds apart.
	client.pushMaxAttempts = 3
	client.pushRetryBaseWait = time.Millisecond
	client.pushRetryMaxWait = 2 * time.Millisecond
	return client, func() []pricingRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]pricingRequest(nil), seen...)
	}
}

// withPricing writes a catalog file, points the llm-cost policy's config at it and turns
// cost tracking on, as a gateway with prices configured.
func withPricing(t *testing.T, client *Client, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "model_prices.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	client.systemConfig.AICostManagement.Tracking.Enabled = true
	client.systemConfig.PolicyConfigurations = map[string]interface{}{
		"llm_cost_v1": map[string]interface{}{"pricing_file": path},
	}
	return path
}

func TestPricingPush_PostsTheFileAsItIs(t *testing.T) {
	client, seen := pricingServer(t, http.StatusNoContent)
	withPricing(t, client, smallCatalog)

	client.pushPricingOnConnect(testGatewayID)

	got := seen()
	require.Len(t, got, 1)
	assert.Equal(t, http.MethodPost, got[0].method)
	assert.Equal(t, "/api/internal/v1/gateways/gw-1/pricing", got[0].path)
	assert.Equal(t, "application/json", got[0].contentType)
	assert.Equal(t, "test-token", got[0].apiKey, "the gateway's token, as the manifest push sends it")
	assert.Equal(t, smallCatalog, string(got[0].body), "the file's bytes, unwrapped and unchanged")
}

func TestPricingPush_BudgetsAloneAlsoPushIt(t *testing.T) {
	client, seen := pricingServer(t, http.StatusNoContent)
	withPricing(t, client, smallCatalog)
	client.systemConfig.AICostManagement.Tracking.Enabled = false
	client.systemConfig.AICostManagement.Budgets.Enabled = true

	client.pushPricingOnConnect(testGatewayID)

	assert.Len(t, seen(), 1)
}

func TestPricingPush_NothingWhenTrackingAndBudgetsAreOff(t *testing.T) {
	client, seen := pricingServer(t, http.StatusNoContent)
	withPricing(t, client, smallCatalog)
	client.systemConfig.AICostManagement.Tracking.Enabled = false
	client.systemConfig.AICostManagement.Budgets.Enabled = false

	assert.False(t, client.pricingEnabled())
	client.pushPricingOnConnect(testGatewayID)

	assert.Empty(t, seen(), "both off: the gateway does nothing about prices")
}

func TestPricingPush_AMissingOrBrokenFileIsNotPushedAndWarnedOnce(t *testing.T) {
	for name, content := range map[string]*string{
		"missing":       nil,
		"not json":      ptr("model prices"),
		"not an object": ptr(`["gpt-4o"]`),
		"empty":         ptr(""),
	} {
		t.Run(name, func(t *testing.T) {
			client, seen := pricingServer(t, http.StatusNoContent)
			path := withPricing(t, client, "{}")
			if content == nil {
				require.NoError(t, os.Remove(path))
			} else {
				require.NoError(t, os.WriteFile(path, []byte(*content), 0o600))
			}

			client.pushPricingOnConnect(testGatewayID)
			assert.True(t, client.pricingWarned.Load(), "a warning was logged")
			client.pushPricingOnConnect(testGatewayID) // a reconnect: still no push, no second warning
			assert.Empty(t, seen())
		})
	}
}

func TestPricingPush_AFileOverTheLimitIsNotPushed(t *testing.T) {
	client, seen := pricingServer(t, http.StatusNoContent)
	withPricing(t, client, "{}")
	path := client.pricingFile()
	filler := make([]byte, maxPricingBytes+10)
	for i := range filler {
		filler[i] = 'a'
	}
	require.NoError(t, os.WriteFile(path, append(append([]byte(`{"m": "`), filler...), []byte(`"}`)...), 0o600))

	client.pushPricingOnConnect(testGatewayID)

	assert.Empty(t, seen())
	assert.True(t, client.pricingWarned.Load())
}

func TestPricingPush_ARefusalIsLoggedOnceAndNeverRetriedOrFatal(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusForbidden, http.StatusUnauthorized, http.StatusBadRequest, http.StatusRequestEntityTooLarge} {
		client, seen := pricingServer(t, status)
		withPricing(t, client, smallCatalog)

		// Returns (no exit of the process, no endless retry) after a single attempt.
		client.pushPricingOnConnect(testGatewayID)
		assert.Len(t, seen(), 1, "status %d is not retried", status)
		assert.True(t, client.pricingRejected.Load())

		client.pushPricingOnConnect(testGatewayID) // the next connect tries again, once
		assert.Len(t, seen(), 2)
	}
}

func TestPricingPush_AServerErrorIsRetriedLikeTheOtherPushes(t *testing.T) {
	client, seen := pricingServer(t, http.StatusServiceUnavailable, http.StatusBadGateway, http.StatusNoContent)
	withPricing(t, client, smallCatalog)

	client.pushPricingOnConnect(testGatewayID)

	got := seen()
	require.Len(t, got, 3, "two failures, then the third attempt (max attempts 3) succeeds")
	assert.Equal(t, smallCatalog, string(got[2].body))
}

func TestPricingPush_GivesUpAfterTheConfiguredAttemptsWithoutBlockingForever(t *testing.T) {
	client, seen := pricingServer(t, http.StatusServiceUnavailable)
	withPricing(t, client, smallCatalog)
	client.pushMaxAttempts = 2

	done := make(chan struct{})
	go func() { client.pushPricingOnConnect(testGatewayID); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the push did not give up")
	}
	assert.Len(t, seen(), 2)
	assert.False(t, client.pricingRejected.Load(), "a server error is not a refusal")
}

func TestPricingFile_ComesFromTheLlmCostConfigElseTheDefaultMount(t *testing.T) {
	client := createTestClient(t)
	assert.Equal(t, defaultPricingFile, client.pricingFile())

	client.systemConfig.PolicyConfigurations = map[string]interface{}{
		"llm_cost_v1": map[string]interface{}{"pricing_file": "/data/prices.json"},
	}
	assert.Equal(t, "/data/prices.json", client.pricingFile())

	client.systemConfig.PolicyConfigurations = map[string]interface{}{
		"llm_cost_v1": map[string]interface{}{"pricing_file": ""},
	}
	assert.Equal(t, defaultPricingFile, client.pricingFile(), "an empty value falls back to the mount")
}

func ptr(s string) *string { return &s }
