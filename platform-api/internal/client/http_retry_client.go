/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/utils"
)

var errInsecureRedirect = errors.New("redirect to a non-HTTPS URL refused")

// RetryConfig is the retry and time policy of a RetryableHTTPClient. A request is sent
// MaxRetries+1 times at most, and AttemptTimeout runs until the response body is closed.
type RetryConfig struct {
	MaxRetries     int
	AttemptTimeout time.Duration
	Backoff        time.Duration
}

func (c RetryConfig) validate() error {
	if c.MaxRetries < 0 {
		return fmt.Errorf("retry count must not be negative (got %d)", c.MaxRetries)
	}
	if c.AttemptTimeout <= 0 {
		return fmt.Errorf("attempt timeout must be greater than zero (got %s)", c.AttemptTimeout)
	}
	if c.Backoff < 0 {
		return fmt.Errorf("retry backoff must not be negative (got %s)", c.Backoff)
	}
	return nil
}

// RetryableHTTPClient wraps an HTTP client with retry logic
type RetryableHTTPClient struct {
	client *http.Client
	cfg    RetryConfig
}

// NewRetryableHTTPClient creates a new HTTP client with retry capabilities.
//
// This client is built around the single shared, SSRF-guarded *http.Client the process
// constructs once at startup (see internal/utils.InitSharedHTTPClient) —
// it does not build its own httpclient.New config. It inherits the shared
// client's SSRF policy (netguard.PermitPrivateBlockMetadata() by default, operator-configurable
// via platform_api.http_client in config.toml), and additionally refuses any redirect to a
// non-HTTPS URL so a credential header is never sent in cleartext.
//
// Each attempt runs under its own cfg.AttemptTimeout; the request context's deadline is the
// limit for the whole call and is the caller's to set.
//
// Returns:
//   - *RetryableHTTPClient: A configured HTTP client with retry logic
//   - error: if cfg is invalid, or the shared HTTP client has not yet been initialized
func NewRetryableHTTPClient(cfg RetryConfig) (*RetryableHTTPClient, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	sharedClient, err := utils.NewUpstreamFetchClient(0)
	if err != nil {
		return nil, err
	}

	// Copy the shared client so the HTTPS-only redirect rule stays local to this client. The
	// copy keeps the same Transport, so the SSRF dial guard still applies.
	httpClient := *sharedClient
	sharedCheckRedirect := sharedClient.CheckRedirect
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		// Requests carry a credential; never follow a redirect to cleartext, regardless of the
		// shared client's allowed_schemes setting.
		if req.URL.Scheme != "https" {
			return errInsecureRedirect
		}
		if sharedCheckRedirect != nil {
			return sharedCheckRedirect(req, via)
		}
		return nil
	}

	return &RetryableHTTPClient{client: &httpClient, cfg: cfg}, nil
}

// Do executes an HTTP request with retry logic
//
// Retry behavior:
//   - Retries on network errors or 5xx server errors
//   - Does NOT retry on 4xx client errors (non-retryable)
//   - A cut-off attempt is a network error and is retried like one
//   - Waits the backoff between retries, and stops early when the request context ends or
//     has less time left than the backoff
//   - Resends the full request body on every retry (via req.GetBody); a request whose body
//     cannot be rewound is not retried
//   - Maximum attempts = MaxRetries + 1 (initial attempt + retries)
//
// The attempt's limit also covers reading the response body, so the caller must close it.
//
// Parameters:
//   - req: The HTTP request to execute
//
// Returns:
//   - *http.Response: The HTTP response if successful
//   - error: Error if all retry attempts fail
func (r *RetryableHTTPClient) Do(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	for attempt := 1; ; attempt++ {
		started := time.Now()
		resp, err := r.send(req, attempt)
		if err == nil && resp.StatusCode < http.StatusInternalServerError {
			return resp, nil
		}

		final := attempt > r.cfg.MaxRetries || !canResend(req) || !r.hasTimeToRetry(ctx)
		r.logFailure(req, attempt, time.Since(started), resp, err, final)
		if final {
			return resp, err
		}

		if resp != nil {
			resp.Body.Close()
		}
		select {
		case <-time.After(r.cfg.Backoff):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// send runs one attempt; its time limit stays in force until the response body is closed.
func (r *RetryableHTTPClient) send(req *http.Request, attempt int) (*http.Response, error) {
	attemptCtx, cancel := context.WithTimeout(req.Context(), r.cfg.AttemptTimeout)
	attemptReq := req.Clone(attemptCtx)
	if attempt > 1 && req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to rewind request body for retry: %w", err)
		}
		attemptReq.Body = body
	}

	resp, err := r.client.Do(attemptReq)
	if err != nil {
		cancel()
		return nil, err
	}
	resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

func (r *RetryableHTTPClient) hasTimeToRetry(ctx context.Context) bool {
	if ctx.Err() != nil {
		return false
	}
	deadline, ok := ctx.Deadline()
	return !ok || time.Until(deadline) > r.cfg.Backoff
}

func canResend(req *http.Request) bool {
	return req.Body == nil || req.Body == http.NoBody || req.GetBody != nil
}

func (r *RetryableHTTPClient) logFailure(req *http.Request, attempt int, elapsed time.Duration, resp *http.Response, err error, final bool) {
	outcome := "will retry"
	if final {
		outcome = "giving up"
	}
	call := fmt.Sprintf("%s %s", req.Method, req.URL.Path)
	switch {
	case err == nil:
		log.Printf("[RetryClient] %s attempt %d/%d failed with status %d after %s, %s",
			call, attempt, r.cfg.MaxRetries+1, resp.StatusCode, elapsed.Round(time.Millisecond), outcome)
	case req.Context().Err() != nil:
		log.Printf("[RetryClient] WARN %s attempt %d/%d stopped by the request deadline after %s: %v, %s",
			call, attempt, r.cfg.MaxRetries+1, elapsed.Round(time.Millisecond), err, outcome)
	case errors.Is(err, context.DeadlineExceeded):
		log.Printf("[RetryClient] WARN %s attempt %d/%d exceeded the attempt timeout of %s, %s",
			call, attempt, r.cfg.MaxRetries+1, r.cfg.AttemptTimeout, outcome)
	default:
		log.Printf("[RetryClient] %s attempt %d/%d failed after %s: %v, %s",
			call, attempt, r.cfg.MaxRetries+1, elapsed.Round(time.Millisecond), err, outcome)
	}
}

type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}
