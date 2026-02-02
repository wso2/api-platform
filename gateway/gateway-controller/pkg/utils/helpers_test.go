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

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetValueFromSourceConfig(t *testing.T) {
	tests := []struct {
		name         string
		sourceConfig any
		key          string
		expected     any
		wantErr      bool
		errContains  string
	}{
		{
			name: "Simple key lookup",
			sourceConfig: map[string]interface{}{
				"kind":    "RestApi",
				"version": "v1",
			},
			key:      "kind",
			expected: "RestApi",
			wantErr:  false,
		},
		{
			name: "Nested key lookup",
			sourceConfig: map[string]interface{}{
				"spec": map[string]interface{}{
					"template": "openai",
					"version":  "v1.0",
				},
			},
			key:      "spec.template",
			expected: "openai",
			wantErr:  false,
		},
		{
			name: "Deep nested key lookup",
			sourceConfig: map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"app": "gateway",
					},
				},
			},
			key:      "metadata.labels.app",
			expected: "gateway",
			wantErr:  false,
		},
		{
			name:         "Nil sourceConfig",
			sourceConfig: nil,
			key:          "key",
			expected:     nil,
			wantErr:      true,
			errContains:  "sourceConfig is nil",
		},
		{
			name: "Key not found",
			sourceConfig: map[string]interface{}{
				"key1": "value1",
			},
			key:         "nonexistent",
			expected:    nil,
			wantErr:     true,
			errContains: "not found",
		},
		{
			name: "Invalid nested path - not a map",
			sourceConfig: map[string]interface{}{
				"spec": "not-a-map",
			},
			key:         "spec.template",
			expected:    nil,
			wantErr:     true,
			errContains: "is not a map",
		},
		{
			name: "Numeric value lookup",
			sourceConfig: map[string]interface{}{
				"count": 42,
			},
			key:      "count",
			expected: float64(42), // JSON unmarshals numbers as float64
			wantErr:  false,
		},
		{
			name: "Boolean value lookup",
			sourceConfig: map[string]interface{}{
				"enabled": true,
			},
			key:      "enabled",
			expected: true,
			wantErr:  false,
		},
		{
			name: "Array value lookup",
			sourceConfig: map[string]interface{}{
				"items": []interface{}{"a", "b", "c"},
			},
			key:      "items",
			expected: []interface{}{"a", "b", "c"},
			wantErr:  false,
		},
		{
			name: "Nested key not found at final level",
			sourceConfig: map[string]interface{}{
				"spec": map[string]interface{}{
					"version": "v1.0",
				},
			},
			key:         "spec.nonexistent",
			expected:    nil,
			wantErr:     true,
			errContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetValueFromSourceConfig(tt.sourceConfig, tt.key)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}

func TestGetValueFromSourceConfig_StructInput(t *testing.T) {
	// Test with a struct that can be marshaled to JSON
	type TestConfig struct {
		Kind string `json:"kind"`
		Spec struct {
			Template string `json:"template"`
			Version  string `json:"version"`
		} `json:"spec"`
	}

	config := TestConfig{
		Kind: "LlmProvider",
	}
	config.Spec.Template = "openai"
	config.Spec.Version = "v1.0"

	// Test simple key
	val, err := GetValueFromSourceConfig(config, "kind")
	assert.NoError(t, err)
	assert.Equal(t, "LlmProvider", val)

	// Test nested key
	val, err = GetValueFromSourceConfig(config, "spec.template")
	assert.NoError(t, err)
	assert.Equal(t, "openai", val)
}
