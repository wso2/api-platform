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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/gateway/it/steps"
)

// AnalyticsSteps wraps TestState and HTTPSteps for analytics step definitions
type AnalyticsSteps struct {
	state            *TestState
	httpSteps        *steps.HTTPSteps
	lastMatchedEvent *AnalyticsEvent // Stores the last matched event for validation steps
}

// AnalyticsEvent represents the structure of a Moesif analytics event
type AnalyticsEvent struct {
	Request struct {
		Time      string                 `json:"time"`
		URI       string                 `json:"uri"`
		Verb      string                 `json:"verb"`
		Headers   map[string]string      `json:"headers"`
		APIVersion string                `json:"api_version"`
		IPAddress string                 `json:"ip_address"`
	} `json:"request"`
	Response struct {
		Time    string            `json:"time"`
		Status  int               `json:"status"`
		Headers map[string]string `json:"headers"`
	} `json:"response"`
	Metadata map[string]interface{} `json:"metadata"`
}

// RegisterAnalyticsSteps registers all analytics step definitions
func RegisterAnalyticsSteps(ctx *godog.ScenarioContext, state *TestState, httpSteps *steps.HTTPSteps) {
	a := &AnalyticsSteps{state: state, httpSteps: httpSteps}
	
	ctx.Step(`^I reset the analytics collector$`, a.iResetTheAnalyticsCollector)
	ctx.Step(`^I wait (\d+) seconds for analytics to be published$`, a.iWaitSecondsForAnalytics)
	ctx.Step(`^the analytics collector should have received (\d+) events?$`, a.theAnalyticsCollectorShouldHaveReceivedEvents)
	ctx.Step(`^the analytics collector should have received at least (\d+) events?$`, a.theAnalyticsCollectorShouldHaveReceivedAtLeastEvents)
	ctx.Step(`^the latest analytics event should have request URI "([^"]*)"$`, a.theLatestAnalyticsEventShouldHaveRequestURI)
	ctx.Step(`^the latest analytics event should have request method "([^"]*)"$`, a.theLatestAnalyticsEventShouldHaveRequestMethod)
	ctx.Step(`^the latest analytics event should have response status (\d+)$`, a.theLatestAnalyticsEventShouldHaveResponseStatus)
	ctx.Step(`^the latest analytics event should have metadata field "([^"]*)" with value "([^"]*)"$`, a.theLatestAnalyticsEventShouldHaveMetadataField)
	ctx.Step(`^the latest analytics event should have A2A field "([^"]*)" with value "([^"]*)"$`, a.theLatestAnalyticsEventShouldHaveA2AField)
	ctx.Step(`^the latest analytics event should not have A2A field "([^"]*)"$`, a.theLatestAnalyticsEventShouldNotHaveA2AField)
	ctx.Step(`^I send a GET request to the analytics collector events endpoint$`, a.iSendGETRequestToAnalyticsCollectorEvents)
}

// iResetTheAnalyticsCollector resets all events in the mock analytics collector
func (a *AnalyticsSteps) iResetTheAnalyticsCollector() error {
	// Clear the last matched event for test isolation
	a.lastMatchedEvent = nil

	url := fmt.Sprintf("http://localhost:8086/test/reset")

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create reset request: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reset analytics collector: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("reset failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// iWaitSecondsForAnalytics waits for the specified duration to allow analytics to be published
func (a *AnalyticsSteps) iWaitSecondsForAnalytics(seconds int) error {
	time.Sleep(time.Duration(seconds) * time.Second)
	return nil
}

// theAnalyticsCollectorShouldHaveReceivedEvents verifies exact event count
func (a *AnalyticsSteps) theAnalyticsCollectorShouldHaveReceivedEvents(expectedCount int) error {
	url := fmt.Sprintf("http://localhost:8086/test/events/count")
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create count request: %w", err)
	}
	
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get event count: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("count request failed with status %d", resp.StatusCode)
	}
	
	var result map[string]int
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode count response: %w", err)
	}
	
	actualCount := result["count"]
	if actualCount != expectedCount {
		return fmt.Errorf("expected %d events, but got %d", expectedCount, actualCount)
	}
	
	return nil
}

