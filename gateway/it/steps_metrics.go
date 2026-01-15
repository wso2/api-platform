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
	"regexp"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/gateway/it/steps"
)

var (
	// apiCountMetricRegex is the regex pattern for parsing API count metrics
	apiCountMetricRegex = regexp.MustCompile(`gateway_controller_apis_total\{[^}]*\}\s+(\d+)`)
)

const (
	// testAPIDefinition is a simple API definition used for testing metrics
	testAPIDefinition = `
name: test-metrics-api
version: v1
basePath: /test-metrics
backend:
  url: http://it-sample-backend:9080
endpoints:
  - path: /test
    method: GET
`
)

// RegisterMetricsSteps registers all metrics step definitions
func RegisterMetricsSteps(ctx *godog.ScenarioContext, state *TestState, httpSteps *steps.HTTPSteps) {
	ctx.Step(`^I send a GET request to the gateway controller metrics endpoint$`, func() error {
		return state.iSendGETRequestToGatewayControllerMetrics()
	})

	ctx.Step(`^I send a GET request to the policy engine metrics endpoint$`, func() error {
		return state.iSendGETRequestToPolicyEngineMetrics()
	})

	ctx.Step(`^the response should contain prometheus metrics$`, state.theResponseShouldContainPrometheusMetrics)

	ctx.Step(`^the metrics should contain "([^"]*)"$`, state.theMetricsShouldContain)

	ctx.Step(`^I extract the current API count from metrics$`, state.iExtractCurrentAPICountFromMetrics)

	ctx.Step(`^I create a new API via the gateway controller$`, func() error {
		return state.iCreateTestAPIViaGatewayController(httpSteps)
	})

	ctx.Step(`^the API count metric should have increased$`, state.theAPICountMetricShouldHaveIncreased)
}

// iSendGETRequestToGatewayControllerMetrics sends a GET request to gateway controller metrics endpoint
func (s *TestState) iSendGETRequestToGatewayControllerMetrics() error {
	url := "http://localhost:9091/metrics"
	return s.sendGETRequest(url)
}

// iSendGETRequestToPolicyEngineMetrics sends a GET request to policy engine metrics endpoint
func (s *TestState) iSendGETRequestToPolicyEngineMetrics() error {
	url := "http://localhost:9003/metrics"
	return s.sendGETRequest(url)
}

// theResponseShouldContainPrometheusMetrics verifies the response contains prometheus format metrics
func (s *TestState) theResponseShouldContainPrometheusMetrics() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	bodyStr, err := s.getResponseBody()
	if err != nil {
		return err
	}

	// Check for prometheus format indicators
	// Prometheus metrics typically start with # HELP or # TYPE comments
	if !strings.Contains(bodyStr, "# HELP") && !strings.Contains(bodyStr, "# TYPE") {
		return fmt.Errorf("response does not appear to be in prometheus format")
	}

	return nil
}

// getResponseBody is a helper to read and cache the response body
// NOTE: This method assumes the caller already holds s.mutex lock
func (s *TestState) getResponseBody() (string, error) {
	// Check if body is already cached
	bodyStr, ok := s.Context["last_response_body"].(string)
	if ok {
		return bodyStr, nil
	}

	// Read body if not already read
	if s.LastResponse == nil {
		return "", fmt.Errorf("no response received")
	}

	if s.LastResponse.Body != nil {
		body, err := io.ReadAll(s.LastResponse.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read response body: %w", err)
		}
		s.LastResponse.Body.Close()
		bodyStr = string(body)
		s.Context["last_response_body"] = bodyStr
		return bodyStr, nil
	}

	return "", fmt.Errorf("response body not available")
}

// parseAPICountFromMetrics parses the API count from prometheus metrics
func parseAPICountFromMetrics(metricsBody string) int {
	count := 0
	matches := apiCountMetricRegex.FindAllStringSubmatch(metricsBody, -1)
	for _, match := range matches {
		if len(match) > 1 {
			val, err := strconv.Atoi(match[1])
			if err != nil {
				// Log parsing error but continue processing other matches
				fmt.Printf("Warning: failed to parse metric value '%s': %v\n", match[1], err)
				continue
			}
			count += val
		}
	}
	return count
}

// theMetricsShouldContain verifies the metrics contain a specific metric name
func (s *TestState) theMetricsShouldContain(metricName string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	bodyStr, err := s.getResponseBody()
	if err != nil {
		return err
	}

	if !strings.Contains(bodyStr, metricName) {
		return fmt.Errorf("metrics do not contain '%s'", metricName)
	}

	return nil
}

// iExtractCurrentAPICountFromMetrics extracts the API count metric value
func (s *TestState) iExtractCurrentAPICountFromMetrics() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	bodyStr, err := s.getResponseBody()
	if err != nil {
		return err
	}

	// Parse the API count from metrics
	count := parseAPICountFromMetrics(bodyStr)
	s.SetContextValue("initial_api_count", count)
	return nil
}

// iCreateTestAPIViaGatewayController creates a test API
func (s *TestState) iCreateTestAPIViaGatewayController(httpSteps *steps.HTTPSteps) error {
	body := &godog.DocString{Content: testAPIDefinition}
	httpSteps.SetHeader("Content-Type", "application/yaml")
	if err := httpSteps.SendPOSTToService("gateway-controller", "/apis", body); err != nil {
		return err
	}

	// Clear the cached response body since we'll be reading metrics again
	s.mutex.Lock()
	delete(s.Context, "last_response_body")
	s.mutex.Unlock()

	return nil
}

// theAPICountMetricShouldHaveIncreased verifies the API count increased
func (s *TestState) theAPICountMetricShouldHaveIncreased() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	initialCount, ok := s.GetContextInt("initial_api_count")
	if !ok {
		return fmt.Errorf("initial API count not found in context")
	}

	bodyStr, err := s.getResponseBody()
	if err != nil {
		return err
	}

	// Parse the current API count from metrics
	currentCount := parseAPICountFromMetrics(bodyStr)

	if currentCount <= initialCount {
		return fmt.Errorf("API count did not increase: initial=%d, current=%d", initialCount, currentCount)
	}

	return nil
}
