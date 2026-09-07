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

package subscription

import (
	"errors"
	"fmt"
)

var (
	// ErrPlanNotFound is returned when a subscription plan does not exist.
	ErrPlanNotFound = errors.New("subscription plan not found")

	// ErrSubscriptionNotFound is returned when a subscription does not exist.
	ErrSubscriptionNotFound = errors.New("subscription not found")

	// ErrAPINotFound is returned when an API identifier resolves to nothing.
	ErrAPINotFound = errors.New("api not found")
)

// Operation names carried by OpError. They exist so a handler can reproduce the
// distinct message each operation reported before this service layer existed —
// a failed load during an update said "Failed to get subscription plan", not
// "Failed to update subscription plan".
const (
	OpLoad     = "load"
	OpCreate   = "create"
	OpUpdate   = "update"
	OpDelete   = "delete"
	OpList     = "list"
	OpResolve  = "resolve"
	OpValidate = "validate"
)

// ValidationError is a request-shaped validation failure. Its Message reaches
// the caller verbatim, so it must never carry internal detail.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// NotRestAPIError is returned when an identifier resolves to a stored artifact
// that is not a RestApi. Only REST APIs bear subscriptions.
type NotRestAPIError struct {
	Identifier string
	Kind       string
}

func (e *NotRestAPIError) Error() string {
	return fmt.Sprintf("configuration with identifier '%s' is not a REST API", e.Identifier)
}

// OpError wraps an unexpected storage failure and names the stage that failed.
// It unwraps to the cause, so storage.IsConflictError and storage.IsNotFoundError
// still match through it.
type OpError struct {
	Op    string
	Cause error
}

func (e *OpError) Error() string {
	return fmt.Sprintf("subscription store %s failed: %v", e.Op, e.Cause)
}

func (e *OpError) Unwrap() error {
	return e.Cause
}