// theAnalyticsCollectorShouldHaveReceivedAtLeastEvents verifies minimum event count
func (a *AnalyticsSteps) theAnalyticsCollectorShouldHaveReceivedAtLeastEvents(minCount int) error {
	url := fmt.Sprintf("http://localhost:8086/test/events/count")
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create count request: %w", err)
	}
	
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get event count: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("count request failed with status %d", resp.StatusCode)
	}
	
	var result map[string]int
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode count response: %w", err)
	}
	
	actualCount := result["count"]
	if actualCount < minCount {
		return fmt.Errorf("expected at least %d events, but got %d", minCount, actualCount)
	}
	
	return nil
}

// getLatestAnalyticsEvent retrieves the most recent analytics event, optionally filtered by URI
func (a *AnalyticsSteps) getLatestAnalyticsEvent(uriFilter string) (*AnalyticsEvent, error) {
	url := fmt.Sprintf("http://localhost:8086/test/events")

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create events request: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get events: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("events request failed with status %d", resp.StatusCode)
	}

	var events []AnalyticsEvent
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		return nil, fmt.Errorf("failed to decode events response: %w", err)
	}

	if len(events) == 0 {
		return nil, fmt.Errorf("no events found in analytics collector")
	}

	// If no filter provided, return the last event (backward compatible)
	if uriFilter == "" {
		return &events[len(events)-1], nil
	}

	// Filter events by URI and return the latest matching event
	// Search from end to beginning to find the most recent match
	for i := len(events) - 1; i >= 0; i-- {
		if strings.Contains(events[i].Request.URI, uriFilter) {
			return &events[i], nil
		}
	}

	return nil, fmt.Errorf("no analytics events found with URI containing '%s' (total events: %d)", uriFilter, len(events))
}

// theLatestAnalyticsEventShouldHaveRequestURI verifies the request URI in the latest event
func (a *AnalyticsSteps) theLatestAnalyticsEventShouldHaveRequestURI(expectedURI string) error {
	// Pass expectedURI as filter to get only matching events
	event, err := a.getLatestAnalyticsEvent(expectedURI)
	if err != nil {
		return err
	}

	// Store the matched event for subsequent validation steps
	a.lastMatchedEvent = event

	// URI may include query params, check if it contains the expected path
	if !strings.Contains(event.Request.URI, expectedURI) {
		return fmt.Errorf("expected URI to contain '%s', but got '%s'", expectedURI, event.Request.URI)
	}

	return nil
}

// theLatestAnalyticsEventShouldHaveRequestMethod verifies the request method in the latest event
func (a *AnalyticsSteps) theLatestAnalyticsEventShouldHaveRequestMethod(expectedMethod string) error {
	// Use the last matched event if available, otherwise fetch latest without filter
	event := a.lastMatchedEvent
	if event == nil {
		var err error
		event, err = a.getLatestAnalyticsEvent("")
		if err != nil {
			return err
		}
	}

	if event.Request.Verb != expectedMethod {
		return fmt.Errorf("expected method '%s', but got '%s'", expectedMethod, event.Request.Verb)
	}

	return nil
}

// theLatestAnalyticsEventShouldHaveResponseStatus verifies the response status in the latest event
func (a *AnalyticsSteps) theLatestAnalyticsEventShouldHaveResponseStatus(expectedStatus int) error {
	// Use the last matched event if available, otherwise fetch latest without filter
	event := a.lastMatchedEvent
	if event == nil {
		var err error
		event, err = a.getLatestAnalyticsEvent("")
		if err != nil {
			return err
		}
	}

	if event.Response.Status != expectedStatus {
		return fmt.Errorf("expected status %d, but got %d", expectedStatus, event.Response.Status)
	}

	return nil
}

