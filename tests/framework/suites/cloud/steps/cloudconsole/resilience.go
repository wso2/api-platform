/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cloudconsole

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

const (
	keyUnreachableBackendURL = "cloudUnreachableBackendURL"

	// unreachableBackend is a host the reserved .invalid top level guarantees never resolves.
	unreachableBackend = "https://backend-unavailable.invalid"

	// requestBurst is how many requests one probe of a scaled gateway sends, each on a connection
	// of its own so a load balancer can spread them.
	requestBurst = 20

	// scaledGatewayTimeout bounds how long a scaled-up gateway may take to put every replica
	// behind its endpoint and into the logs.
	scaledGatewayTimeout = 6 * time.Minute
)

// sizeUnits are the units a request body size is written in.
var sizeUnits = map[string]int{"KiB": 1 << 10, "MiB": 1 << 20}

// refusalStatuses are the statuses a gateway may refuse an oversized request with: the one HTTP
// defines for it, and the one the test plan lists.
var refusalStatuses = map[int]bool{http.StatusBadRequest: true, http.StatusRequestEntityTooLarge: true}

// leakMarkers are fragments only an unsanitised server error carries: a Go panic, a stack frame
// or an exception from a runtime behind the gateway.
var leakMarkers = []string{"panic", "goroutine", "runtime error", "stack trace", "traceback", ".go:", "exception"}

// bodyBytes converts a body size written as a count and a unit to bytes.
func bodyBytes(count int, unit string) (int, error) {
	factor, ok := sizeUnits[unit]
	if !ok {
		return 0, fmt.Errorf("unsupported body size unit %q", unit)
	}
	if count < 1 {
		return 0, fmt.Errorf("a request body size must be positive, got %d %s", count, unit)
	}
	return count * factor, nil
}

// sizedJSONBody returns a JSON object of exactly size bytes.
func sizedJSONBody(size int) ([]byte, error) {
	const prefix, suffix = `{"data":"`, `"}`
	if size < len(prefix)+len(suffix) {
		return nil, fmt.Errorf("a JSON body needs at least %d bytes, asked for %d", len(prefix)+len(suffix), size)
	}
	body := bytes.Repeat([]byte{'a'}, size)
	copy(body, prefix)
	copy(body[size-len(suffix):], suffix)
	return body, nil
}

// connectionCut reports whether a transport error is the peer ending the exchange before the
// request was fully sent, which is how a gateway that refuses a body early can look to a client.
func connectionCut(err error) bool {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	text := strings.ToLower(err.Error())
	for _, fragment := range []string{"connection reset", "broken pipe", "stream error", "stream closed", "eof"} {
		if strings.Contains(text, fragment) {
			return true
		}
	}
	return false
}

// judgeOversizedRequest decides whether the gateway refused a request body of the given size. It
// refused when it answered with a refusal status or ended the exchange before accepting the body;
// a success means the body went on to the upstream.
func judgeOversizedRequest(resp *httpx.Response, sendErr error, size int) error {
	switch {
	case resp != nil && refusalStatuses[resp.StatusCode]:
		return nil
	case resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 400:
		return fmt.Errorf("the gateway accepted a request body of %d bytes and passed it on to the upstream; no limit refused it (%s)",
			size, resp.Describe())
	case resp != nil:
		return fmt.Errorf("a request body of %d bytes was answered %s, want a refusal with 413 or 400", size, resp.Describe())
	case sendErr == nil:
		return fmt.Errorf("sending a request body of %d bytes produced no response", size)
	case connectionCut(sendErr):
		return nil
	default:
		return fmt.Errorf("sending a request body of %d bytes failed without the gateway refusing it: %w", size, sendErr)
	}
}

// sendSizedUntilStatus sends a JSON body of the given size until the gateway answers with the
// status, which is how a scenario shows a route works before it sends what is refused.
func (s *Steps) sendSizedUntilStatus(ctx context.Context, method, target string, count int, unit string, want int) error {
	size, err := bodyBytes(count, unit)
	if err != nil {
		return err
	}
	body, err := sizedJSONBody(size)
	if err != nil {
		return err
	}
	return s.sendRequestUntilStatus(ctx, httpx.Request{Method: method, URL: target, ContentType: "application/json", Body: body}, want)
}

// assertOversizedRefused sends one request body above the gateway's limit, whole or chunked, and
// requires the gateway to refuse it.
func (s *Steps) assertOversizedRefused(ctx context.Context, chunked, method, target string, count int, unit string) error {
	size, err := bodyBytes(count, unit)
	if err != nil {
		return err
	}
	body, err := sizedJSONBody(size)
	if err != nil {
		return err
	}
	request, err := s.prepareDataPlaneRequest(ctx, httpx.Request{
		Method: method, URL: target, ContentType: "application/json", Body: body,
		Chunked: strings.TrimSpace(chunked) != "", Headers: map[string]string{"Connection": "close"},
	})
	if err != nil {
		return err
	}
	resp, sendErr := s.funnel.Send(ctx, request)
	return judgeOversizedRequest(resp, sendErr, size)
}

// useUnreachableBackend records a backend that never resolves, for a deployment override.
func (s *Steps) useUnreachableBackend(ctx context.Context) error {
	return tcontext.Set(ctx, keyUnreachableBackendURL, unreachableBackend)
}

// sendUntilRouted sends a request until the gateway routes it, that is until it stops answering
// not found, and publishes that answer.
func (s *Steps) sendUntilRouted(ctx context.Context, method, target string) error {
	request, err := s.prepareDataPlaneRequest(ctx, httpx.Request{Method: method, URL: target})
	if err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{Timeout: propagationTimeout(s.topo)},
		func(ctx context.Context) (*httpx.Response, error) {
			response, sendErr := s.funnel.Send(ctx, request)
			if sendErr != nil {
				return nil, retry.Transient(sendErr)
			}
			return response, nil
		},
		func(response *httpx.Response) bool {
			return response != nil && response.StatusCode != http.StatusNotFound
		},
		fmt.Sprintf("waiting for the gateway to route %s %s", request.Method, request.URL))
}

// judgeRouterFailure decides whether a response is the router's documented answer to a backend
// it cannot reach: 503, or 504 when the backend does not answer in time, with nothing from the
// server behind it. The host the gateway could not reach must not be named either.
func judgeRouterFailure(resp *httpx.Response, backendHost string) error {
	if resp.StatusCode != http.StatusServiceUnavailable && resp.StatusCode != http.StatusGatewayTimeout {
		return fmt.Errorf("an unreachable backend was answered %s, want the router's 503 or 504", resp.Describe())
	}
	body := strings.ToLower(string(resp.Body))
	for _, marker := range leakMarkers {
		if strings.Contains(body, marker) {
			return fmt.Errorf("the router failure exposes server detail (%q): %s", marker, resp.Describe())
		}
	}
	if backendHost != "" && strings.Contains(body, strings.ToLower(backendHost)) {
		return fmt.Errorf("the router failure names the backend host %q: %s", backendHost, resp.Describe())
	}
	return nil
}

// assertRouterFailure checks the last response is the documented router failure for the
// scenario's unreachable backend.
func (s *Steps) assertRouterFailure(ctx context.Context) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	host := ""
	if backend := optionalValue(ctx, keyUnreachableBackendURL); backend != "" {
		if parsed, parseErr := url.Parse(backend); parseErr == nil {
			host = parsed.Hostname()
		}
	}
	return judgeRouterFailure(response, host)
}
