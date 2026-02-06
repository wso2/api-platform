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

package storage

import "errors"

// Common storage errors - implementation agnostic
var (
	// ErrNotFound is returned when a configuration is not found
	ErrNotFound = errors.New("configuration not found")

	// ErrConflict is returned when a configuration with the same name/version already exists
	ErrConflict = errors.New("configuration already exists")

	// ErrDatabaseLocked is returned when the database is locked (SQLite specific)
	ErrDatabaseLocked = errors.New("database is locked")

	// ErrDatabaseUnavailable is returned when the database storage is unavailable
	ErrDatabaseUnavailable = errors.New("database storage is unavailable")

	// ErrOperationNotAllowed is returned when an operation is not permitted
	ErrOperationNotAllowed = errors.New("operation not allowed")
)

// IsConflictError checks if an error is a conflict error
// This function allows handlers to distinguish between conflict errors
// and other types of errors for appropriate logging and response handling
func IsConflictError(err error) bool {
	return errors.Is(err, ErrConflict)
}

// IsNotFoundError checks if an error is a not found error
func IsNotFoundError(err error) bool {
	return errors.Is(err, ErrNotFound)
}

func IsDatabaseUnavailableError(err error) bool {
	return errors.Is(err, ErrDatabaseUnavailable)
}

// IsOperationNotAllowedError checks if an error is an operation not allowed error
func IsOperationNotAllowedError(err error) bool {
	return errors.Is(err, ErrOperationNotAllowed)
}
