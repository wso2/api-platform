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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cucumber/godog"
)

// sensitiveHeaderNames are redacted from logged request dumps; the request
// itself keeps the real value.
var sensitiveHeaderNames = map[string]bool{
	"authorization": true,
	"api-key":       true,
}

// sensitiveBodyFieldPattern matches common secret-bearing JSON fields (e.g.
// key-provisioning responses) so their values never reach test logs.
var sensitiveBodyFieldPattern = regexp.MustCompile(`(?i)"(api[_-]?key|token|secret|password|credential)"\s*:\s*"[^"]*"`)

// redactRequestDump masks sensitive header values in a httputil.DumpRequestOut
// dump before it's logged.
func redactRequestDump(dump []byte) []byte {
	lines := strings.Split(string(dump), "\r\n")
	for i, line := range lines {
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		if sensitiveHeaderNames[strings.ToLower(strings.TrimSpace(line[:idx]))] {
			lines[i] = line[:idx+1] + " [REDACTED]"
		}
	}
	return []byte(strings.Join(lines, "\r\n"))
}

// redactSensitiveBodyFields masks secret-bearing JSON field values before a
// response body is logged.
func redactSensitiveBodyFields(body string) string {
	return sensitiveBodyFieldPattern.ReplaceAllString(body, `"$1":"[REDACTED]"`)
}

// HTTPSteps provides common HTTP request step definitions
type HTTPSteps struct {
	client       *http.Client
	baseURLs     map[string]string
	lastRequest  *http.Request
	lastResponse *http.Response
	lastBody     []byte
	// bodyCutOff records that the response body ended before the protocol said it
	// would — the client-visible signature of a stream reset after the headers
	// were already sent. Only ever set by SendGETRequestAllowingCutOffBody.
	bodyCutOff bool
	// requestAbandoned records that the client gave up before any response arrived — a
	// caller walking away mid-request. Only ever set by SendGETRequestGivingUpAfter.
	requestAbandoned bool
	// load holds the tally from the most recent bulk drive. Kept separately from the
	// single-request fields because a load run deliberately keeps no bodies.
	load        loadTally
	headers     map[string]string
	requestHost string
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
	ctx.Step(`^I set header "([^"]*)" to the JSON response field "([^"]*)"$`, h.iSetHeaderFromJSONField)
	ctx.Step(`^I set the following headers:$`, h.iSetHeaders)
	ctx.Step(`^I clear all headers$`, h.iClearHeaders)
	ctx.Step(`^I set request host to "([^"]*)"$`, h.iSetRequestHost)

	// HTTP method steps
	ctx.Step(`^I send a GET request to \"([^\"]*)\"$`, h.ISendGETRequest)
	ctx.Step(`^I send a GET request to \"([^\"]*)\" allowing a cut-off body$`, h.SendGETRequestAllowingCutOffBody)
	ctx.Step(`^I send a POST request to \"([^\"]*)\" allowing a cut-off body with body:$`, h.SendPOSTRequestAllowingCutOffBody)
	ctx.Step(`^the response body should have been cut off$`, h.theResponseBodyShouldHaveBeenCutOff)
	ctx.Step(`^the response body should not have been cut off$`, h.theResponseBodyShouldNotHaveBeenCutOff)
	ctx.Step(`^I send a GET request to \"([^\"]*)\" and give up after (\d+)ms$`, h.SendGETRequestGivingUpAfter)
	ctx.Step(`^the request should have been abandoned$`, h.theRequestShouldHaveBeenAbandoned)
	ctx.Step(`^I drive (\d+) POST requests to \"([^\"]*)\" with body:$`, h.IDrivePOSTRequests)
	ctx.Step(`^every response status should have been (\d+)$`, h.everyResponseStatusShouldHaveBeen)
	ctx.Step(`^no response should have reported more than (\d+) attempts$`, h.noResponseShouldHaveReportedMoreThanAttempts)
	ctx.Step(`^the response should report one upstream attempt$`, h.theResponseShouldReportOneUpstreamAttempt)
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
	h.bodyCutOff = false
	h.requestAbandoned = false
	h.load = loadTally{}
	h.headers = make(map[string]string)
	h.requestHost = ""
}

