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
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
)

func TestGetParamsOfPolicy(t *testing.T) {
	tests := []struct {
		name      string
		policyDef string
		params    []string
		expected  map[string]any
		wantErr   bool
	}{
		{
			name: "Valid YAML with parameters",
			policyDef: `
key: %s
value: %s
`,
			params: []string{"testKey", "testValue"},
			expected: map[string]any{
				"key":   "testKey",
				"value": "testValue",
			},
			wantErr: false,
		},
		{
			name: "Valid YAML without parameters",
			policyDef: `
key: staticValue
`,
			params: []string{},
			expected: map[string]any{
				"key": "staticValue",
			},
			wantErr: false,
		},
		{
			name: "Invalid YAML",
			policyDef: `
key: : value
`,
			params:   []string{},
			expected: map[string]any{},
			wantErr:  true,
		},
		{
			name: "Valid YAML with nested structure",
			policyDef: `
parent:
  child: %s
`,
			params: []string{"childValue"},
			expected: map[string]any{
				"parent": map[string]any{
					"child": "childValue",
				},
			},
			wantErr: false,
		},
		{
			name:      "Valid MODIFY_HEADERS_POLICY_PARAMS",
			policyDef: constants.MODIFY_HEADERS_POLICY_PARAMS,
			params:    []string{"Authorization", "Bearer token"},
			expected: map[string]any{
				"requestHeaders": []any{
					map[string]any{
						"action": "SET",
						"name":   "Authorization",
						"value":  "Bearer token",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetParamsOfPolicy(tt.policyDef, tt.params...)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}
