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

package steps

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

// debugHTTP returns true if HTTP_DEBUG environment variable is set
func debugHTTP() bool {
	return os.Getenv("HTTP_DEBUG") != ""
}

// debugLog logs a message if debug mode is enabled
func debugLog(format string, args ...interface{}) {
	if debugHTTP() {
		log.Printf("[HTTP_DEBUG] "+format, args...)
	}
}

// HTTPSteps provides HTTP request step definitions
type HTTPSteps struct {
	client       *http.Client
	lastRequest  *http.Request
	lastResponse *http.Response
	lastBody     []byte
	headers      map[string]string

	// Token storage for JWT tests
	Token string
}

// NewHTTPSteps creates a new HTTPSteps instance
func NewHTTPSteps(client *http.Client) *HTTPSteps {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &HTTPSteps{
		client:  client,
		headers: make(map[string]string),
	}
}

// Register registers all HTTP step definitions
func (h *HTTPSteps) Register(ctx *godog.ScenarioContext) {
	// Request building steps
	ctx.Step(`^I set header "([^"]*)" to "([^"]*)"$`, h.setHeader)
	ctx.Step(`^I clear all headers$`, h.clearHeaders)

	// HTTP method steps
	ctx.Step(`^I send a GET request to "([^"]*)"$`, h.sendGETRequest)
	ctx.Step(`^I send a POST request to "([^"]*)"$`, h.sendPOSTRequest)
	ctx.Step(`^I send a POST request to "([^"]*)" with body:$`, h.sendPOSTRequestWithBody)
	ctx.Step(`^I send a PUT request to "([^"]*)"$`, h.sendPUTRequest)
	ctx.Step(`^I send a PUT request to "([^"]*)" with body:$`, h.sendPUTRequestWithBody)
	ctx.Step(`^I send a DELETE request to "([^"]*)"$`, h.sendDELETERequest)

	// Request with header in step
	ctx.Step(`^I send a GET request with header "([^"]*)": "([^"]*)" to "([^"]*)"$`, h.sendGETRequestWithHeader)

	// Token steps for JWT tests
	ctx.Step(`^I obtain a token from "([^"]*)"$`, h.obtainToken)
	ctx.Step(`^I send a GET request with the token to "([^"]*)"$`, h.sendGETRequestWithToken)

	// HTTP method steps with retry
	ctx.Step(`^I send a GET request to "([^"]*)" expecting (\d+) not accepting ([\d,]+) with (\d+) retries$`, h.sendGETRequestWithRetry)
	ctx.Step(`^I send a POST request to "([^"]*)" expecting (\d+) not accepting ([\d,]+) with (\d+) retries$`, h.sendPOSTRequestWithRetry)
	ctx.Step(`^I send a POST request to "([^"]*)" expecting (\d+) not accepting ([\d,]+) with (\d+) retries with body:$`, h.sendPOSTRequestWithBodyAndRetry)
	ctx.Step(`^I send a PUT request to "([^"]*)" expecting (\d+) not accepting ([\d,]+) with (\d+) retries$`, h.sendPUTRequestWithRetry)
	ctx.Step(`^I send a PUT request to "([^"]*)" expecting (\d+) not accepting ([\d,]+) with (\d+) retries with body:$`, h.sendPUTRequestWithBodyAndRetry)
	ctx.Step(`^I send a DELETE request to "([^"]*)" expecting (\d+) not accepting ([\d,]+) with (\d+) retries$`, h.sendDELETERequestWithRetry)
	ctx.Step(`^I send a GET request with the token to "([^"]*)" expecting (\d+) not accepting ([\d,]+) with (\d+) retries$`, h.sendGETRequestWithTokenAndRetry)
}

// Reset clears state between scenarios
func (h *HTTPSteps) Reset() {
	h.lastRequest = nil
	h.lastResponse = nil
	h.lastBody = nil
	h.headers = make(map[string]string)
	h.Token = ""
}

// LastResponse returns the last HTTP response
func (h *HTTPSteps) LastResponse() *http.Response {
	return h.lastResponse
}

