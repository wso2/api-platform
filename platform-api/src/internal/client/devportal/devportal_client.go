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