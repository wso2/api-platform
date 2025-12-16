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

// Package steps provides common step definitions for BDD tests
package steps

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cucumber/godog"
)

// HTTPSteps provides common HTTP request step definitions
type HTTPSteps struct {
	client       *http.Client
	baseURLs     map[string]string
	lastRequest  *http.Request
	lastResponse *http.Response
	lastBody     []byte
	headers      map[string]string
}

// NewHTTPSteps creates a new HTTPSteps instance
func NewHTTPSteps(client *http.Client, baseURLs map[string]string) *HTTPSteps {
	return &HTTPSteps{
		client:   client,
		baseURLs: baseURLs,
		headers:  make(map[string]string),
	}
}

// Register registers all HTTP step definitions
func (h *HTTPSteps) Register(ctx *godog.ScenarioContext) {
	// Request building steps
	ctx.Step(`^I set header "([^"]*)" to "([^"]*)"$`, h.iSetHeader)
	ctx.Step(`^I set the following headers:$`, h.iSetHeaders)
	ctx.Step(`^I clear all headers$`, h.iClearHeaders)

	// HTTP method steps
	ctx.Step(`^I send a GET request to "([^"]*)"$`, h.iSendGETRequest)
	ctx.Step(`^I send a POST request to "([^"]*)"$`, h.iSendPOSTRequest)
	ctx.Step(`^I send a POST request to "([^"]*)" with body:$`, h.iSendPOSTRequestWithBody)
	ctx.Step(`^I send a PUT request to "([^"]*)" with body:$`, h.iSendPUTRequestWithBody)
	ctx.Step(`^I send a DELETE request to "([^"]*)"$`, h.iSendDELETERequest)
	ctx.Step(`^I send a PATCH request to "([^"]*)" with body:$`, h.iSendPATCHRequestWithBody)

	// Service-specific shortcuts
	ctx.Step(`^I send a GET request to the "([^"]*)" service at "([^"]*)"$`, h.iSendGETToService)
	ctx.Step(`^I send a POST request to the "([^"]*)" service at "([^"]*)" with body:$`, h.iSendPOSTToServiceWithBody)
}

// Reset clears state between scenarios
func (h *HTTPSteps) Reset() {
	h.lastRequest = nil
	h.lastResponse = nil
	h.lastBody = nil
	h.headers = make(map[string]string)
}

// LastResponse returns the last HTTP response
func (h *HTTPSteps) LastResponse() *http.Response {
	return h.lastResponse
}

// LastBody returns the last response body
func (h *HTTPSteps) LastBody() []byte {
	return h.lastBody
}

// iSetHeader sets a single header
func (h *HTTPSteps) iSetHeader(name, value string) error {
	h.headers[name] = value
	return nil
}

// iSetHeaders sets multiple headers from a table
func (h *HTTPSteps) iSetHeaders(table *godog.Table) error {
	for _, row := range table.Rows[1:] { // Skip header row
		if len(row.Cells) >= 2 {
			h.headers[row.Cells[0].Value] = row.Cells[1].Value
		}
	}
	return nil
}

// iClearHeaders clears all headers
func (h *HTTPSteps) iClearHeaders() error {
	h.headers = make(map[string]string)
	return nil
}

// iSendGETRequest sends a GET request
func (h *HTTPSteps) iSendGETRequest(url string) error {
	return h.sendRequest(http.MethodGet, url, nil)
}

// iSendPOSTRequest sends a POST request without body
func (h *HTTPSteps) iSendPOSTRequest(url string) error {
	return h.sendRequest(http.MethodPost, url, nil)
}

// iSendPOSTRequestWithBody sends a POST request with body
func (h *HTTPSteps) iSendPOSTRequestWithBody(url string, body *godog.DocString) error {
	return h.sendRequest(http.MethodPost, url, []byte(body.Content))
}

// iSendPUTRequestWithBody sends a PUT request with body
func (h *HTTPSteps) iSendPUTRequestWithBody(url string, body *godog.DocString) error {
	return h.sendRequest(http.MethodPut, url, []byte(body.Content))
}

// iSendDELETERequest sends a DELETE request
func (h *HTTPSteps) iSendDELETERequest(url string) error {
	return h.sendRequest(http.MethodDelete, url, nil)
}

// iSendPATCHRequestWithBody sends a PATCH request with body
func (h *HTTPSteps) iSendPATCHRequestWithBody(url string, body *godog.DocString) error {
	return h.sendRequest(http.MethodPatch, url, []byte(body.Content))
}

// iSendGETToService sends a GET request to a named service
func (h *HTTPSteps) iSendGETToService(serviceName, path string) error {
	baseURL, ok := h.baseURLs[serviceName]
	if !ok {
		return fmt.Errorf("unknown service: %s", serviceName)
	}
	url := baseURL + path
	return h.sendRequest(http.MethodGet, url, nil)
}

// iSendPOSTToServiceWithBody sends a POST request to a named service with body
func (h *HTTPSteps) iSendPOSTToServiceWithBody(serviceName, path string, body *godog.DocString) error {
	baseURL, ok := h.baseURLs[serviceName]
	if !ok {
		return fmt.Errorf("unknown service: %s", serviceName)
	}
	url := baseURL + path
	return h.sendRequest(http.MethodPost, url, []byte(body.Content))
}

// sendRequest is a helper to send HTTP requests
func (h *HTTPSteps) sendRequest(method, url string, body []byte) error {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Apply headers
	for name, value := range h.headers {
		req.Header.Set(name, value)
	}

	// Set Content-Type for requests with body
	if body != nil && req.Header.Get("Content-Type") == "" {
		// Auto-detect JSON
		trimmed := strings.TrimSpace(string(body))
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			req.Header.Set("Content-Type", "application/json")
		}
	}

	h.lastRequest = req

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	h.lastResponse = resp

	// Read and store body
	h.lastBody, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	return nil
}
