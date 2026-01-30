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
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
)

func TestStoredLLMProviderTemplate_GetHandle(t *testing.T) {
	tests := []struct {
		name     string
		template StoredLLMProviderTemplate
		expected string
	}{
		{
			name: "Standard template name",
			template: StoredLLMProviderTemplate{
				ID: "template-1",
				Configuration: api.LLMProviderTemplate{
					Metadata: api.Metadata{
						Name: "openai-template",
					},
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			expected: "openai-template",
		},
		{
			name: "Empty template name",
			template: StoredLLMProviderTemplate{
				ID: "template-2",
				Configuration: api.LLMProviderTemplate{
					Metadata: api.Metadata{
						Name: "",
					},
				},
			},
			expected: "",
		},
		{
			name: "Template name with special characters",
			template: StoredLLMProviderTemplate{
				ID: "template-3",
				Configuration: api.LLMProviderTemplate{
					Metadata: api.Metadata{
						Name: "azure-openai-gpt4-v1",
					},
				},
			},
			expected: "azure-openai-gpt4-v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.template.GetHandle())
		})
	}
}
