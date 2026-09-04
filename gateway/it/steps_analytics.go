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
	ctx.Step(`^the latest analytics event should carry only A2A field "([^"]*)"$`, a.theLatestAnalyticsEventShouldCarryOnlyA2AField)
	ctx.Step(`^the latest analytics event should have a non-empty A2A field "([^"]*)"$`, a.theLatestAnalyticsEventShouldHaveNonEmptyA2AField)
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
// The A2A block sits under metadata.agentAnalytics.a2a, unlike the flat metadata
// keys theLatestAnalyticsEventShouldHaveMetadataField reads, which is why that
// step cannot be reused. Within that block every dimension is one flat level.
func (a *AnalyticsSteps) theLatestAnalyticsEventShouldHaveA2AField(fieldName, expectedValue string) error {
	block, err := a.a2aAnalyticsBlock()
	if err != nil {
		return err
	}

	actualValue, err := a2aField(block, fieldName)
	if err != nil {
		return err
	}

	actualValueStr := fmt.Sprintf("%v", actualValue)
	if actualValueStr != expectedValue {
		return fmt.Errorf("expected A2A analytics field '%s' to be '%s', but got '%s'", fieldName, expectedValue, actualValueStr)
	}
	return nil
}

// a2aField reads one dimension out of the A2A block.
//
// The published a2a section is one flat level, so a scenario names the dimension
// directly — "taskState", or "responseTaskId" for one of the two response
// identifiers a request field also carries.
func a2aField(block map[string]interface{}, name string) (interface{}, error) {
	value, ok := block[name]
	if !ok {
		return nil, fmt.Errorf("A2A analytics field '%s' not found in {%s}",
			name, sortedKeys(block))
	}
	return value, nil
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
	if value, err := a2aField(block, fieldName); err == nil {
		return fmt.Errorf("expected no A2A analytics field '%s', but it is present with value '%v'", fieldName, value)
	}
	return nil
}

// theLatestAnalyticsEventShouldCarryOnlyA2AField asserts the whole A2A block is one
// named dimension and nothing else.
//
// A card fetch and a preflight are the only events shaped this way, and listing the
// dimensions they must not carry would go stale the moment one is added. Asserting the
// block's entire key set is what keeps a new dimension from silently leaking onto
// them — the published model being one flat level is what makes that checkable.
func (a *AnalyticsSteps) theLatestAnalyticsEventShouldCarryOnlyA2AField(fieldName string) error {
	block, err := a.a2aAnalyticsBlock()
	if err != nil {
		return err
	}
	if len(block) != 1 || block[fieldName] == nil {
		return fmt.Errorf("expected the A2A block to carry only '%s', but it carries {%s}",
			fieldName, sortedKeys(block))
	}
	return nil
}

// theLatestAnalyticsEventShouldHaveNonEmptyA2AField asserts a dimension is present
// and carries something, without pinning what.
//
// It exists for the identifiers the agent generates — a task id, a context id — whose
// values are the agent's to choose and are different on every run. Asserting they are
// present and non-empty is the whole claim worth making about them: an absent one means
// correlation was lost, which is the failure this guards, while their actual value is
// meaningful only to whoever is correlating on it.
func (a *AnalyticsSteps) theLatestAnalyticsEventShouldHaveNonEmptyA2AField(fieldName string) error {
	block, err := a.a2aAnalyticsBlock()
	if err != nil {
		return err
	}

	value, err := a2aField(block, fieldName)
	if err != nil {
		return err
	}
	if text := fmt.Sprintf("%v", value); text == "" {
		return fmt.Errorf("A2A analytics field '%s' is present but empty", fieldName)
	}
	return nil
}

// a2aAnalyticsBlock returns the a2a section of the published Agent analytics
// envelope on the selected event.
//
// The envelope is keyed by domain — metadata.agentAnalytics.a2a — so a later Agent
// analytics domain can be added as a sibling without its fields having to be told
// apart from A2A's by name. The legacy flat metadata.a2aAnalytics key is gone; an
// event still carrying it would mean the publisher was writing both shapes, so it
// is reported rather than silently accepted as a fallback.
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
	if _, legacy := event.Metadata["a2aAnalytics"]; legacy {
		return nil, fmt.Errorf("event carries the legacy flat a2aAnalytics key, " +
			"which the agentAnalytics envelope replaced")
	}
	raw, ok := event.Metadata["agentAnalytics"]
	if !ok {
		return nil, fmt.Errorf("event carries no agentAnalytics envelope (metadata keys: %s)",
			sortedKeys(event.Metadata))
	}
	envelope, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("agentAnalytics is %T, expected an object", raw)
	}
	rawA2A, ok := envelope["a2a"]
	if !ok {
		return nil, fmt.Errorf("agentAnalytics carries no a2a section (keys: %s)", sortedKeys(envelope))
	}
	block, ok := rawA2A.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("agentAnalytics.a2a is %T, expected an object", rawA2A)
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
