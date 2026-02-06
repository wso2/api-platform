/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package it

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/gateway/it/steps"
)

// HealthSteps wraps TestState and HTTPSteps for health check step definitions
type HealthSteps struct {
	state     *TestState
	httpSteps *steps.HTTPSteps
}

// RegisterHealthSteps registers all health check step definitions
func RegisterHealthSteps(ctx *godog.ScenarioContext, state *TestState, httpSteps *steps.HTTPSteps) {
	h := &HealthSteps{state: state, httpSteps: httpSteps}
	ctx.Step(`^the gateway services are running$`, h.theGatewayServicesAreRunning)
	ctx.Step(`^I send a GET request to the gateway controller health endpoint$`, h.iSendGETRequestToGatewayControllerHealth)
	ctx.Step(`^I send a GET request to the router ready endpoint$`, h.iSendGETRequestToRouterReady)
	// Note: "the response status code should be X" is registered in AssertSteps which uses HTTPSteps response
	ctx.Step(`^the response should indicate healthy status$`, h.theResponseShouldIndicateHealthyStatus)
	ctx.Step(`^I check the health of all gateway services$`, h.iCheckHealthOfAllGatewayServices)
	ctx.Step(`^all services should report healthy status$`, h.allServicesShouldReportHealthyStatus)
	ctx.Step(`^I wait for the endpoint "([^"]*)" to be ready$`, h.iWaitForEndpointToBeReady)
	ctx.Step(`^I wait for the endpoint "([^"]*)" to be ready with method "([^"]*)" and body '([^']*)'$`, h.iWaitForEndpointToBeReadyWithMethodAndBody)
}

// theGatewayServicesAreRunning verifies that gateway services are available
func (h *HealthSteps) theGatewayServicesAreRunning() error {
	// This is verified during suite setup, so we just confirm the state is valid
	if h.state.Config == nil || h.state.HTTPClient == nil {
		return fmt.Errorf("test state not properly initialized")
	}
	return nil
}

// iSendGETRequestToGatewayControllerHealth sends a GET request to the gateway controller health endpoint
func (h *HealthSteps) iSendGETRequestToGatewayControllerHealth() error {
	url := fmt.Sprintf("%s/health", h.state.Config.GatewayControllerURL)
	return h.httpSteps.SendGETRequest(url)
}

// iSendGETRequestToRouterReady sends a GET request to the router ready endpoint
func (h *HealthSteps) iSendGETRequestToRouterReady() error {
	url := fmt.Sprintf("http://localhost:%s/ready", EnvoyAdminPort)
	return h.httpSteps.SendGETRequest(url)
}

// theResponseShouldIndicateHealthyStatus verifies the response body indicates healthy
func (h *HealthSteps) theResponseShouldIndicateHealthyStatus() error {
	resp := h.httpSteps.LastResponse()
	if resp == nil {
		return fmt.Errorf("no response received")
	}

	body := h.httpSteps.LastBody()
	bodyStr := strings.ToLower(string(body))
	if !strings.Contains(bodyStr, "ok") && !strings.Contains(bodyStr, "healthy") {
		return fmt.Errorf("response does not indicate healthy status: %s", bodyStr)
	}

	return nil
}

// iCheckHealthOfAllGatewayServices checks all gateway service health endpoints
func (h *HealthSteps) iCheckHealthOfAllGatewayServices() error {
	healthEndpoints := []struct {
		name string
		url  string
	}{
		{"gateway-controller", fmt.Sprintf("%s/health", h.state.Config.GatewayControllerURL)},
		{"router", fmt.Sprintf("http://localhost:%s/ready", EnvoyAdminPort)},
	}

	results := make(map[string]bool)
	for _, endpoint := range healthEndpoints {
		resp, err := h.state.HTTPClient.Get(endpoint.url)
		if err != nil {
			results[endpoint.name] = false
			continue
		}
		resp.Body.Close()
		results[endpoint.name] = resp.StatusCode == http.StatusOK
	}

	h.state.SetContextValue("health_results", results)
	return nil
}

// allServicesShouldReportHealthyStatus verifies all services are healthy
func (h *HealthSteps) allServicesShouldReportHealthyStatus() error {
	val, ok := h.state.GetContextValue("health_results")
	if !ok {
		return fmt.Errorf("health check results not found")
	}

	results, ok := val.(map[string]bool)
	if !ok {
		return fmt.Errorf("invalid health results format")
	}

	var unhealthy []string
	for name, healthy := range results {
		if !healthy {
			unhealthy = append(unhealthy, name)
		}
	}

	if len(unhealthy) > 0 {
		return fmt.Errorf("unhealthy services: %v", unhealthy)
	}

	return nil
}

// iWaitForEndpointToBeReady polls an endpoint until it returns 200 or times out
// Optimized: 50 attempts × 300ms = 15s max (reduced from 10 × 2s = 20s)
func (h *HealthSteps) iWaitForEndpointToBeReady(url string) error {
	maxAttempts := 50
	attemptInterval := 300 * time.Millisecond

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		resp, err := h.state.HTTPClient.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}

		if attempt < maxAttempts {
			time.Sleep(attemptInterval)
		}
	}

	return fmt.Errorf("endpoint %s did not become ready after %d attempts", url, maxAttempts)
}

// iWaitForEndpointToBeReadyWithMethodAndBody polls an endpoint with specified method and body until it returns 200 or times out
// This is useful for testing POST endpoints that require a request body
func (h *HealthSteps) iWaitForEndpointToBeReadyWithMethodAndBody(url, method, body string) error {
	maxAttempts := 50
	attemptInterval := 300 * time.Millisecond

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequest(method, url, strings.NewReader(body))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := h.state.HTTPClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}

		if attempt < maxAttempts {
			time.Sleep(attemptInterval)
		}
	}

	return fmt.Errorf("endpoint %s did not become ready with %s method after %d attempts", url, method, maxAttempts)
}
