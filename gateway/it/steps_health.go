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
	"io"
	"net/http"
	"strings"

	"github.com/cucumber/godog"
)

// RegisterHealthSteps registers all health check step definitions
func RegisterHealthSteps(ctx *godog.ScenarioContext, state *TestState) {
	ctx.Step(`^the gateway services are running$`, state.theGatewayServicesAreRunning)
	ctx.Step(`^I send a GET request to the gateway controller health endpoint$`, state.iSendGETRequestToGatewayControllerHealth)
	ctx.Step(`^I send a GET request to the router ready endpoint$`, state.iSendGETRequestToRouterReady)
	ctx.Step(`^the response status code should be (\d+)$`, state.theResponseStatusCodeShouldBe)
	ctx.Step(`^the response should indicate healthy status$`, state.theResponseShouldIndicateHealthyStatus)
	ctx.Step(`^I check the health of all gateway services$`, state.iCheckHealthOfAllGatewayServices)
	ctx.Step(`^all services should report healthy status$`, state.allServicesShouldReportHealthyStatus)
}

// theGatewayServicesAreRunning verifies that gateway services are available
func (s *TestState) theGatewayServicesAreRunning() error {
	// This is verified during suite setup, so we just confirm the state is valid
	if s.Config == nil || s.HTTPClient == nil {
		return fmt.Errorf("test state not properly initialized")
	}
	return nil
}

// iSendGETRequestToGatewayControllerHealth sends a GET request to the gateway controller health endpoint
func (s *TestState) iSendGETRequestToGatewayControllerHealth() error {
	url := fmt.Sprintf("%s/health", s.Config.GatewayControllerURL)
	return s.sendGETRequest(url)
}

// iSendGETRequestToRouterReady sends a GET request to the router ready endpoint
func (s *TestState) iSendGETRequestToRouterReady() error {
	url := fmt.Sprintf("http://localhost:%s/ready", EnvoyAdminPort)
	return s.sendGETRequest(url)
}

// sendGETRequest is a helper to send GET requests
func (s *TestState) sendGETRequest(url string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		s.LastError = err
		return fmt.Errorf("failed to create request: %w", err)
	}

	s.LastRequest = req

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		s.LastError = err
		return fmt.Errorf("failed to send request: %w", err)
	}

	s.LastResponse = resp
	s.LastError = nil
	return nil
}

// theResponseStatusCodeShouldBe verifies the response status code
func (s *TestState) theResponseStatusCodeShouldBe(expectedCode int) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.LastResponse == nil {
		return fmt.Errorf("no response received")
	}

	if s.LastResponse.StatusCode != expectedCode {
		return fmt.Errorf("expected status code %d, got %d", expectedCode, s.LastResponse.StatusCode)
	}

	return nil
}

// theResponseShouldIndicateHealthyStatus verifies the response body indicates healthy
func (s *TestState) theResponseShouldIndicateHealthyStatus() error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.LastResponse == nil {
		return fmt.Errorf("no response received")
	}

	body, err := io.ReadAll(s.LastResponse.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	bodyStr := strings.ToLower(string(body))
	if !strings.Contains(bodyStr, "ok") && !strings.Contains(bodyStr, "healthy") {
		return fmt.Errorf("response does not indicate healthy status: %s", bodyStr)
	}

	return nil
}

// iCheckHealthOfAllGatewayServices checks all gateway service health endpoints
func (s *TestState) iCheckHealthOfAllGatewayServices() error {
	healthEndpoints := []struct {
		name string
		url  string
	}{
		{"gateway-controller", fmt.Sprintf("%s/health", s.Config.GatewayControllerURL)},
		{"router", fmt.Sprintf("http://localhost:%s/ready", EnvoyAdminPort)},
	}

	results := make(map[string]bool)
	for _, endpoint := range healthEndpoints {
		resp, err := s.HTTPClient.Get(endpoint.url)
		if err != nil {
			results[endpoint.name] = false
			continue
		}
		defer resp.Body.Close()
		results[endpoint.name] = resp.StatusCode == http.StatusOK
	}

	s.SetContextValue("health_results", results)
	return nil
}

// allServicesShouldReportHealthyStatus verifies all services are healthy
func (s *TestState) allServicesShouldReportHealthyStatus() error {
	val, ok := s.GetContextValue("health_results")
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