// beginRequest clears the per-response flags so they describe the response this request
// is about to produce, not any earlier one in the same scenario.
//
// bodyCutOff used to be cleared only by Reset, i.e. between scenarios. A scenario that
// made a cut-off request and then a normal one would see the normal response reported as
// cut off too -- "the response body should not have been cut off" failed while naming the
// LATER response's byte count, which reads as a real truncation and is not one. Both
// assertion steps say "the response body", singular, so clearing per request is what
// their wording already promised.
func (h *HTTPSteps) beginRequest() {
	h.bodyCutOff = false
	h.requestAbandoned = false
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

// ClearHeader removes a single header from the request state.
//
// Distinct from setting it to "": an empty value is still sent, and for headers
// whose absence and emptiness mean different things — A2A-Version, where an
// empty value is read as protocol version 0.3 rather than as "unset" — the two
// are not interchangeable.
func (h *HTTPSteps) ClearHeader(name string) {
	for key := range h.headers {
		if strings.EqualFold(key, name) {
			delete(h.headers, key)
		}
	}
}

// Header returns a header the scenario has set for subsequent requests, or ""
// if it has not set one.
//
// For a step that builds its own request and needs to carry the scenario's
// credentials onto it without inheriting the rest of the header state.
func (h *HTTPSteps) Header(name string) string {
	for key, value := range h.headers {
		if strings.EqualFold(key, name) {
			return value
		}
	}
	return ""
}

// SendRequest performs an arbitrary request with the currently configured
// headers and records it as the last response.
//
// It exists so a step defined in another package can still be followed by the
// shared assertion steps, which read the last response through this type.
// Prefer the specific step methods above; this is for callers that build the
// method, URL, or body themselves — A2A's steps substitute captured task
// identifiers into all three before sending.
func (h *HTTPSteps) SendRequest(method, url string, body []byte) error {
	return h.sendRequest(method, url, body)
}

// RecordResponse installs a response captured elsewhere as the last response,
// so the shared assertion steps apply to it.
//
// Needed by any step that cannot use SendRequest because it must read the body
// incrementally rather than to completion — an SSE stream, where the point of
// the test is when each event arrived, not just the final bytes. The body
// passed here is whatever that reader chose to retain.
func (h *HTTPSteps) RecordResponse(resp *http.Response, body []byte) {
	h.beginRequest()
	h.lastResponse = resp
	h.lastBody = body
}

// iSetHeader sets a single header
func (h *HTTPSteps) iSetHeader(name, value string) error {
	h.headers[name] = value
	return nil
}

// iSetHeaderFromJSONField sets a header from a field in the last JSON response (dot notation).
func (h *HTTPSteps) iSetHeaderFromJSONField(name, field string) error {
	var data map[string]interface{}
	if err := json.Unmarshal(h.lastBody, &data); err != nil {
		return fmt.Errorf("failed to parse last response as JSON: %w", err)
	}

	var current interface{} = data
	for _, part := range strings.Split(field, ".") {
		m, ok := current.(map[string]interface{})
		if !ok {
			return fmt.Errorf("expected a JSON object while resolving field %q, got %T", field, current)
		}
		v, exists := m[part]
		if !exists {
			return fmt.Errorf("field %q not found in last JSON response", field)
		}
		current = v
	}

	value, ok := current.(string)
	if !ok {
		return fmt.Errorf("field %q is not a string (got %T)", field, current)
	}

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
	h.requestHost = ""
	return nil
}

// iSetRequestHost sets an explicit HTTP Host value for subsequent requests.
func (h *HTTPSteps) iSetRequestHost(host string) error {
	h.requestHost = strings.TrimSpace(host)
	return nil
}

func (h *HTTPSteps) resolveRequestHost(tempHeaderName, tempHeaderValue string) string {
	if h.requestHost != "" {
		return h.requestHost
	}
	if strings.EqualFold(tempHeaderName, "Host") {
		return strings.TrimSpace(tempHeaderValue)
	}
	for name, value := range h.headers {
		if strings.EqualFold(name, "Host") {
			return strings.TrimSpace(value)
		}
	}
	return ""
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
	h.beginRequest()
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
	if host := h.resolveRequestHost(headerName, headerValue); host != "" {
		req.Host = host
	}

	// Set Content-Type for requests with body
	if body != nil && req.Header.Get("Content-Type") == "" {
		trimmed := strings.TrimSpace(string(body))
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			req.Header.Set("Content-Type", "application/json")
		}
	}

	h.lastRequest = req

	reqDump, _ := httputil.DumpRequestOut(req, true)
	fmt.Printf("REQUEST:\n%s\n", string(redactRequestDump(reqDump)))
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
	bodyStr := redactSensitiveBodyFields(string(h.lastBody))
	if len(bodyStr) > 500 {
		bodyStr = bodyStr[:500] + "..."
	}
	log.Printf("DEBUG: Response body: %s", bodyStr)

	return nil
}

