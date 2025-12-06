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

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
)

func TestValidator_URLFriendlyName(t *testing.T) {
	validator := NewAPIValidator()

	tests := []struct {
		name        string
		apiName     string
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "valid name with spaces",
			apiName:     "Weather API",
			shouldError: false,
		},
		{
			name:        "valid name with hyphens",
			apiName:     "Weather-API",
			shouldError: false,
		},
		{
			name:        "valid name with underscores",
			apiName:     "Weather_API",
			shouldError: false,
		},
		{
			name:        "valid name with dots",
			apiName:     "Weather.API",
			shouldError: false,
		},
		{
			name:        "valid name alphanumeric",
			apiName:     "WeatherAPI123",
			shouldError: false,
		},
		{
			name:        "invalid name with slash",
			apiName:     "Weather/API",
			shouldError: true,
			errorMsg:    "API name must be URL-friendly",
		},
		{
			name:        "invalid name with question mark",
			apiName:     "Weather?API",
			shouldError: true,
			errorMsg:    "API name must be URL-friendly",
		},
		{
			name:        "invalid name with ampersand",
			apiName:     "Weather&API",
			shouldError: true,
			errorMsg:    "API name must be URL-friendly",
		},
		{
			name:        "invalid name with hash",
			apiName:     "Weather#API",
			shouldError: true,
			errorMsg:    "API name must be URL-friendly",
		},
		{
			name:        "invalid name with percent",
			apiName:     "Weather%API",
			shouldError: true,
			errorMsg:    "API name must be URL-friendly",
		},
		{
			name:        "invalid name with brackets",
			apiName:     "Weather[API]",
			shouldError: true,
			errorMsg:    "API name must be URL-friendly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specUnion := api.APIConfiguration_Spec{}
			specUnion.FromAPIConfigData(api.APIConfigData{
				Name:    tt.apiName,
				Version: "v1.0",
				Context: "/test",
				Upstreams: []api.Upstream{
					{Url: "http://example.com"},
				},
				Operations: []api.Operation{
					{Method: "GET", Path: "/test"},
				},
			})
			config := &api.APIConfiguration{
				Version: "api-platform.wso2.com/v1",
				Kind:    "http/rest",
				Spec:    specUnion,
			}

			errors := validator.Validate(config)

			// Check if we got errors when we expected them
			hasNameError := false
			for _, err := range errors {
				if err.Field == "spec.name" {
					hasNameError = true
					if tt.shouldError && tt.errorMsg != "" {
						if err.Message[:len(tt.errorMsg)] != tt.errorMsg {
							t.Errorf("Expected error message to start with '%s', got '%s'", tt.errorMsg, err.Message)
						}
					}
					break
				}
			}

			if tt.shouldError && !hasNameError {
				t.Errorf("Expected validation error for name '%s', but got none", tt.apiName)
			}

			if !tt.shouldError && hasNameError {
				t.Errorf("Did not expect validation error for name '%s', but got one", tt.apiName)
			}
		})
	}
}
