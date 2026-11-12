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
	"strings"
)

var (
	// ErrNotFound is returned when no budget has the given name.
	ErrNotFound = errors.New("budget not found")
	// ErrReadOnly is returned when changing a budget that came from the control plane.
	ErrReadOnly = errors.New("the budget comes from the control plane and is read-only here")
)

// FieldError is one validation failure.
type FieldError struct {
	Field   string
	Message string
}

// ValidationError lists why a budget was rejected.
type ValidationError struct {
	Errors []FieldError
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
