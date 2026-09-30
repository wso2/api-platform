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
	"regexp"
	"strconv"
	"strings"
	"time"

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
	// (.*) because a metric argument can carry an escaped quoted label value
	// (e.g. `cert_name=\"obs-expiring\"`), which [^"]* would stop at.
	ctx.Step(`^the response should contain metric "(.*)"$`, m.theResponseShouldContainMetric)
	ctx.Step(`^the response should not contain metric "(.*)"$`, m.theResponseShouldNotContainMetric)
	ctx.Step(`^I note the policy engine counter "([^"]*)" for series labelled "(.*)"$`, m.iNotePolicyEngineCounter)
	ctx.Step(`^the policy engine counter "([^"]*)" for series labelled "(.*)" should have grown by at least (\d+) within (\d+) seconds$`, m.policyEngineCounterShouldHaveGrown)
}

// notedCounterContextKeyPrefix prefixes the context key under which a noted
// counter total is kept, per counter name and label selector.
const notedCounterContextKeyPrefix = "notedCounter:"

// metricLabelPattern matches one name="value" pair of a Prometheus label set
// or of a step's label selector.
var metricLabelPattern = regexp.MustCompile(`(\w+)="((?:[^"\\]|\\.)*)"`)

// iNotePolicyEngineCounter records the current total of the policy engine
// counter over the series carrying every selected label, for a later step to
// compare against.
func (m *MetricsSteps) iNotePolicyEngineCounter(name, selector string) error {
	total, err := policyEngineCounterTotal(name, unescapeGherkinQuotes(selector))
	if err != nil {
		return err
	}
	m.state.SetContextValue(notedCounterContextKeyPrefix+name+"|"+selector, total)
	return nil
}

// policyEngineCounterShouldHaveGrown polls the counter's total until it has
// grown by at least minGrowth since it was noted.
func (m *MetricsSteps) policyEngineCounterShouldHaveGrown(name, selector string, minGrowth, seconds int) error {
	raw, ok := m.state.GetContextValue(notedCounterContextKeyPrefix + name + "|" + selector)
	if !ok {
		return fmt.Errorf("the policy engine counter %q for series labelled %s was not noted before", name, selector)
	}
	before := raw.(float64)
	deadline := time.Now().Add(time.Duration(seconds) * time.Second)
	for {
		total, err := policyEngineCounterTotal(name, unescapeGherkinQuotes(selector))
		if err == nil && total-before >= float64(minGrowth) {
			return nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return err
			}
			return fmt.Errorf("the policy engine counter %q for series labelled %s grew by %g within %ds, expected at least %d", name, selector, total-before, seconds, minGrowth)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// policyEngineCounterTotal sums the counter over the policy engine's series
// whose name is name or ends in "_"+name and whose labels include every pair
// in selector.
func policyEngineCounterTotal(name, selector string) (float64, error) {
	want := map[string]string{}
	for _, pair := range metricLabelPattern.FindAllStringSubmatch(selector, -1) {
		want[pair[1]] = pair[2]
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%s/metrics", PolicyEngineMetricsPort))
	if err != nil {
		return 0, fmt.Errorf("failed to read policy engine metrics: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("policy engine metrics returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read policy engine metrics: %w", err)
	}
	total := 0.0
	for _, line := range strings.Split(string(body), "\n") {
		open := strings.IndexByte(line, '{')
		closing := strings.LastIndexByte(line, '}')
		if strings.HasPrefix(line, "#") || open < 0 || closing < open {
			continue
		}
		series := line[:open]
		if series != name && !strings.HasSuffix(series, "_"+name) {
			continue
		}
		labels := map[string]string{}
		for _, pair := range metricLabelPattern.FindAllStringSubmatch(line[open+1:closing], -1) {
			labels[pair[1]] = pair[2]
		}
		matches := true
		for k, v := range want {
			if labels[k] != v {
				matches = false
				break
			}
		}
		if !matches {
			continue
		}
		fields := strings.Fields(line[closing+1:])
		if len(fields) == 0 {
			continue
		}
		value, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			return 0, fmt.Errorf("unreadable value on metric line %q: %w", line, err)
		}
		total += value
	}
	return total, nil
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

// unescapeGherkinQuotes turns `\"` in a captured step argument back into a
// bare quote. Godog keeps step text verbatim, so an escaped quote inside a
// quoted argument (e.g. a Prometheus label value) arrives with its backslash.
func unescapeGherkinQuotes(s string) string {
	return strings.ReplaceAll(s, `\"`, `"`)
}

// theResponseShouldContainMetric verifies the response contains a specific metric
func (m *MetricsSteps) theResponseShouldContainMetric(metricName string) error {
	metricName = unescapeGherkinQuotes(metricName)
	body := m.httpSteps.LastBody()
	bodyStr := string(body)

	if !strings.Contains(bodyStr, metricName) {
		return fmt.Errorf("response does not contain metric '%s'", metricName)
	}

	return nil
}

// theResponseShouldNotContainMetric verifies the response does not contain a
// specific metric or metric series.
func (m *MetricsSteps) theResponseShouldNotContainMetric(metricName string) error {
	metricName = unescapeGherkinQuotes(metricName)
	body := m.httpSteps.LastBody()
	bodyStr := string(body)

	if strings.Contains(bodyStr, metricName) {
		return fmt.Errorf("response unexpectedly contains metric '%s'", metricName)
	}

	return nil
}
