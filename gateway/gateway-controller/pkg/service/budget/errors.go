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

package budget

import (
	"errors"
	"fmt"
	"strings"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
)

var (
	// ErrNotFound is returned when no budget has the given name.
	ErrNotFound = errors.New("budget not found")
	// ErrReadOnly is returned when changing a budget that came from the control plane.
	ErrReadOnly = errors.New("the budget comes from the control plane and is read-only here")
	// ErrPublish is returned when a change is saved but the policy engine could not be updated.
	ErrPublish = errors.New("budget saved but failed to update the policy engine")
)

// ValidationError lists why a budget was rejected.
type ValidationError struct {
	Errors []config.ValidationError
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Errors))
	for _, fe := range e.Errors {
		if fe.Field == "" {
			parts = append(parts, fe.Message)
		} else {
			parts = append(parts, fe.Field+": "+fe.Message)
		}
	}
	return "invalid budget: " + strings.Join(parts, "; ")
}

// ParseError wraps a body that is not a JSON or YAML document.
type ParseError struct {
	Cause error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("failed to parse budget: %v", e.Cause)
}

func (e *ParseError) Unwrap() error {
	return e.Cause
}

// NameMismatchError is returned when an update's path names one budget and the body's
// metadata.name another.
type NameMismatchError struct {
	PathName string
	BodyName string
}

func (e *NameMismatchError) Error() string {
	return fmt.Sprintf("name mismatch: path has '%s' but metadata.name has '%s'", e.PathName, e.BodyName)
}
