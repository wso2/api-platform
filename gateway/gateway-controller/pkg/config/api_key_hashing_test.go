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

package config

import (
	"testing"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
)

func TestValidateAPIKeyHashingConfig(t *testing.T) {
	tests := []struct {
		name        string
		enabled     bool
		algorithm   string
		expectError bool
	}{
		{
			name:        "hashing disabled",
			enabled:     false,
			algorithm:   "",
			expectError: false,
		},
		{
			name:        "hashing disabled with algorithm set",
			enabled:     false,
			algorithm:   constants.HashingAlgorithmSHA256,
			expectError: false,
		},
		{
			name:        "hashing enabled without algorithm - should default to SHA256",
			enabled:     true,
			algorithm:   "",
			expectError: false,
		},
		{
			name:        "hashing enabled with valid SHA256 algorithm",
			enabled:     true,
			algorithm:   constants.HashingAlgorithmSHA256,
			expectError: false,
		},
		{
			name:        "hashing enabled with valid bcrypt algorithm",
			enabled:     true,
			algorithm:   constants.HashingAlgorithmBcrypt,
			expectError: false,
		},
		{
			name:        "hashing enabled with valid Argon2id algorithm",
			enabled:     true,
			algorithm:   constants.HashingAlgorithmArgon2ID,
			expectError: false,
		},
		{
			name:        "hashing enabled with invalid algorithm",
			enabled:     true,
			algorithm:   "invalid-algorithm",
			expectError: true,
		},
		{
			name:        "hashing enabled with case-insensitive valid algorithm",
			enabled:     true,
			algorithm:   "SHA256", // uppercase
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal config with API key hashing settings
			config := &Config{
				GatewayController: GatewayController{
					APIKeyHashing: APIKeyHashingConfig{
						Enabled:   tt.enabled,
						Algorithm: tt.algorithm,
					},
				},
			}

			err := config.validateAPIKeyHashingConfig()

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}
