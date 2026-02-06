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
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

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
	ctx.Step(`^I send a GET request to \"([^\"]*)\"$`, h.ISendGETRequest)
	ctx.Step(`^I send a POST request to \"([^\"]*)\"$`, h.ISendPOSTRequest)
	ctx.Step(`^I send a POST request to \"([^\"]*)\" with body:$`, h.ISendPOSTRequestWithBody)
	ctx.Step(`^I send a PUT request to \"([^\"]*)\" with body:$`, h.ISendPUTRequestWithBody)
	ctx.Step(`^I send a DELETE request to \"([^\"]*)\"$`, h.ISendDELETERequest)
	ctx.Step(`^I send a PATCH request to \"([^\"]*)\" with body:$`, h.ISendPATCHRequestWithBody)
	ctx.Step(`^I send an OPTIONS request to \"([^\"]*)\"$`, h.ISendOPTIONSRequest)
	ctx.Step(`^I send (\d+) GET requests to \"([^\"]*)\"$`, h.ISendManyGETRequests)
	ctx.Step(`^I send a GET request to \"([^\"]*)\" with header \"([^\"]*)\" value \"([^\"]*)\"$`, h.iSendGETRequestWithHeader)
	ctx.Step(`^I send (\d+) GET requests to \"([^\"]*)\" with header \"([^\"]*)\" value \"([^\"]*)\"$`, h.iSendManyGETRequestsWithHeader)
	ctx.Step(`^I send a POST request to \"([^\"]*)\" with header \"([^\"]*)\" value \"([^\"]*)\" with body:$`, h.iSendPOSTRequestWithHeaderAndBody)
	ctx.Step(`^I send a GET request to \"([^\"]*)\" with header \"([^\"]*)\"$`, h.iSendGETRequestWithHeaderPair)
	ctx.Step(`^I send a POST request to \"([^\"]*)\" with header \"([^\"]*)\" and body:$`, h.iSendPOSTRequestWithHeaderPairAndBody)

	// Service-specific shortcuts
	ctx.Step(`^I send a GET request to the "([^"]*)" service at "([^"]*)"$`, h.iSendGETToService)
	ctx.Step(`^I send a POST request to the "([^"]*)" service at "([^"]*)" with body:$`, h.iSendPOSTToServiceWithBody)
	ctx.Step(`^I send a DELETE request to the "([^"]*)" service at "([^"]*)"$`, h.iSendDELETEToService)
	ctx.Step(`^I send a PUT request to the "([^"]*)" service at "([^"]*)" with body:$`, h.iSendPUTToServiceWithBody)

	// Utility steps
	ctx.Step(`^I wait for (\d+) seconds$`, h.iWaitForSeconds)
}

// Reset clears state between scenarios
func (h *HTTPSteps) Reset() {
	h.lastRequest = nil
	h.lastResponse = nil
	h.lastBody = nil
	h.headers = make(map[string]string)
}

// SetHeader sets a header for subsequent requests
func (h *HTTPSteps) SetHeader(name, value string) {
	h.headers[name] = value
}

// SendPOSTToService sends a POST request to a named service with body
func (h *HTTPSteps) SendPOSTToService(serviceName, path string, body *godog.DocString) error {
	return h.iSendPOSTToServiceWithBody(serviceName, path, body)
}

// SendGETToService sends a GET request to a named service
func (h *HTTPSteps) SendGETToService(serviceName, path string) error {
	return h.iSendGETToService(serviceName, path)
}

// SendPUTToService sends a PUT request to a named service with body
func (h *HTTPSteps) SendPUTToService(serviceName, path string, body *godog.DocString) error {
	baseURL, ok := h.baseURLs[serviceName]
	if !ok {
		return fmt.Errorf("unknown service: %s", serviceName)
	}
	url := baseURL + path
	return h.sendRequest(http.MethodPut, url, []byte(body.Content))
}