// sendRequest is a helper to send HTTP requests
func (h *HTTPSteps) sendRequest(method, url string, body []byte) error {
	h.beginRequest()
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
	if host := h.resolveRequestHost("", ""); host != "" {
		req.Host = host
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
	fmt.Printf("REQUEST:\n%s\n", string(redactRequestDump(reqDump)))
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
	bodyStr := redactSensitiveBodyFields(string(h.lastBody))
	if len(bodyStr) > 500 {
		bodyStr = bodyStr[:500] + "..."
	}
	log.Printf("DEBUG: Response body: %s", bodyStr)

	return nil
}

func (h *HTTPSteps) SendMcpRequest(url string, body *godog.DocString) error {
	h.beginRequest()
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
		if strings.EqualFold(name, "Authorization") || strings.EqualFold(name, "mcp-session-id") {
			log.Printf("DEBUG: Adding header %s: %s", name, value)
			httpReq.Header.Set(name, value)
		}
	}
	if host := h.resolveRequestHost("", ""); host != "" {
		httpReq.Host = host
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

	// Check for JSON-RPC error in response (skip for HTTP-level auth/conflict/rate-limit
	// errors so tests can assert on the status code and headers directly)
	skipJSONRPCCheck := resp.StatusCode == 400 || resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 409 || resp.StatusCode == 429
	if !skipJSONRPCCheck {
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
	}
	// Only overwrite the stored session ID when this response actually carries one — a
	// notification response (e.g. notifications/initialized) has no mcp-session-id header,
	// and an unconditional assignment here would clobber the session ID captured from the
	// preceding initialize response before it's used by a later request.
	if sessionID := resp.Header.Get("mcp-session-id"); sessionID != "" {
		h.headers["mcp-session-id"] = sessionID
	}
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

// SendGETRequestAllowingCutOffBody issues a GET and keeps whatever arrived even when the
// body ends early.
//
// The ordinary send fails the step on a short read, which makes one behaviour untestable:
// a response whose headers reached the client and whose body was then cut off. That is
// exactly what a post-header upstream failure looks like from outside (FR-013) — the status
// and headers are real and already delivered, and no retry is possible because a 200 cannot
// be un-sent. Status and headers are captured before the read either way, so the partial
// body is kept and the truncation is recorded as an assertable fact rather than an error.
//
// Deliberately narrow: only an early end of body is tolerated. A connection that fails
// before any response, or any other transport error, still fails the step.
func (h *HTTPSteps) SendGETRequestAllowingCutOffBody(url string) error {
	err := h.ISendGETRequest(url)
	if err == nil {
		return nil
	}
	if h.lastResponse == nil || !isUnexpectedEOF(err) {
		return err
	}
	h.bodyCutOff = true
	log.Printf("DEBUG: response body was cut off mid-stream (status=%d, %d bytes read): %v",
		h.lastResponse.StatusCode, len(h.lastBody), err)
	return nil
}

// SendPOSTRequestAllowingCutOffBody is SendGETRequestAllowingCutOffBody for the verb an
// LLM call actually uses. A provider that truncates a completion is reached by POST, so
// the GET-only variant cannot express it.
func (h *HTTPSteps) SendPOSTRequestAllowingCutOffBody(url string, body *godog.DocString) error {
	err := h.ISendPOSTRequestWithBody(url, body)
	if err == nil {
		return nil
	}
	if h.lastResponse == nil || !isTruncatedBody(err) {
		return err
	}
	h.bodyCutOff = true
	log.Printf("DEBUG: response body was cut off mid-stream (status=%d, %d bytes read): %v",
		h.lastResponse.StatusCode, len(h.lastBody), err)
	return nil
}

// isTruncatedBody reports whether err is a body that stopped before its framing said it
// would. Two shapes reach the client depending on how the upstream died and how Envoy
// passes that on: a short read of a declared length (io.ErrUnexpectedEOF), or the
// downstream connection itself being reset (ECONNRESET). A provider process dying sends
// an RST rather than a FIN, so the second shape is the one a real truncation produces and
// accepting only the first would make a truncation test fail for the wrong reason.
func isTruncatedBody(err error) bool {
	return isUnexpectedEOF(err) || errors.Is(err, syscall.ECONNRESET)
}

// isUnexpectedEOF reports whether err is the body ending before the framing said it would.
// io.ErrUnexpectedEOF covers a short read of a declared length; http.ErrBodyReadAfterClose and
// a bare io.EOF are NOT this condition and are left to fail.
func isUnexpectedEOF(err error) bool {
	return errors.Is(err, io.ErrUnexpectedEOF)
}

func (h *HTTPSteps) theResponseBodyShouldHaveBeenCutOff() error {
	if h.lastResponse == nil {
		return fmt.Errorf("no response recorded")
	}
	if !h.bodyCutOff {
		return fmt.Errorf("expected the response body to have been cut off, but it was "+
			"delivered complete (status %d, %d bytes)", h.lastResponse.StatusCode, len(h.lastBody))
	}
	return nil
}

func (h *HTTPSteps) theResponseBodyShouldNotHaveBeenCutOff() error {
	if h.lastResponse == nil {
		return fmt.Errorf("no response recorded")
	}
	if h.bodyCutOff {
		return fmt.Errorf("expected a complete response body, but it was cut off after %d bytes",
			len(h.lastBody))
	}
	return nil
}

// SendGETRequestGivingUpAfter issues a GET and walks away after the given number of
// milliseconds, the way a real caller does when it stops waiting.
//
// Cancelling the request context closes the downstream connection, which is the signal the
// gateway sees. The step succeeds when the client gave up, because that IS the condition
// under test; a response arriving first is a failed setup, not a pass, and says so.
func (h *HTTPSteps) SendGETRequestGivingUpAfter(url string, afterMs int) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(afterMs)*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to build request for %s: %w", url, err)
	}
	for name, value := range h.headers {
		req.Header.Set(name, value)
	}
	if h.requestHost != "" {
		req.Host = h.requestHost
	}

	log.Printf("DEBUG: Sending GET to %s, giving up after %dms", url, afterMs)
	start := time.Now()
	resp, err := h.client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("expected to give up before a response, but one arrived after %s with "+
			"status %d — the upstream answered too quickly for this scenario to test anything",
			time.Since(start).Round(time.Millisecond), resp.StatusCode)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("request to %s failed for a reason other than giving up: %w", url, err)
	}

	h.requestAbandoned = true
	h.lastResponse = nil
	h.lastBody = nil
	log.Printf("DEBUG: gave up after %s, no response received", time.Since(start).Round(time.Millisecond))
	return nil
}

