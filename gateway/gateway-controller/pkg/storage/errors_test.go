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

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsConflictError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "ErrConflict",
			err:      ErrConflict,
			expected: true,
		},
		{
			name:     "Wrapped ErrConflict",
			err:      fmt.Errorf("wrapper: %w", ErrConflict),
			expected: true,
		},
		{
			name:     "ErrNotFound",
			err:      ErrNotFound,
			expected: false,
		},
		{
			name:     "Generic error",
			err:      errors.New("some error"),
			expected: false,
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsConflictError(tt.err))
		})
	}
}

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "ErrNotFound",
			err:      ErrNotFound,
			expected: true,
		},
		{
			name:     "Wrapped ErrNotFound",
			err:      fmt.Errorf("wrapper: %w", ErrNotFound),
			expected: true,
		},
		{
			name:     "ErrConflict",
			err:      ErrConflict,
			expected: false,
		},
		{
			name:     "Generic error",
			err:      errors.New("some error"),
			expected: false,
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsNotFoundError(tt.err))
		})
	}
}

func TestIsDatabaseUnavailableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "ErrDatabaseUnavailable",
			err:      ErrDatabaseUnavailable,
			expected: true,
		},
		{
			name:     "Wrapped ErrDatabaseUnavailable",
			err:      fmt.Errorf("wrapper: %w", ErrDatabaseUnavailable),
			expected: true,
		},
		{
			name:     "ErrDatabaseLocked",
			err:      ErrDatabaseLocked,
			expected: false,
		},
		{
			name:     "Generic error",
			err:      errors.New("database error"),
			expected: false,
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsDatabaseUnavailableError(tt.err))
		})
	}
}

func TestErrorMessages(t *testing.T) {
	// Verify error messages are as expected
	assert.Equal(t, "configuration not found", ErrNotFound.Error())
	assert.Equal(t, "configuration already exists", ErrConflict.Error())
	assert.Equal(t, "database is locked", ErrDatabaseLocked.Error())
	assert.Equal(t, "database storage is unavailable", ErrDatabaseUnavailable.Error())
}