// LastBody returns the last response body
func (h *HTTPSteps) LastBody() []byte {
	return h.lastBody
}

// setHeader sets a header for subsequent requests
func (h *HTTPSteps) setHeader(name, value string) error {
	h.headers[name] = value
	return nil
}

// clearHeaders clears all headers
func (h *HTTPSteps) clearHeaders() error {
	h.headers = make(map[string]string)
	return nil
}

// sendGETRequest sends a GET request
func (h *HTTPSteps) sendGETRequest(url string) error {
	return h.sendRequest(http.MethodGet, url, nil)
}

// sendPOSTRequest sends a POST request
func (h *HTTPSteps) sendPOSTRequest(url string) error {
	return h.sendRequest(http.MethodPost, url, nil)
}

// sendPOSTRequestWithBody sends a POST request with body
func (h *HTTPSteps) sendPOSTRequestWithBody(url string, body *godog.DocString) error {
	return h.sendRequest(http.MethodPost, url, []byte(body.Content))
}

// sendPUTRequest sends a PUT request
func (h *HTTPSteps) sendPUTRequest(url string) error {
	return h.sendRequest(http.MethodPut, url, nil)
}

// sendPUTRequestWithBody sends a PUT request with body
func (h *HTTPSteps) sendPUTRequestWithBody(url string, body *godog.DocString) error {
	return h.sendRequest(http.MethodPut, url, []byte(body.Content))
}

// sendDELETERequest sends a DELETE request
func (h *HTTPSteps) sendDELETERequest(url string) error {
	return h.sendRequest(http.MethodDelete, url, nil)
}

// sendGETRequestWithHeader sends a GET request with a specific header
func (h *HTTPSteps) sendGETRequestWithHeader(headerName, headerValue, url string) error {
	// Replace ${token} placeholder with actual token
	if strings.Contains(headerValue, "${token}") {
		headerValue = strings.Replace(headerValue, "${token}", h.Token, -1)
	}
	h.headers[headerName] = headerValue
	return h.sendRequest(http.MethodGet, url, nil)
}

// obtainToken gets a JWT token from a URL
func (h *HTTPSteps) obtainToken(url string) error {
	resp, err := h.client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to obtain token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read token response: %w", err)
	}

	h.Token = strings.TrimSpace(string(body))
	return nil
}

// sendGETRequestWithToken sends a GET request with the stored token
func (h *HTTPSteps) sendGETRequestWithToken(url string) error {
	if h.Token == "" {
		return fmt.Errorf("no token available - call 'I obtain a token from' first")
	}
	h.headers["Authorization"] = "Bearer " + h.Token
	return h.sendRequest(http.MethodGet, url, nil)
}