func (h *HTTPSteps) theRequestShouldHaveBeenAbandoned() error {
	if !h.requestAbandoned {
		return fmt.Errorf("expected the client to have given up before a response arrived")
	}
	return nil
}

// loadTally is what a bulk drive keeps: counts, not payloads.
type loadTally struct {
	sent        int
	byStatus    map[int]int
	byAttempts  map[int]int
	maxAttempts int
}

// IDrivePOSTRequests sends count identical POSTs and tallies what came back.
//
// Separate from ISendManyGETRequests because the scale is different in kind: this is used for
// runs of ten thousand, where a log line per request buries the result and keeping each body
// wastes memory for data nothing reads. Only the counts survive, which is all the invariants
// below need.
//
// Connections are reused — each body is drained and closed — so the run measures the gateway
// rather than TCP setup.
func (h *HTTPSteps) IDrivePOSTRequests(count int, url string, body *godog.DocString) error {
	payload := []byte(body.Content)
	tally := loadTally{byStatus: map[int]int{}, byAttempts: map[int]int{}}

	log.Printf("DEBUG: driving %d POST requests to %s", count, url)
	start := time.Now()
	for i := 0; i < count; i++ {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("request %d: %w", i+1, err)
		}
		for name, value := range h.headers {
			req.Header.Set(name, value)
		}
		if h.requestHost != "" {
			req.Host = h.requestHost
		}

		resp, err := h.client.Do(req)
		if err != nil {
			return fmt.Errorf("request %d of %d failed after %d sent: %w", i+1, count, tally.sent, err)
		}
		// Drain before close so the connection returns to the pool.
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		tally.sent++
		tally.byStatus[resp.StatusCode]++
		attempts := attemptsFromHeader(resp)
		tally.byAttempts[attempts]++
		if attempts > tally.maxAttempts {
			tally.maxAttempts = attempts
		}

		if (i+1)%1000 == 0 {
			log.Printf("DEBUG: %d/%d sent (%s elapsed)", i+1, count, time.Since(start).Round(time.Second))
		}
	}

	h.load = tally
	log.Printf("DEBUG: drove %d requests in %s; statuses=%v attempts=%v",
		tally.sent, time.Since(start).Round(time.Second), tally.byStatus, tally.byAttempts)
	return nil
}

