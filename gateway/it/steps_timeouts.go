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
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

// elapsedTimeToleranceSeconds is the tolerance when asserting minimum elapsed time (e.g. clock skew, scheduling).
const elapsedTimeToleranceSeconds = 1

// rawResponseContextKey is where the slow-header raw response is stashed for later assertion.
const rawResponseContextKey = "raw_timeout_response"

// RegisterTimeoutSteps registers step definitions for upstream and HCM timeout scenarios
func RegisterTimeoutSteps(ctx *godog.ScenarioContext, state *TestState) {
	ctx.Step(`^I record the current time as "([^"]*)"$`, func(key string) error {
		state.SetContextValue(key, time.Now())
		return nil
	})

	ctx.Step(`^the request should have taken at least "(\d+)" seconds since "([^"]*)"$`, func(expectedSecondsStr, key string) error {
		expectedSeconds, err := strconv.Atoi(expectedSecondsStr)
		if err != nil {
			return fmt.Errorf("expected seconds must be a number, got: %s", expectedSecondsStr)
		}
		val, ok := state.GetContextValue(key)
		if !ok {
			return fmt.Errorf("no start time recorded; record the current time as \"request_start\" before sending the request")
		}
		start, ok := val.(time.Time)
		if !ok {
			return fmt.Errorf("no start time recorded; record the current time as %q before sending the request", key)
		}
		elapsed := time.Since(start)
		minElapsed := time.Duration(expectedSeconds-elapsedTimeToleranceSeconds) * time.Second
		if elapsed < minElapsed {
			return fmt.Errorf("request should have taken at least %d seconds (with %ds tolerance), but elapsed time was %s",
				expectedSeconds, elapsedTimeToleranceSeconds, elapsed.Round(time.Millisecond))
		}
		return nil
	})

	ctx.Step(`^the request should have taken at most "(\d+)" seconds since "([^"]*)"$`, func(expectedSecondsStr, key string) error {
		expectedSeconds, err := strconv.Atoi(expectedSecondsStr)
		if err != nil {
			return fmt.Errorf("expected seconds must be a number, got: %s", expectedSecondsStr)
		}
		val, ok := state.GetContextValue(key)
		if !ok {
			return fmt.Errorf("no start time recorded; record the current time as \"request_start\" before sending the request")
		}
		start, ok := val.(time.Time)
		if !ok {
			return fmt.Errorf("no start time recorded; record the current time as %q before sending the request", key)
		}
		elapsed := time.Since(start)
		maxElapsed := time.Duration(expectedSeconds+elapsedTimeToleranceSeconds) * time.Second
		if elapsed > maxElapsed {
			return fmt.Errorf("request should have taken at most %d seconds (with %ds tolerance), but elapsed time was %s",
				expectedSeconds, elapsedTimeToleranceSeconds, elapsed.Round(time.Millisecond))
		}
		return nil
	})

	// Opens a raw TCP connection and sends a request line plus one header, but never
	// terminates the header block (no final blank line). This forces the HCM
	// request_headers_timeout to fire. The connection blocks until the gateway responds
	// or closes it, and the raw response is stored for assertion.
	ctx.Step(`^I open a raw connection to "([^"]*)" and send incomplete request headers for path "([^"]*)"$`, func(address, path string) error {
		conn, err := net.DialTimeout("tcp", address, 10*time.Second)
		if err != nil {
			return fmt.Errorf("failed to connect to %s: %w", address, err)
		}
		defer conn.Close()

		// Request line + Host header, with NO terminating CRLF that ends the headers.
		partial := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\n", path, address)
		if _, err := conn.Write([]byte(partial)); err != nil {
			return fmt.Errorf("failed to send partial request: %w", err)
		}

		// Read until the gateway responds and closes, bounded by a deadline that is
		// comfortably longer than request_headers_timeout so we capture the 408.
		if err := conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
			return fmt.Errorf("failed to set read deadline: %w", err)
		}
		raw, err := io.ReadAll(conn)
		if err != nil && len(raw) == 0 {
			return fmt.Errorf("failed to read response from gateway: %w", err)
		}
		state.SetContextValue(rawResponseContextKey, string(raw))
		return nil
	})

	ctx.Step(`^the raw response status code should be "([^"]*)"$`, func(expectedCode string) error {
		val, ok := state.GetContextValue(rawResponseContextKey)
		if !ok {
			return fmt.Errorf("no raw response recorded; open a raw connection first")
		}
		raw, ok := val.(string)
		if !ok {
			return fmt.Errorf("recorded raw response has unexpected type %T", val)
		}
		statusLine := raw
		if idx := strings.Index(raw, "\r\n"); idx >= 0 {
			statusLine = raw[:idx]
		}
		if !strings.Contains(statusLine, expectedCode) {
			return fmt.Errorf("expected raw response status line to contain %q, got %q", expectedCode, statusLine)
		}
		return nil
	})
}
