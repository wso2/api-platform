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
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/utils"
)

var errInsecureRedirect = errors.New("redirect to a non-HTTPS URL refused")

// retryBackoff is the fixed wait between attempts.
const retryBackoff = time.Second

// RetryableHTTPClient wraps an HTTP client with retry logic
type RetryableHTTPClient struct {
	client     *http.Client
	maxRetries int
	timeout    time.Duration
}

// NewRetryableHTTPClient creates a new HTTP client with retry capabilities.
//
// This client is built around the single shared, SSRF-guarded *http.Client the process
// constructs once at startup (see internal/utils.InitSharedHTTPClient and cmd/main.go) —
// it does not build its own httpclient.New config. It inherits the shared
// client's SSRF policy (netguard.PermitPrivateBlockMetadata() by default, operator-configurable
// via platform_api.http_client in config.toml), and additionally refuses any redirect to a
// non-HTTPS URL so a credential header is never sent in cleartext.
//
// timeout is the allowance for a single attempt. It does not vary the underlying transport;
// callers derive the request context's deadline from TotalTimeout so every attempt fits.
//
// Parameters:
//   - maxRetries: Maximum number of retry attempts (e.g., 3 for spec requirement)
//   - timeout: Time allowed for each attempt
//
// Returns:
//   - *RetryableHTTPClient: A configured HTTP client with retry logic
//   - error: if the shared HTTP client has not yet been initialized (see
//     utils.InitSharedHTTPClient, called once at process startup)
func NewRetryableHTTPClient(maxRetries int, timeout time.Duration) (*RetryableHTTPClient, error) {
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

	return &RetryableHTTPClient{
		client:     &httpClient,
		maxRetries: maxRetries,
		timeout:    timeout,
	}, nil
}

// TotalTimeout is the time all attempts and the waits between them can take, for use as
// the deadline of the context passed to Do.
func (r *RetryableHTTPClient) TotalTimeout() time.Duration {
	attempts := time.Duration(r.maxRetries + 1)
	return attempts*r.timeout + time.Duration(r.maxRetries)*retryBackoff
}

// Do executes an HTTP request with retry logic
//
// Retry behavior:
//   - Retries on network errors or 5xx server errors
//   - Does NOT retry on 4xx client errors (non-retryable)
//   - Waits retryBackoff between retries, or stops early if the request context ends
//   - Resends the full request body on every retry (via req.GetBody)
//   - Maximum attempts = maxRetries + 1 (initial attempt + retries)
//
// Parameters:
//   - req: The HTTP request to execute
//
// Returns:
//   - *http.Response: The HTTP response if successful
//   - error: Error if all retry attempts fail
func (r *RetryableHTTPClient) Do(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		if attempt > 0 && req.GetBody != nil {
			body, bodyErr := req.GetBody()
			if bodyErr != nil {
				return nil, fmt.Errorf("failed to rewind request body for retry: %w", bodyErr)
			}
			req.Body = body
		}

		// Execute the request
		resp, err = r.client.Do(req)

		// Success: no error and status code < 500
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		// Log retry attempt
		if attempt < r.maxRetries {
			if err != nil {
				log.Printf("[RetryClient] Attempt %d/%d failed with error: %v. Retrying in %s...",
					attempt+1, r.maxRetries+1, err, retryBackoff)
			} else {
				log.Printf("[RetryClient] Attempt %d/%d failed with status %d. Retrying in %s...",
					attempt+1, r.maxRetries+1, resp.StatusCode, retryBackoff)
				resp.Body.Close()
			}
			select {
			case <-time.After(retryBackoff):
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		}
	}

	// All retries exhausted
	if err != nil {
		log.Printf("[RetryClient] All %d attempts failed with error: %v", r.maxRetries+1, err)
		return nil, err
	}

	log.Printf("[RetryClient] All %d attempts failed with status %d", r.maxRetries+1, resp.StatusCode)
	return resp, nil
}
