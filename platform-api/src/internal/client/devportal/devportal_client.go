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

package devportal

import (
	"fmt"
	"log"
	"time"

	"platform-api/src/config"
	"platform-api/src/internal/client"
)

// DevPortalError represents an error from developer portal operations
//
// This error type provides structured error information for intelligent
// retry logic and clear error messages to API consumers.
type DevPortalError struct {
	Code       int    // HTTP status code from developer portal
	Message    string // Human-readable error message
	Retryable  bool   // Whether the error should trigger a retry
	Underlying error  // Underlying error if any
}

// Error implements the error interface for DevPortalError
//
// Returns:
//   - string: Formatted error message including status code and message
func (e *DevPortalError) Error() string {
	if e.Code > 0 {
		return fmt.Sprintf("devportal error (%d): %s", e.Code, e.Message)
	}
	return fmt.Sprintf("devportal error: %s", e.Message)
}

// Unwrap returns the underlying error for error unwrapping
//
// Returns:
//   - error: The underlying error if present, nil otherwise
func (e *DevPortalError) Unwrap() error {
	return e.Underlying
}

// NewDevPortalError creates a new DevPortalError
//
// Parameters:
//   - code: HTTP status code (0 if not applicable)
//   - message: Error message
//   - retryable: Whether this error should trigger a retry
//   - underlying: Underlying error (can be nil)
//
// Returns:
//   - *DevPortalError: A structured error instance
func NewDevPortalError(code int, message string, retryable bool, underlying error) *DevPortalError {
	return &DevPortalError{
		Code:       code,
		Message:    message,
		Retryable:  retryable,
		Underlying: underlying,
	}
}

// IsRetryable checks if the error should trigger a retry
//
// Returns:
//   - bool: True if the error is retryable (5xx errors, network errors)
func (e *DevPortalError) IsRetryable() bool {
	return e.Retryable
}

// DevPortalClient handles HTTP communication with the developer portal
//
// This client provides methods for creating organizations, managing subscription policies,
// and publishing APIs to the developer portal with automatic retry logic.
type DevPortalClient struct {
	httpClient *client.RetryableHTTPClient // HTTP client with retry capabilities
	baseURL    string                      // Developer portal base URL (e.g., "172.17.0.1:3001")
	apiKey     string                      // Authentication API key
	enabled    bool                        // Whether developer portal integration is enabled
}

// NewDevPortalClient creates a new developer portal client from configuration
//
// Parameters:
//   - cfg: Developer portal configuration from config package
//
// Returns:
//   - *DevPortalClient: Configured client instance
//
// The client initializes with:
//   - 3 retry attempts (per spec requirement)
//   - Configured timeout duration (default 15 seconds)
//   - Base URL and API key from configuration
func NewDevPortalClient(cfg config.DevPortal) *DevPortalClient {
	// Convert timeout from seconds to duration
	timeout := time.Duration(cfg.Timeout) * time.Second

	// Create HTTP client with retry logic (max 3 retries per spec)
	httpClient := client.NewRetryableHTTPClient(3, timeout)

	// Log configuration status
	if cfg.Enabled {
		log.Printf("[DevPortal] Developer portal integration enabled. BaseURL: %s, Timeout: %d seconds",
			cfg.BaseURL, cfg.Timeout)
	} else {
		log.Printf("[DevPortal] Developer portal integration disabled")
	}

	return &DevPortalClient{
		httpClient: httpClient,
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		enabled:    cfg.Enabled,
	}
}

// IsEnabled checks if developer portal integration is enabled
//
// Returns:
//   - bool: True if integration is enabled in configuration
func (c *DevPortalClient) IsEnabled() bool {
	return c.enabled
}