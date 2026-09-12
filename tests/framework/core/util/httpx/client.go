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

// Response is a fully read HTTP exchange.
type Response struct {
	StatusCode int
	Body       []byte
	Headers    http.Header

	// Request describes the call that produced the response.
	Method string
	URL    string

	// Elapsed is the duration of the request and body read.
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

// RequireSuccessWithBody checks that a response can be parsed as a successful non-empty response.
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

// Client is the shared HTTP transport layer.
type Client struct {
	http    *http.Client
	retryOn []TransientMatcher
}

// TransientMatcher identifies responses that may be retried by Client.
type TransientMatcher func(*Response) bool

// Options configure a client.
type Options struct {
	// Timeout bounds one request.
	Timeout time.Duration
	// FollowRedirects enables normal HTTP redirect handling.
	FollowRedirects bool
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

	httpClient := &http.Client{
		Timeout: opts.Timeout,
		Transport: &http.Transport{
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
			MaxIdleConns:        200,
			MaxIdleConnsPerHost: 50,
			MaxConnsPerHost:     100,
		},
	}
	if !opts.FollowRedirects {
		httpClient.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return &Client{
		http:    httpClient,
		retryOn: append([]TransientMatcher(nil), opts.RetryOn...),
	}
}

// Request describes one call.
type Request struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    []byte
	// ContentType is a convenience; an explicit Content-Type header wins.
	ContentType string

	// Host overrides the HTTP Host header. Empty uses the URL host.
	Host string
}

// Do issues a request and retries only responses recognized as transient.
func (c *Client) Do(ctx context.Context, req Request, maxRetries int, retryDelay time.Duration) (*Response, error) {
	if req.Method == "" {
		req.Method = http.MethodGet
	}
	if strings.TrimSpace(req.URL) == "" {
		return nil, fmt.Errorf("httpx: request has no URL")
	}

	var last *Response
	attempts := maxRetries + 1
	if attempts < 1 {
		attempts = 1
	}

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

// TransientByCodeInBody recognizes a transient response by status and body marker.
func TransientByCodeInBody(marker string) TransientMatcher {
	return func(r *Response) bool {
		if r == nil || r.StatusCode < 500 {
			return false
		}
		return bytes.Contains(r.Body, []byte(marker))
	}
}
