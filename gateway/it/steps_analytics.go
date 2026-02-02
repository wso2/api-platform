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
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/gateway/it/steps"
)

// AnalyticsSteps wraps TestState and HTTPSteps for analytics step definitions
type AnalyticsSteps struct {
	state     *TestState
	httpSteps *steps.HTTPSteps
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
	ctx.Step(`^I send a GET request to the analytics collector events endpoint$`, a.iSendGETRequestToAnalyticsCollectorEvents)
}

// iResetTheAnalyticsCollector resets all events in the mock analytics collector
func (a *AnalyticsSteps) iResetTheAnalyticsCollector() error {
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

// getLatestAnalyticsEvent retrieves the most recent analytics event
func (a *AnalyticsSteps) getLatestAnalyticsEvent() (*AnalyticsEvent, error) {
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
	
	// Return the last event
	return &events[len(events)-1], nil
}

// theLatestAnalyticsEventShouldHaveRequestURI verifies the request URI in the latest event
func (a *AnalyticsSteps) theLatestAnalyticsEventShouldHaveRequestURI(expectedURI string) error {
	event, err := a.getLatestAnalyticsEvent()
	if err != nil {
		return err
	}
	
	// URI may include query params, check if it contains the expected path
	if !strings.Contains(event.Request.URI, expectedURI) {
		return fmt.Errorf("expected URI to contain '%s', but got '%s'", expectedURI, event.Request.URI)
	}
	
	return nil
}

// theLatestAnalyticsEventShouldHaveRequestMethod verifies the request method in the latest event
func (a *AnalyticsSteps) theLatestAnalyticsEventShouldHaveRequestMethod(expectedMethod string) error {
	event, err := a.getLatestAnalyticsEvent()
	if err != nil {
		return err
	}
	
	if event.Request.Verb != expectedMethod {
		return fmt.Errorf("expected method '%s', but got '%s'", expectedMethod, event.Request.Verb)
	}
	
	return nil
}

// theLatestAnalyticsEventShouldHaveResponseStatus verifies the response status in the latest event
func (a *AnalyticsSteps) theLatestAnalyticsEventShouldHaveResponseStatus(expectedStatus int) error {
	event, err := a.getLatestAnalyticsEvent()
	if err != nil {
		return err
	}
	
	if event.Response.Status != expectedStatus {
		return fmt.Errorf("expected status %d, but got %d", expectedStatus, event.Response.Status)
	}
	
	return nil
}

// theLatestAnalyticsEventShouldHaveMetadataField verifies a metadata field in the latest event
func (a *AnalyticsSteps) theLatestAnalyticsEventShouldHaveMetadataField(fieldName, expectedValue string) error {
	event, err := a.getLatestAnalyticsEvent()
	if err != nil {
		return err
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

// iSendGETRequestToAnalyticsCollectorEvents sends a GET request to the analytics collector events endpoint
func (a *AnalyticsSteps) iSendGETRequestToAnalyticsCollectorEvents() error {
	url := "http://localhost:8086/test/events"
	return a.httpSteps.SendGETRequest(url)
}