// SendDELETEToService sends a DELETE request to a named service
func (h *HTTPSteps) SendDELETEToService(serviceName, path string) error {
	baseURL, ok := h.baseURLs[serviceName]
	if !ok {
		return fmt.Errorf("unknown service: %s", serviceName)
	}
	url := baseURL + path
	return h.sendRequest(http.MethodDelete, url, nil)
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

// ISendGETRequest sends a GET request
func (h *HTTPSteps) ISendGETRequest(url string) error {
	return h.sendRequest(http.MethodGet, url, nil)
}

// SendGETRequest is a public wrapper to send GET request to any URL
func (h *HTTPSteps) SendGETRequest(url string) error {
	return h.sendRequest(http.MethodGet, url, nil)
}

// iSendPOSTRequest sends a POST request without body
func (h *HTTPSteps) ISendPOSTRequest(url string) error {
	return h.sendRequest(http.MethodPost, url, nil)
}

// ISendPOSTRequestWithBody sends a POST request with body
func (h *HTTPSteps) ISendPOSTRequestWithBody(url string, body *godog.DocString) error {
	return h.sendRequest(http.MethodPost, url, []byte(body.Content))
}

// ISendPUTRequestWithBody sends a PUT request with body
func (h *HTTPSteps) ISendPUTRequestWithBody(url string, body *godog.DocString) error {
	return h.sendRequest(http.MethodPut, url, []byte(body.Content))
}

// ISendDELETERequest sends a DELETE request
func (h *HTTPSteps) ISendDELETERequest(url string) error {
	return h.sendRequest(http.MethodDelete, url, nil)
}

// ISendPATCHRequestWithBody sends a PATCH request with body
func (h *HTTPSteps) ISendPATCHRequestWithBody(url string, body *godog.DocString) error {
	return h.sendRequest(http.MethodPatch, url, []byte(body.Content))
}

// ISendOPTIONSRequest sends an OPTIONS request
func (h *HTTPSteps) ISendOPTIONSRequest(url string) error {
	return h.sendRequest(http.MethodOptions, url, nil)
}

// iSendManyGETRequests sends multiple GET requests
func (h *HTTPSteps) ISendManyGETRequests(count int, url string) error {
	log.Printf("DEBUG: Sending %d GET requests to %s", count, url)
	for i := 0; i < count; i++ {
		if err := h.sendRequest(http.MethodGet, url, nil); err != nil {
			return fmt.Errorf("request %d failed: %w", i+1, err)
		}
		log.Printf("DEBUG: Request %d/%d completed, last response status: %d", i+1, count, h.lastResponse.StatusCode)
	}
	log.Printf("DEBUG: All %d requests completed, final response status: %d", count, h.lastResponse.StatusCode)
	return nil
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

// iSendDELETEToService sends a DELETE request to a named service
func (h *HTTPSteps) iSendDELETEToService(serviceName, path string) error {
	baseURL, ok := h.baseURLs[serviceName]
	if !ok {
		return fmt.Errorf("unknown service: %s", serviceName)
	}
	url := baseURL + path
	return h.sendRequest(http.MethodDelete, url, nil)
}

// iSendPUTToServiceWithBody sends a PUT request to a named service with body
func (h *HTTPSteps) iSendPUTToServiceWithBody(serviceName, path string, body *godog.DocString) error {
	baseURL, ok := h.baseURLs[serviceName]
	if !ok {
		return fmt.Errorf("unknown service: %s", serviceName)
	}
	url := baseURL + path
	return h.sendRequest(http.MethodPut, url, []byte(body.Content))
}

// iWaitForSeconds waits for the specified number of seconds
func (h *HTTPSteps) iWaitForSeconds(seconds int) error {
	time.Sleep(time.Duration(seconds) * time.Second)
	return nil
}

// iSendGETRequestWithHeader sends a GET request with a specific header
func (h *HTTPSteps) iSendGETRequestWithHeader(url, headerName, headerValue string) error {
	return h.sendRequestWithTempHeader(http.MethodGet, url, nil, headerName, headerValue)
}

// iSendManyGETRequestsWithHeader sends multiple GET requests with a specific header
func (h *HTTPSteps) iSendManyGETRequestsWithHeader(count int, url, headerName, headerValue string) error {
	log.Printf("DEBUG: Sending %d GET requests to %s with header %s=%s", count, url, headerName, headerValue)
	for i := 0; i < count; i++ {
		if err := h.sendRequestWithTempHeader(http.MethodGet, url, nil, headerName, headerValue); err != nil {
			return fmt.Errorf("request %d failed: %w", i+1, err)
		}
		log.Printf("DEBUG: Request %d/%d completed, last response status: %d", i+1, count, h.lastResponse.StatusCode)
	}
	log.Printf("DEBUG: All %d requests completed, final response status: %d", count, h.lastResponse.StatusCode)
	return nil
}

// iSendPOSTRequestWithHeaderAndBody sends a POST request with a header and body
func (h *HTTPSteps) iSendPOSTRequestWithHeaderAndBody(url, headerName, headerValue string, body *godog.DocString) error {
	return h.sendRequestWithTempHeader(http.MethodPost, url, []byte(body.Content), headerName, headerValue)
}

// sendRequestWithTempHeader sends a request with a temporary header that doesn't persist
func (h *HTTPSteps) sendRequestWithTempHeader(method, url string, body []byte, headerName, headerValue string) error {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Apply persistent headers
	for name, value := range h.headers {
		req.Header.Set(name, value)
	}

	// Apply temporary header (overrides if exists)
	req.Header.Set(headerName, headerValue)

	// Set Content-Type for requests with body
	if body != nil && req.Header.Get("Content-Type") == "" {
		trimmed := strings.TrimSpace(string(body))
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			req.Header.Set("Content-Type", "application/json")
		}
	}

	h.lastRequest = req

	reqDump, _ := httputil.DumpRequestOut(req, true)
	fmt.Printf("REQUEST:\n%s\n", string(reqDump))
	log.Printf("DEBUG: Sending %s request to %s", method, url)

	resp, err := h.client.Do(req)
	if err != nil {
		log.Printf("ERROR: Failed to send request to %s: %v", url, err)
		return fmt.Errorf("failed to send request to %s: %w", url, err)
	}

	log.Printf("DEBUG: Received response from %s: status=%d", url, resp.StatusCode)
	// Log response headers
	for name, values := range resp.Header {
		for _, value := range values {
			log.Printf("DEBUG: Response header %s: %s", name, value)
		}
	}

	h.lastResponse = resp

	// Read and store body
	h.lastBody, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Log response body for debugging (truncate if too large)
	bodyStr := string(h.lastBody)
	if len(bodyStr) > 500 {
		bodyStr = bodyStr[:500] + "..."
	}
	log.Printf("DEBUG: Response body: %s", bodyStr)

	return nil
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

	reqDump, _ := httputil.DumpRequestOut(req, true)
	fmt.Printf("REQUEST:\n%s\n", string(reqDump))
	// Log the request for debugging
	log.Printf("DEBUG: Sending %s request to %s", method, url)

	resp, err := h.client.Do(req)
	if err != nil {
		log.Printf("ERROR: Failed to send request to %s: %v", url, err)
		return fmt.Errorf("failed to send request to %s: %w", url, err)
	}

	log.Printf("DEBUG: Received response from %s: status=%d", url, resp.StatusCode)
	// Log response headers
	for name, values := range resp.Header {
		for _, value := range values {
			log.Printf("DEBUG: Response header %s: %s", name, value)
		}
	}

	h.lastResponse = resp

	// Read and store body
	h.lastBody, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Log response body for debugging (truncate if too large)
	bodyStr := string(h.lastBody)
	if len(bodyStr) > 500 {
		bodyStr = bodyStr[:500] + "..."
	}
	log.Printf("DEBUG: Response body: %s", bodyStr)

	return nil
}

func (h *HTTPSteps) SendMcpRequest(url string, body *godog.DocString) error {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader([]byte(body.Content))
	}
	httpReq, err := http.NewRequest("POST", url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create init request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")

	for name, value := range h.headers {
		if name == "mcp-session-id" {
			httpReq.Header.Set(name, value)
			break
		}
	}

	h.lastRequest = httpReq

	resp, err := h.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to reach MCP server for initialize: %w", err)
	}

	h.lastResponse = resp
	defer resp.Body.Close()

	if isEventStream(resp) {
		scanner := bufio.NewScanner(resp.Body)
		foundData := false
		for scanner.Scan() {
			line := scanner.Bytes()
			if bytes.HasPrefix(line, []byte("data: ")) {
				data := bytes.TrimPrefix(line, []byte("data: "))
				data = bytes.TrimSpace(data)
				if len(data) > 0 && !bytes.Equal(data, []byte("{}")) {
					h.lastBody = data
					foundData = true
					break
				}
			}
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("error reading event stream: %w", err)
		}
		if !foundData {
			return fmt.Errorf("no data found in event stream")
		}
	} else {
		h.lastBody, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
	}

	// Check for JSON-RPC error in response
	var initResponse map[string]interface{}
	if err := json.Unmarshal(h.lastBody, &initResponse); err == nil {
		if errObj, hasError := initResponse["error"]; hasError {
			if errMap, ok := errObj.(map[string]interface{}); ok {
				if msg, ok := errMap["message"].(string); ok {
					return fmt.Errorf("initialize request returned an error: %s", msg)
				}
			}
			return fmt.Errorf("initialize request returned an error: %v", errObj)
		}
	}
	h.headers["mcp-session-id"] = resp.Header.Get("mcp-session-id")
	return nil
}

