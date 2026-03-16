/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package restapi

import (
	"errors"
	"fmt"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
)

var (
	// ErrNotFound is returned when a REST API is not found.
	ErrNotFound = errors.New("rest api not found")

	// ErrDatabaseUnavailable is returned when the database storage is not available.
	ErrDatabaseUnavailable = errors.New("database storage not available")
)

// ValidationError wraps configuration validation errors.
type ValidationError struct {
	Errors []config.ValidationError
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("configuration validation failed (%d errors)", len(e.Errors))
}

// ParseError wraps a configuration parse failure.
type ParseError struct {
	Cause error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("failed to parse configuration: %v", e.Cause)
}

func (e *ParseError) Unwrap() error {
	return e.Cause
}

// HandleMismatchError is returned when the path handle doesn't match the YAML metadata.name.
type HandleMismatchError struct {
	PathHandle string
	YAMLHandle string
}

func (e *HandleMismatchError) Error() string {
	return fmt.Sprintf("handle mismatch: path has '%s' but YAML metadata.name has '%s'", e.PathHandle, e.YAMLHandle)
}
