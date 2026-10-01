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
	"strings"

	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/gateway/it/steps"
)

const (
	// GatewayControllerMetricsPort is the port for gateway-controller metrics
	GatewayControllerMetricsPort = "9091"

	// PolicyEngineMetricsPort is the port for policy-engine metrics
	PolicyEngineMetricsPort = "9003"
)

// MetricsSteps wraps TestState and HTTPSteps for metrics step definitions
type MetricsSteps struct {
	state     *TestState
	httpSteps *steps.HTTPSteps
}

// RegisterMetricsSteps registers all metrics step definitions
func RegisterMetricsSteps(ctx *godog.ScenarioContext, state *TestState, httpSteps *steps.HTTPSteps) {
	m := &MetricsSteps{state: state, httpSteps: httpSteps}
	ctx.Step(`^I send a GET request to the gateway controller metrics endpoint$`, m.iSendGETRequestToGatewayControllerMetrics)
	ctx.Step(`^I send a GET request to the policy engine metrics endpoint$`, m.iSendGETRequestToPolicyEngineMetrics)
	ctx.Step(`^the response should contain Prometheus metrics$`, m.theResponseShouldContainPrometheusMetrics)
	ctx.Step(`^the response should contain metric "([^"]*)"$`, m.theResponseShouldContainMetric)
}

// iSendGETRequestToGatewayControllerMetrics sends a GET request to the gateway controller metrics endpoint
func (m *MetricsSteps) iSendGETRequestToGatewayControllerMetrics() error {
	url := fmt.Sprintf("http://localhost:%s/metrics", GatewayControllerMetricsPort)
	return m.httpSteps.SendGETRequest(url)
}

// iSendGETRequestToPolicyEngineMetrics sends a GET request to the policy engine metrics endpoint
func (m *MetricsSteps) iSendGETRequestToPolicyEngineMetrics() error {
	url := fmt.Sprintf("http://localhost:%s/metrics", PolicyEngineMetricsPort)
	return m.httpSteps.SendGETRequest(url)
}

// theResponseShouldContainPrometheusMetrics verifies the response contains valid Prometheus metrics
func (m *MetricsSteps) theResponseShouldContainPrometheusMetrics() error {
	resp := m.httpSteps.LastResponse()
	if resp == nil {
		return fmt.Errorf("no response received")
	}

	body := m.httpSteps.LastBody()
	bodyStr := string(body)

	// Check for Prometheus metric format indicators
	// Valid metrics should have lines starting with # (comments) or metric names
	if !strings.Contains(bodyStr, "# HELP") && !strings.Contains(bodyStr, "# TYPE") {
		return fmt.Errorf("response does not contain Prometheus metric format headers")
	}

	// Ensure there's actual metric data (not just comments)
	lines := strings.Split(bodyStr, "\n")
	hasMetricData := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// If we find a non-empty, non-comment line, it's metric data
		hasMetricData = true
		break
	}

	if !hasMetricData {
		return fmt.Errorf("response contains Prometheus headers but no actual metric data")
	}

	return nil
}

// theResponseShouldContainMetric verifies the response contains a specific metric
func (m *MetricsSteps) theResponseShouldContainMetric(metricName string) error {
	body := m.httpSteps.LastBody()
	bodyStr := string(body)

	if !strings.Contains(bodyStr, metricName) {
		return fmt.Errorf("response does not contain metric '%s'", metricName)
	}

	return nil
}
