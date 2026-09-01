/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package httpx

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Response is one HTTP exchange, fully read.
//
// The body is read eagerly and kept, because a step frequently asserts on the status and
// then parses the body, and a streaming body cannot be read twice. Test payloads are
// small; holding them is cheaper than the class of bug where a second read returns empty.
type Response struct {
	StatusCode int
	Body       []byte
	Headers    http.Header

	// Request describes what produced this, so an assertion failure can say which call
	// it is talking about.
	Method string
	URL    string

	// Elapsed is how long this one round trip took, including reading the body. A property of
	// the request rather than of the wall clock, so a retried call reports the attempt that
	// produced it and not the whole loop.
	Elapsed time.Duration
}

// Text returns the body as a string.
func (r *Response) Text() string {
	if r == nil {
		return ""
	}
	return string(r.Body)
}

// Succeeded reports a 2xx.
func (r *Response) Succeeded() bool {
	return r != nil && r.StatusCode >= 200 && r.StatusCode < 300
}

// HasBody reports a non-empty body.
func (r *Response) HasBody() bool {
	return r != nil && len(bytes.TrimSpace(r.Body)) > 0
}

// Describe renders the exchange for an assertion message.
func (r *Response) Describe() string {
	if r == nil {
		return "no response"
	}
	body := r.Text()
	const max = 512
	if len(body) > max {
		body = body[:max] + "… (truncated)"
	}
	return fmt.Sprintf("%s %s -> %d, body=%q", r.Method, r.URL, r.StatusCode, body)
}

// RequireSuccessWithBody checks that a response is usable before its body is parsed.
//
// Worth a named helper because the failure it prevents is so poor: parsing the body of a
// failed or empty response yields an opaque JSON error, several frames away from the
// actual problem, instead of a message naming the call and its status.
func (r *Response) RequireSuccessWithBody(what string) error {
	switch {
	case r == nil:
		return fmt.Errorf("%s: no response", what)
	case !r.Succeeded():
		return fmt.Errorf("%s: %s", what, r.Describe())
	case !r.HasBody():
		return fmt.Errorf("%s: succeeded with an empty body: %s", what, r.Describe())
	}
	return nil
}

// Client is the raw HTTP chokepoint.
//
// EVERY request the suites make goes through here, which is what makes it the one place to
// put a cross-cutting HTTP concern — TLS trust, connection pooling, a transparent retry of
// a known-transient product error. A step that builds its own http.Client silently opts out
// of all of it.
//
// It deliberately does NOT touch the published response the assertion steps read. That is
// the Requests layer's job, and keeping the two separate is what lets an intermediate read
// happen without disturbing the response under test.
type Client struct {
	http    *http.Client
	retryOn []TransientMatcher
}

// TransientMatcher recognises a response that should be retried rather than returned.
//
// Products under load return errors that are genuinely transient — a just-created resource
// racing an asynchronous evaluator, for instance. Absorbing those HERE, once, is what lets
// every step issue a single call and assert on it. A step that hand-rolls a retry loop is a
// step that will eventually retry a real failure too.
type TransientMatcher func(*Response) bool

// Options configure a client.
type Options struct {
	// Timeout bounds one request.
	Timeout time.Duration
	// MaxRetries bounds transient-error retries.
	MaxRetries int
	// RetryDelay is the pause between transient retries.
	RetryDelay time.Duration
	// RetryOn recognises transient responses.
	RetryOn []TransientMatcher
}

// NewClient returns a client suitable for talking to components under test.
func NewClient(opts Options) *Client {
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.MaxRetries < 0 {
		opts.MaxRetries = 0
	}
	if opts.RetryDelay <= 0 {
		opts.RetryDelay = 2 * time.Second
	}

	return &Client{
		http: &http.Client{
			Timeout: opts.Timeout,
			Transport: &http.Transport{
				// Components present per-block generated certificates. Asserting that the
				// harness trusts them proves nothing about the product; certificate
				// behaviour is a product assertion, made by a test that says so.
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
				// Sized for many concurrent scenarios against a handful of hosts.
				MaxIdleConns:        200,
				MaxIdleConnsPerHost: 50,
				MaxConnsPerHost:     100,
			},
			CheckRedirect: func(*http.Request, []*http.Request) error {
				// Redirects are behaviour under test — several features assert on a 301/302
				// and its Location. Following them silently would make those assertions
				// impossible to write.
				return http.ErrUseLastResponse
			},
		},
		retryOn: append([]TransientMatcher(nil), opts.RetryOn...),
	}
	// maxRetries/retryDelay are captured via the closure in Do below.
}

// Request describes one call.
type Request struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    []byte
	// ContentType is a convenience; an explicit Content-Type header wins.
	ContentType string

	// Host overrides the HTTP Host header, for addressing a virtual host.
	//
	// It cannot be expressed through Headers: net/http takes the Host from Request.Host and
	// IGNORES a "Host" entry in the header map, silently. Routing tests that set it as an
	// ordinary header would therefore all address the default vhost and quietly assert
	// nothing. Empty means "use the URL's host".
	Host string
}

// Do issues a request, retrying only responses a matcher recognises as transient.
//
// A transport error is NOT retried here: it means the call did not happen, which is a
// different situation from a product answering transiently, and the retry package owns
// deadline-bounded waiting for something to become reachable.
func (c *Client) Do(ctx context.Context, req Request, maxRetries int, retryDelay time.Duration) (*Response, error) {
	if req.Method == "" {
		req.Method = http.MethodGet
	}
	if strings.TrimSpace(req.URL) == "" {
		return nil, fmt.Errorf("httpx: request has no URL")
	}

	var last *Response
	attempts := maxRetries + 1

	for attempt := range attempts {
		resp, err := c.once(ctx, req)
		if err != nil {
			return nil, err
		}
		last = resp

		if !c.isTransient(resp) {
			return resp, nil
		}
		if attempt == attempts-1 {
			break
		}
		select {
		case <-ctx.Done():
			return last, ctx.Err()
		case <-time.After(retryDelay):
		}
	}

	// Returned rather than errored: the caller may legitimately want to assert on it.
	return last, nil
}

func (c *Client) once(ctx context.Context, req Request) (*Response, error) {
	var body io.Reader
	if len(req.Body) > 0 {
		body = bytes.NewReader(req.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, body)
	if err != nil {
		return nil, fmt.Errorf("httpx: building %s %s: %w", req.Method, req.URL, err)
	}
	if req.ContentType != "" {
		httpReq.Header.Set("Content-Type", req.ContentType)
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}
	if h := strings.TrimSpace(req.Host); h != "" {
		httpReq.Host = h
	}

	started := time.Now()

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("httpx: %s %s: %w", req.Method, req.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("httpx: reading the body of %s %s: %w", req.Method, req.URL, err)
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       raw,
		Headers:    resp.Header.Clone(),
		Method:     req.Method,
		URL:        req.URL,
		Elapsed:    time.Since(started),
	}, nil
}

func (c *Client) isTransient(resp *Response) bool {
	for _, match := range c.retryOn {
		if match(resp) {
			return true
		}
	}
	return false
}

// TransientByCodeInBody recognises a product error carried in the response body.
//
// Matching on BOTH a status class and a body marker, deliberately narrowly: a matcher that
// retried every 5xx would also retry a real server failure, turning a fast clear failure
// into a slow confusing one.
func TransientByCodeInBody(marker string) TransientMatcher {
	return func(r *Response) bool {
		if r == nil || r.StatusCode < 500 {
			return false
		}
		return bytes.Contains(r.Body, []byte(marker))
	}
}
