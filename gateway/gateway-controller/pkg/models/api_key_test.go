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

package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAPIKeyIsValid(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   APIKey
		expected bool
	}{
		{
			name: "Active key without expiration",
			apiKey: APIKey{
				Status:    APIKeyStatusActive,
				ExpiresAt: nil,
			},
			expected: true,
		},
		{
			name: "Active key with future expiration",
			apiKey: APIKey{
				Status:    APIKeyStatusActive,
				ExpiresAt: func() *time.Time { t := time.Now().Add(1 * time.Hour); return &t }(),
			},
			expected: true,
		},
		{
			name: "Active key with past expiration",
			apiKey: APIKey{
				Status:    APIKeyStatusActive,
				ExpiresAt: func() *time.Time { t := time.Now().Add(-1 * time.Hour); return &t }(),
			},
			expected: false,
		},
		{
			name: "Revoked key without expiration",
			apiKey: APIKey{
				Status:    APIKeyStatusRevoked,
				ExpiresAt: nil,
			},
			expected: false,
		},
		{
			name: "Revoked key with future expiration",
			apiKey: APIKey{
				Status:    APIKeyStatusRevoked,
				ExpiresAt: func() *time.Time { t := time.Now().Add(1 * time.Hour); return &t }(),
			},
			expected: false,
		},
		{
			name: "Expired status key",
			apiKey: APIKey{
				Status:    APIKeyStatusExpired,
				ExpiresAt: nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.apiKey.IsValid())
		})
	}
}

func TestAPIKeyIsExpired(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   APIKey
		expected bool
	}{
		{
			name: "No expiration set",
			apiKey: APIKey{
				ExpiresAt: nil,
			},
			expected: false,
		},
		{
			name: "Future expiration",
			apiKey: APIKey{
				ExpiresAt: func() *time.Time { t := time.Now().Add(1 * time.Hour); return &t }(),
			},
			expected: false,
		},
		{
			name: "Past expiration",
			apiKey: APIKey{
				ExpiresAt: func() *time.Time { t := time.Now().Add(-1 * time.Hour); return &t }(),
			},
			expected: true,
		},
		{
			name: "Expiration exactly now (edge case)",
			apiKey: APIKey{
				ExpiresAt: func() *time.Time { t := time.Now().Add(-1 * time.Millisecond); return &t }(),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.apiKey.IsExpired())
		})
	}
}

func TestAPIKeyStatusConstants(t *testing.T) {
	// Verify status constants have expected values
	assert.Equal(t, APIKeyStatus("active"), APIKeyStatusActive)
	assert.Equal(t, APIKeyStatus("revoked"), APIKeyStatusRevoked)
	assert.Equal(t, APIKeyStatus("expired"), APIKeyStatusExpired)
}

func TestConfigStatusConstants(t *testing.T) {
	// Verify config status constants have expected values
	assert.Equal(t, ConfigStatus("pending"), StatusPending)
	assert.Equal(t, ConfigStatus("deployed"), StatusDeployed)
	assert.Equal(t, ConfigStatus("failed"), StatusFailed)
}