// isEventStream checks if the response is an event stream
func isEventStream(resp *http.Response) bool {
	contentType := resp.Header.Get("Content-Type")
	return bytes.Contains([]byte(contentType), []byte("text/event-stream"))
}

// iSendGETRequestWithHeaderPair sends a GET request with a header in "Name: Value" format
func (h *HTTPSteps) iSendGETRequestWithHeaderPair(url, headerPair string) error {
	// Parse "Name: Value" format
	parts := strings.SplitN(headerPair, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid header format, expected 'Name: Value', got: %s", headerPair)
	}
	headerName := strings.TrimSpace(parts[0])
	headerValue := strings.TrimSpace(parts[1])

	return h.sendRequestWithTempHeader(http.MethodGet, url, nil, headerName, headerValue)
}

// iSendPOSTRequestWithHeaderPairAndBody sends a POST request with a header in "Name: Value" format and body
func (h *HTTPSteps) iSendPOSTRequestWithHeaderPairAndBody(url, headerPair string, body *godog.DocString) error {
	// Parse "Name: Value" format
	parts := strings.SplitN(headerPair, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid header format, expected 'Name: Value', got: %s", headerPair)
	}
	headerName := strings.TrimSpace(parts[0])
	headerValue := strings.TrimSpace(parts[1])

	return h.sendRequestWithTempHeader(http.MethodPost, url, []byte(body.Content), headerName, headerValue)
}
