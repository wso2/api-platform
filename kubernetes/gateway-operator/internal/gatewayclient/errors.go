/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package gatewayclient

import "net/http"

// RetryableError indicates an error that should be retried.
type RetryableError struct {
	Err        error
	StatusCode int
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

// NonRetryableError indicates an error that should not be retried.
type NonRetryableError struct {
	Err        error
	StatusCode int
}

func (e *NonRetryableError) Error() string {
	return e.Err.Error()
}

// IsRetryableStatusCode determines if an HTTP status code is retryable.
func IsRetryableStatusCode(code int) bool {
	switch code {
	case http.StatusInternalServerError,
		http.StatusServiceUnavailable,
		http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}
