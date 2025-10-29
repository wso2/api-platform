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
)

// RetryableHTTPClient wraps an HTTP client with retry logic
type RetryableHTTPClient struct {
	client     *http.Client
	maxRetries int
	timeout    time.Duration
}

// NewRetryableHTTPClient creates a new HTTP client with retry capabilities
//
// Parameters:
//   - maxRetries: Maximum number of retry attempts (e.g., 3 for spec requirement)
//   - timeout: Timeout duration for each HTTP request (e.g., 15 seconds)
//
// Returns:
//   - *RetryableHTTPClient: A configured HTTP client with retry logic
func NewRetryableHTTPClient(maxRetries int, timeout time.Duration) *RetryableHTTPClient {
	return &RetryableHTTPClient{
		client: &http.Client{
			Timeout: timeout,
		},
		maxRetries: maxRetries,
		timeout:    timeout,
	}
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