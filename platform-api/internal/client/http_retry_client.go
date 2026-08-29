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
	"log"
	"net/http"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/utils"
)

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
// it no longer builds its own independent httpclient.New config. As of this writing
// RetryableHTTPClient has no callers in this module; whoever wires it to a concrete call
// site inherits the shared client's SSRF policy (netguard.PermitPrivateBlockMetadata() by
// default, operator-configurable via platform_api.http_client in config.toml) automatically,
// rather than needing to decide on one independently.
//
// timeout is accepted for call-site compatibility (and is still used as this client's own
// Do-loop budget expectations) but no longer varies the underlying transport's construction
// — the shared client's own Timeouts.Overall is a safety-net only; a real per-call budget
// should be supplied via context.WithTimeout on the request passed to Do, matching every
// other caller of the shared client (see internal/utils/mcp.go, common.go).
//
// Parameters:
//   - maxRetries: Maximum number of retry attempts (e.g., 3 for spec requirement)
//   - timeout: Retained for call-site compatibility; see doc comment above
//
// Returns:
//   - *RetryableHTTPClient: A configured HTTP client with retry logic
//   - error: if the shared HTTP client has not yet been initialized (see
//     utils.InitSharedHTTPClient, called once at process startup)
func NewRetryableHTTPClient(maxRetries int, timeout time.Duration) (*RetryableHTTPClient, error) {
	httpClient, err := utils.NewUpstreamFetchClient(0)
	if err != nil {
		return nil, err
	}

	return &RetryableHTTPClient{
		client:     httpClient,
		maxRetries: maxRetries,
		timeout:    timeout,
	}, nil
}

// Do executes an HTTP request with retry logic
//
// Retry behavior:
//   - Retries on network errors or 5xx server errors
//   - Does NOT retry on 4xx client errors (non-retryable)
//   - Uses linear backoff (1 second between retries)
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
		// Execute the request
		resp, err = r.client.Do(req)

		// Success: no error and status code < 500
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		// Log retry attempt
		if attempt < r.maxRetries {
			if err != nil {
				log.Printf("[RetryClient] Attempt %d/%d failed with error: %v. Retrying in 1 second...",
					attempt+1, r.maxRetries+1, err)
			} else {
				log.Printf("[RetryClient] Attempt %d/%d failed with status %d. Retrying in 1 second...",
					attempt+1, r.maxRetries+1, resp.StatusCode)
			}
			time.Sleep(1 * time.Second) // Linear backoff
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
