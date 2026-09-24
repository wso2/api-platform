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
	// A2A is Moesif's first-class A2A block, a sibling of metadata rather than a key
	// inside it. Decoded as a map because these assertions are about the published
	// document: a typed mirror of the schema here would make a renamed or relocated
	// field compile and pass, which is the failure the step exists to catch.
	A2A map[string]interface{} `json:"a2a"`
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
// The A2A block is a first-class field of the event, a sibling of metadata rather
// than a key inside it, which is why the flat metadata-field step cannot be
// reused. Within it, a dimension is named by its path — "response.task_state".
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

// a2aField reads one dimension out of the A2A block by its published path.
//
// The block nests the two directions, so a scenario names a dimension the way the
// document spells it: "operation" at the top level, "response.task_state" inside one
// of the sub-blocks. Spelling the path out rather than searching every level is what
// makes the assertion catch a field that moved between them — a request identifier
// appearing under response is exactly the confusion the nesting exists to prevent.
func a2aField(block map[string]interface{}, path string) (interface{}, error) {
	current := block
	segments := strings.Split(path, ".")
	for _, segment := range segments[:len(segments)-1] {
		nested, ok := current[segment].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("A2A analytics field '%s' not found: no '%s' sub-block in {%s}",
				path, segment, sortedKeys(current))
		}
		current = nested
	}

	name := segments[len(segments)-1]
	value, ok := current[name]
	if !ok {
		return nil, fmt.Errorf("A2A analytics field '%s' not found in {%s}",
			path, sortedKeys(current))
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
// named dimension, plus the two the schema requires of every event, and nothing else.
//
// A card fetch and a preflight are the only events shaped this way, and listing the
// dimensions they must not carry would go stale the moment one is added. Asserting the
// block's entire key set is what keeps a new dimension from silently leaking onto them.
//
// operation and transport are exempt because the schema requires them on every event:
// neither is determined for a card fetch, so both carry their enum's catch-all. Their
// *values* are asserted here rather than their absence — a real operation appearing on
// a card fetch is precisely the leak this step guards against, and exempting the keys
// without checking what is in them would let it through.
//
// agent_id and agent_name are exempt too: they are the callee agent's identity, which
// the publisher sets on every Agent event whatever its shape. Their values are generated
// per deployment, so they are asserted non-empty rather than pinned.
func (a *AnalyticsSteps) theLatestAnalyticsEventShouldCarryOnlyA2AField(fieldName string) error {
	block, err := a.a2aAnalyticsBlock()
	if err != nil {
		return err
	}

	required := map[string]string{"operation": "Unknown", "transport": "UNKNOWN"}
	for name, catchAll := range required {
		if got, ok := block[name]; !ok || fmt.Sprintf("%v", got) != catchAll {
			return fmt.Errorf("expected the required A2A field '%s' to carry its catch-all '%s', but it carries '%v'",
				name, catchAll, got)
		}
	}

	identity := map[string]struct{}{"agent_id": {}, "agent_name": {}}
	for name := range identity {
		if got, ok := block[name]; !ok || fmt.Sprintf("%v", got) == "" {
			return fmt.Errorf("expected the A2A block to carry a non-empty agent identity field '%s', but it carries '%v'",
				name, got)
		}
	}

	extra := make(map[string]interface{})
	for name, value := range block {
		_, isIdentity := identity[name]
		if _, isRequired := required[name]; !isRequired && !isIdentity && name != fieldName {
			extra[name] = value
		}
	}
	if block[fieldName] == nil || len(extra) > 0 {
		return fmt.Errorf("expected the A2A block to carry only '%s' beside the required fields, but it carries {%s}",
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

// a2aAnalyticsBlock returns the published a2a block on the selected event.
//
// The block is a first-class field of Moesif's event schema, a sibling of metadata
// rather than a key inside it. The two metadata shapes that preceded it — the
// agentAnalytics envelope and the flat a2aAnalytics key before that — are gone; an
// event still carrying either would mean the publisher was writing the same
// dimensions twice, so both are reported rather than silently accepted as a fallback.
func (a *AnalyticsSteps) a2aAnalyticsBlock() (map[string]interface{}, error) {
	event := a.lastMatchedEvent
	if event == nil {
		var err error
		event, err = a.getLatestAnalyticsEvent("")
		if err != nil {
			return nil, err
		}
	}

	for _, retired := range []string{"a2aAnalytics", "agentAnalytics"} {
		if _, present := event.Metadata[retired]; present {
			return nil, fmt.Errorf("event carries the retired metadata key %q, "+
				"which the a2a event block replaced", retired)
		}
	}
	if event.A2A == nil {
		return nil, fmt.Errorf("event carries no a2a block (metadata keys: %s)",
			sortedKeys(event.Metadata))
	}
	return event.A2A, nil
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
