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

func TestValidateAPIKeyConfig(t *testing.T) {
	tests := []struct {
		name                 string
		apiKeysPerUserPerAPI int
		algorithm            string
		expectError          bool
	}{
		{
			name:                 "no hashing (empty algorithm)",
			apiKeysPerUserPerAPI: 10,
			algorithm:            "",
			expectError:          false,
		},
		{
			name:                 "valid SHA256 algorithm",
			apiKeysPerUserPerAPI: 5,
			algorithm:            constants.HashingAlgorithmSHA256,
			expectError:          false,
		},
		{
			name:                 "valid bcrypt algorithm",
			apiKeysPerUserPerAPI: 15,
			algorithm:            constants.HashingAlgorithmBcrypt,
			expectError:          false,
		},
		{
			name:                 "valid Argon2id algorithm",
			apiKeysPerUserPerAPI: 20,
			algorithm:            constants.HashingAlgorithmArgon2ID,
			expectError:          false,
		},
		{
			name:                 "invalid algorithm",
			apiKeysPerUserPerAPI: 10,
			algorithm:            "invalid-algorithm",
			expectError:          true,
		},
		{
			name:                 "case-insensitive valid algorithm",
			apiKeysPerUserPerAPI: 10,
			algorithm:            "SHA256", // uppercase
			expectError:          false,
		},
		{
			name:                 "zero api keys per user per api",
			apiKeysPerUserPerAPI: 0,
			algorithm:            constants.HashingAlgorithmSHA256,
			expectError:          true,
		},
		{
			name:                 "negative api keys per user per api",
			apiKeysPerUserPerAPI: -1,
			algorithm:            constants.HashingAlgorithmSHA256,
			expectError:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal config with API key hashing settings
			config := &Config{
				GatewayController: GatewayController{
					APIKey: APIKeyConfig{
						APIKeysPerUserPerAPI: tt.apiKeysPerUserPerAPI,
						Algorithm:            tt.algorithm,
					},
				},
			}

			err := config.validateAPIKeyConfig()

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