// theLatestAnalyticsEventShouldHaveMetadataField verifies a metadata field in the latest event
func (a *AnalyticsSteps) theLatestAnalyticsEventShouldHaveMetadataField(fieldName, expectedValue string) error {
	// Use the last matched event if available, otherwise fetch latest without filter
	event := a.lastMatchedEvent
	if event == nil {
		var err error
		event, err = a.getLatestAnalyticsEvent("")
		if err != nil {
			return err
		}
	}

	if event.Metadata == nil {
		return fmt.Errorf("event has no metadata")
	}

	actualValue, ok := event.Metadata[fieldName]
	if !ok {
		return fmt.Errorf("metadata field '%s' not found", fieldName)
	}

	actualValueStr := fmt.Sprintf("%v", actualValue)
	if actualValueStr != expectedValue {
		return fmt.Errorf("expected metadata field '%s' to be '%s', but got '%s'", fieldName, expectedValue, actualValueStr)
	}

	return nil
}

// theLatestAnalyticsEventShouldHaveA2AField verifies a field inside the A2A
// dimension block of the latest (or last matched) event.
//
// It lives here rather than in steps_a2a.go so it shares lastMatchedEvent with
// the URI-filtering step above: an A2A scenario invokes one operation over two
// transports, so "the latest event" is ambiguous unless the scenario first
// selects one by URI, and a separately-held event would silently assert against
// the wrong one.
//
// The A2A block is nested — metadata.a2aAnalytics.<field> — unlike the flat
// metadata keys theLatestAnalyticsEventShouldHaveMetadataField reads, which is
// why that step cannot be reused.
func (a *AnalyticsSteps) theLatestAnalyticsEventShouldHaveA2AField(fieldName, expectedValue string) error {
	block, err := a.a2aAnalyticsBlock()
	if err != nil {
		return err
	}

	actualValue, ok := block[fieldName]
	if !ok {
		return fmt.Errorf("A2A analytics field '%s' not found (present: %s)", fieldName, sortedKeys(block))
	}

	actualValueStr := fmt.Sprintf("%v", actualValue)
	if actualValueStr != expectedValue {
		return fmt.Errorf("expected A2A analytics field '%s' to be '%s', but got '%s'", fieldName, expectedValue, actualValueStr)
	}
	return nil
}

// theLatestAnalyticsEventShouldNotHaveA2AField asserts a dimension is absent.
//
// A card fetch and a preflight are reported so the traffic is visible, but must
// not be shaped like an invocation — an operation or outcome on one lets a
// downstream rollup count card polling as agent traffic. Absence is the
// assertion, so it needs its own step.
func (a *AnalyticsSteps) theLatestAnalyticsEventShouldNotHaveA2AField(fieldName string) error {
	block, err := a.a2aAnalyticsBlock()
	if err != nil {
		return err
	}
	if value, ok := block[fieldName]; ok {
		return fmt.Errorf("expected no A2A analytics field '%s', but it is present with value '%v'", fieldName, value)
	}
	return nil
}

// a2aAnalyticsBlock returns the a2aAnalytics object from the selected event.
func (a *AnalyticsSteps) a2aAnalyticsBlock() (map[string]interface{}, error) {
	event := a.lastMatchedEvent
	if event == nil {
		var err error
		event, err = a.getLatestAnalyticsEvent("")
		if err != nil {
			return nil, err
		}
	}

	if event.Metadata == nil {
		return nil, fmt.Errorf("event has no metadata")
	}
	raw, ok := event.Metadata["a2aAnalytics"]
	if !ok {
		return nil, fmt.Errorf("event carries no a2aAnalytics block (metadata keys: %s)", sortedKeys(event.Metadata))
	}
	block, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("a2aAnalytics is %T, expected an object", raw)
	}
	return block, nil
}

// sortedKeys renders a map's keys for an error message, ordered so a failure is
// reproducible rather than reshuffled on every run.
func sortedKeys(m map[string]interface{}) string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return "none"
	}
	return strings.Join(keys, ", ")
}

// iSendGETRequestToAnalyticsCollectorEvents sends a GET request to the analytics collector events endpoint
func (a *AnalyticsSteps) iSendGETRequestToAnalyticsCollectorEvents() error {
	url := "http://localhost:8086/test/events"
	return a.httpSteps.SendGETRequest(url)
}