// sendRequest is a helper to send HTTP requests
func (h *HTTPSteps) sendRequest(method, urlStr string, body []byte) error {
	debugLog("========== HTTP Request Debug ==========")
	debugLog("Method: %s", method)
	debugLog("URL: %s", urlStr)

	// Parse URL to extract host and port for diagnostics
	parsedURL, parseErr := url.Parse(urlStr)
	if parseErr != nil {
		debugLog("Failed to parse URL: %v", parseErr)
	} else {
		debugLog("Scheme: %s, Host: %s, Path: %s", parsedURL.Scheme, parsedURL.Host, parsedURL.Path)

		// Check if host:port is reachable
		host := parsedURL.Host
		if !strings.Contains(host, ":") {
			if parsedURL.Scheme == "https" {
				host = host + ":443"
			} else {
				host = host + ":80"
			}
		}
		debugLog("Checking TCP connectivity to: %s", host)
		conn, dialErr := net.DialTimeout("tcp", host, 5*time.Second)
		if dialErr != nil {
			debugLog("TCP connection FAILED: %v", dialErr)
			debugLog("Service may not be running on %s", host)
		} else {
			debugLog("TCP connection SUCCESS to %s", host)
			conn.Close()
		}
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
		debugLog("Request body length: %d bytes", len(body))
		if len(body) < 1000 {
			debugLog("Request body: %s", string(body))
		} else {
			debugLog("Request body (truncated): %s...", string(body[:500]))
		}
	} else {
		debugLog("Request body: <empty>")
	}

	req, err := http.NewRequest(method, urlStr, bodyReader)
	if err != nil {
		debugLog("Failed to create request: %v", err)
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Apply headers
	debugLog("Request headers:")
	for name, value := range h.headers {
		req.Header.Set(name, value)
		// Mask sensitive headers
		if strings.ToLower(name) == "authorization" {
			debugLog("  %s: [REDACTED]", name)
		} else {
			debugLog("  %s: %s", name, value)
		}
	}

	// Set Content-Type for requests with body
	if body != nil && req.Header.Get("Content-Type") == "" {
		trimmed := strings.TrimSpace(string(body))
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			req.Header.Set("Content-Type", "application/json")
			debugLog("  Content-Type: application/json (auto-detected)")
		}
	}

	h.lastRequest = req

	debugLog("Sending request...")
	startTime := time.Now()
	resp, err := h.client.Do(req)
	elapsed := time.Since(startTime)

	if err != nil {
		debugLog("Request FAILED after %v", elapsed)
		debugLog("Error: %v", err)

		// Provide additional diagnostics for common errors
		if strings.Contains(err.Error(), "connection refused") {
			debugLog("=== CONNECTION REFUSED DIAGNOSTICS ===")
			debugLog("The target service is not accepting connections.")
			debugLog("Possible causes:")
			debugLog("  1. Service is not running")
			debugLog("  2. Service is running on a different port")
			debugLog("  3. Firewall blocking the connection")
			debugLog("  4. Service not yet ready (startup delay)")
			if parsedURL != nil {
				debugLog("Target: %s://%s", parsedURL.Scheme, parsedURL.Host)
			}
		} else if strings.Contains(err.Error(), "timeout") {
			debugLog("=== TIMEOUT DIAGNOSTICS ===")
			debugLog("Request timed out. Client timeout: %v", h.client.Timeout)
		} else if strings.Contains(err.Error(), "no such host") {
			debugLog("=== DNS RESOLUTION FAILED ===")
			debugLog("Could not resolve hostname.")
		}
		debugLog("========================================")
		return fmt.Errorf("failed to send request: %w", err)
	}

	debugLog("Request completed in %v", elapsed)
	debugLog("Response status: %d %s", resp.StatusCode, resp.Status)
	debugLog("Response headers:")
	for name, values := range resp.Header {
		debugLog("  %s: %s", name, strings.Join(values, ", "))
	}

	h.lastResponse = resp

	// Read and store body
	h.lastBody, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		debugLog("Failed to read response body: %v", err)
		return fmt.Errorf("failed to read response body: %w", err)
	}

	debugLog("Response body length: %d bytes", len(h.lastBody))
	if len(h.lastBody) < 1000 {
		debugLog("Response body: %s", string(h.lastBody))
	} else {
		debugLog("Response body (truncated): %s...", string(h.lastBody[:500]))
	}
	debugLog("=========================================")

	return nil
}

// parseNotAcceptingCodes parses comma-separated status codes
func parseNotAcceptingCodes(codes string) []int {
	var result []int
	for _, code := range strings.Split(codes, ",") {
		if c, err := strconv.Atoi(strings.TrimSpace(code)); err == nil {
			result = append(result, c)
		}
	}
	return result
}

