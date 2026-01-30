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

	"github.com/stretchr/testify/assert"
	policyenginev1 "github.com/wso2/api-platform/sdk/gateway/policyengine/v1"
)

func TestStoredPolicyConfig_CompositeKey(t *testing.T) {
	tests := []struct {
		name     string
		config   StoredPolicyConfig
		expected string
	}{
		{
			name: "Standard composite key",
			config: StoredPolicyConfig{
				ID: "test-id",
				Configuration: policyenginev1.Configuration{
					Metadata: policyenginev1.Metadata{
						APIName: "test-api",
						Version: "v1.0",
						Context: "/test",
					},
				},
			},
			expected: "test-api:v1.0:/test",
		},
		{
			name: "Empty values",
			config: StoredPolicyConfig{
				ID: "empty-id",
				Configuration: policyenginev1.Configuration{
					Metadata: policyenginev1.Metadata{
						APIName: "",
						Version: "",
						Context: "",
					},
				},
			},
			expected: "::",
		},
		{
			name: "Special characters in values",
			config: StoredPolicyConfig{
				ID: "special-id",
				Configuration: policyenginev1.Configuration{
					Metadata: policyenginev1.Metadata{
						APIName: "my-api-name",
						Version: "v2.0.1",
						Context: "/api/v2/users",
					},
				},
			},
			expected: "my-api-name:v2.0.1:/api/v2/users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.config.CompositeKey())
		})
	}
}

func TestStoredPolicyConfig_APIName(t *testing.T) {
	config := StoredPolicyConfig{
		Configuration: policyenginev1.Configuration{
			Metadata: policyenginev1.Metadata{
				APIName: "weather-api",
			},
		},
	}
	assert.Equal(t, "weather-api", config.APIName())
}

func TestStoredPolicyConfig_APIVersion(t *testing.T) {
	config := StoredPolicyConfig{
		Configuration: policyenginev1.Configuration{
			Metadata: policyenginev1.Metadata{
				Version: "v3.0",
			},
		},
	}
	assert.Equal(t, "v3.0", config.APIVersion())
}

func TestStoredPolicyConfig_Context(t *testing.T) {
	config := StoredPolicyConfig{
		Configuration: policyenginev1.Configuration{
			Metadata: policyenginev1.Metadata{
				Context: "/weather/forecast",
			},
		},
	}
	assert.Equal(t, "/weather/forecast", config.Context())
}
