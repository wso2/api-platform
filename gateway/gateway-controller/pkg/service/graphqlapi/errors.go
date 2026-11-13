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

package graphqlapi

import (
	"errors"
	"fmt"
)

var (
	// ErrNotFound is returned when no GraphQL API has the requested handle. It is
	// deliberately not storage.ErrNotFound, so a caller can tell it apart from a
	// not-found error that comes back from the deployment itself.
	ErrNotFound = errors.New("graphql api not found")
)

// LoadError wraps an unexpected failure looking up a GraphQL API by handle.
type LoadError struct {
	Cause error
}

func (e *LoadError) Error() string {
	return fmt.Sprintf("failed to load graphql api: %v", e.Cause)
}

func (e *LoadError) Unwrap() error {
	return e.Cause
}

// ParseError wraps a failure parsing an update request body.
type ParseError struct {
	Cause error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("failed to parse configuration: %v", e.Cause)
}

func (e *ParseError) Unwrap() error {
	return e.Cause
}

// HandleMismatchError is returned when the path handle doesn't match the body's metadata.name.
type HandleMismatchError struct {
	PathHandle string
	YAMLHandle string
}

func (e *HandleMismatchError) Error() string {
	return fmt.Sprintf("handle mismatch: path has '%s' but YAML metadata.name has '%s'", e.PathHandle, e.YAMLHandle)
}

// DeleteError wraps a failure deleting a GraphQL API from the database.
type DeleteError struct {
	Cause error
}

func (e *DeleteError) Error() string {
	return fmt.Sprintf("failed to delete graphql api: %v", e.Cause)
}

func (e *DeleteError) Unwrap() error {
	return e.Cause
}