// sendRequestWithRetry sends a request with retry logic
func (h *HTTPSteps) sendRequestWithRetry(method, url string, body []byte,
	expectedCode int, notAcceptingCodes []int, maxRetries int) error {

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			debugLog("Retry attempt %d/%d after 2 second delay", attempt, maxRetries)
			time.Sleep(2 * time.Second)
		}

		err := h.sendRequest(method, url, body)
		if err != nil {
			lastErr = fmt.Errorf("attempt %d: request failed: %w", attempt+1, err)
			debugLog("Attempt %d failed with error: %v", attempt+1, err)
			continue
		}

		statusCode := h.lastResponse.StatusCode

		// Check for expected code (success)
		if statusCode == expectedCode {
			debugLog("Attempt %d: received expected status code %d", attempt+1, expectedCode)
			return nil
		}

		// Check for not-accepting codes (immediate failure)
		for _, code := range notAcceptingCodes {
			if statusCode == code {
				bodyStr := string(h.lastBody)
				if len(bodyStr) > 200 {
					bodyStr = bodyStr[:200] + "..."
				}
				return fmt.Errorf("received not-accepting status code %d (body: %s)",
					statusCode, bodyStr)
			}
		}

		// Otherwise, retry
		lastErr = fmt.Errorf("attempt %d: expected %d, got %d",
			attempt+1, expectedCode, statusCode)
		debugLog("Attempt %d: expected %d, got %d - will retry", attempt+1, expectedCode, statusCode)
	}

	return fmt.Errorf("max retries (%d) exceeded: %v", maxRetries, lastErr)
}

// sendGETRequestWithRetry sends a GET request with retry logic
func (h *HTTPSteps) sendGETRequestWithRetry(url string, expectedCode int,
	notAcceptingCodes string, maxRetries int) error {
	codes := parseNotAcceptingCodes(notAcceptingCodes)
	return h.sendRequestWithRetry(http.MethodGet, url, nil, expectedCode, codes, maxRetries)
}

// sendPOSTRequestWithRetry sends a POST request with retry logic
func (h *HTTPSteps) sendPOSTRequestWithRetry(url string, expectedCode int,
	notAcceptingCodes string, maxRetries int) error {
	codes := parseNotAcceptingCodes(notAcceptingCodes)
	return h.sendRequestWithRetry(http.MethodPost, url, nil, expectedCode, codes, maxRetries)
}

// sendPOSTRequestWithBodyAndRetry sends a POST request with body and retry logic
func (h *HTTPSteps) sendPOSTRequestWithBodyAndRetry(url string, expectedCode int,
	notAcceptingCodes string, maxRetries int, body *godog.DocString) error {
	codes := parseNotAcceptingCodes(notAcceptingCodes)
	return h.sendRequestWithRetry(http.MethodPost, url, []byte(body.Content), expectedCode, codes, maxRetries)
}

// sendPUTRequestWithRetry sends a PUT request with retry logic
func (h *HTTPSteps) sendPUTRequestWithRetry(url string, expectedCode int,
	notAcceptingCodes string, maxRetries int) error {
	codes := parseNotAcceptingCodes(notAcceptingCodes)
	return h.sendRequestWithRetry(http.MethodPut, url, nil, expectedCode, codes, maxRetries)
}

// sendPUTRequestWithBodyAndRetry sends a PUT request with body and retry logic
func (h *HTTPSteps) sendPUTRequestWithBodyAndRetry(url string, expectedCode int,
	notAcceptingCodes string, maxRetries int, body *godog.DocString) error {
	codes := parseNotAcceptingCodes(notAcceptingCodes)
	return h.sendRequestWithRetry(http.MethodPut, url, []byte(body.Content), expectedCode, codes, maxRetries)
}

// sendDELETERequestWithRetry sends a DELETE request with retry logic
func (h *HTTPSteps) sendDELETERequestWithRetry(url string, expectedCode int,
	notAcceptingCodes string, maxRetries int) error {
	codes := parseNotAcceptingCodes(notAcceptingCodes)
	return h.sendRequestWithRetry(http.MethodDelete, url, nil, expectedCode, codes, maxRetries)
}

// sendGETRequestWithTokenAndRetry sends a GET request with token and retry logic
func (h *HTTPSteps) sendGETRequestWithTokenAndRetry(url string, expectedCode int,
	notAcceptingCodes string, maxRetries int) error {
	if h.Token == "" {
		return fmt.Errorf("no token available - call 'I obtain a token from' first")
	}
	h.headers["Authorization"] = "Bearer " + h.Token
	codes := parseNotAcceptingCodes(notAcceptingCodes)
	return h.sendRequestWithRetry(http.MethodGet, url, nil, expectedCode, codes, maxRetries)
}