// attemptsFromHeader reads x-envoy-attempt-count, treating absent or unparseable as one
// attempt — the same reading the policy itself applies, so a missing header can never be
// mistaken for extra attempts.
func attemptsFromHeader(resp *http.Response) int {
	raw := resp.Header.Get("x-envoy-attempt-count")
	if raw == "" {
		return 1
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 1
	}
	return n
}

func (h *HTTPSteps) everyResponseStatusShouldHaveBeen(status int) error {
	if h.load.sent == 0 {
		return fmt.Errorf("no bulk run has been driven")
	}
	if got := h.load.byStatus[status]; got != h.load.sent {
		return fmt.Errorf("expected all %d responses to be %d, got %d of them; full distribution: %v",
			h.load.sent, status, got, h.load.byStatus)
	}
	return nil
}

// theResponseShouldReportOneUpstreamAttempt asserts that the request reached its upstream once —
// that no failover occurred — WITHOUT requiring x-envoy-attempt-count to be absent.
//
// Absence is not assertable, and that is a property of Envoy rather than of any policy.
// include_attempt_count_in_response is set per VIRTUAL HOST, so the header appears on every
// response in a vhost as soon as ANY route in it fails over. A scenario demanding the header be
// missing is therefore asserting something about which OTHER APIs happen to be deployed at that
// moment, not about the operation under test: it passes when its feature file runs alone and
// fails the moment a sibling file deploys a failover-bearing API, with no defect in either.
//
// One attempt is the real invariant, and absent reads as one (see attemptsFromHeader).
func (h *HTTPSteps) theResponseShouldReportOneUpstreamAttempt() error {
	if h.lastResponse == nil {
		return fmt.Errorf("no response available")
	}
	if n := attemptsFromHeader(h.lastResponse); n != 1 {
		return fmt.Errorf("the request was attempted %d times, want exactly 1 — a failover occurred "+
			"where the policy was expected to be inert", n)
	}
	return nil
}

func (h *HTTPSteps) noResponseShouldHaveReportedMoreThanAttempts(max int) error {
	if h.load.sent == 0 {
		return fmt.Errorf("no bulk run has been driven")
	}
	if h.load.maxAttempts > max {
		return fmt.Errorf("a request was attempted %d times, more than the %d targets in the chain — "+
			"a target was retried or the chain looped; distribution: %v",
			h.load.maxAttempts, max, h.load.byAttempts)
	}
	return nil
}
