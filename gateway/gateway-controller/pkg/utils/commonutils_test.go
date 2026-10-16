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

// SetHeadersParams must carry the header name and value through unchanged.
// The values below used to be pasted into a single-quoted YAML template, which
// doubled every '%', failed on a quote, folded newlines and let a crafted value
// add headers of its own (#3791).
func TestSetHeadersParams(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"plain", "Bearer token"},
		{"percent", "abc%2F"},
		{"single quote", "Bearer it's"},
		{"yaml injection", "y'\n    - name: 'X-Injected'\n      value: 'z"},
		{"yaml syntax", "{a: [b]} # c: d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SetHeadersParams("Authorization", tt.value)
			assert.Equal(t, map[string]any{
				"request": map[string]any{
					"headers": []any{
						map[string]any{"name": "Authorization", "value": tt.value},
					},
				},
			}, got)
		})
	}
}
