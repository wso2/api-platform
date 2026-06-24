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
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
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
				UUID: "0000-template-1-0000-000000000000",
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
				UUID: "0000-template-2-0000-000000000000",
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
				UUID: "0000-template-3-0000-000000000000",
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
			assert.Equal(t, tt.expected, tt.template.GetGroupVersionID())
		})
	}
}

func strPtr(s string) *string {
	return &s
}

func TestStoredLLMProviderTemplate_GetVersion(t *testing.T) {
	tests := []struct {
		name     string
		version  *string
		expected string
	}{
		{name: "explicit version", version: strPtr("v2.0"), expected: "v2.0"},
		{name: "version with surrounding whitespace", version: strPtr("  v3.0  "), expected: "v3.0"},
		{name: "nil version defaults to v1.0", version: nil, expected: DefaultTemplateVersion},
		{name: "blank version defaults to v1.0", version: strPtr("   "), expected: DefaultTemplateVersion},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := &StoredLLMProviderTemplate{
				Configuration: api.LLMProviderTemplate{
					Spec: api.LLMProviderTemplateData{Version: tt.version},
				},
			}
			assert.Equal(t, tt.expected, template.GetVersion())
		})
	}
}

func TestStoredLLMProviderTemplate_GetManagedBy(t *testing.T) {
	tests := []struct {
		name      string
		managedBy *string
		expected  string
	}{
		{name: "explicit managedBy", managedBy: strPtr("wso2"), expected: "wso2"},
		{name: "managedBy with surrounding whitespace", managedBy: strPtr("  custom  "), expected: "custom"},
		{name: "nil managedBy defaults to customer", managedBy: nil, expected: DefaultTemplateManagedBy},
		{name: "blank managedBy defaults to customer", managedBy: strPtr("  "), expected: DefaultTemplateManagedBy},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := &StoredLLMProviderTemplate{
				Configuration: api.LLMProviderTemplate{
					Spec: api.LLMProviderTemplateData{ManagedBy: tt.managedBy},
				},
			}
			assert.Equal(t, tt.expected, template.GetManagedBy())
		})
	}
}

func TestStoredLLMProviderTemplate_GetID(t *testing.T) {
	template := &StoredLLMProviderTemplate{
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: "mistralai"},
			Spec:     api.LLMProviderTemplateData{Version: strPtr("v2.0")},
		},
	}
	assert.Equal(t, "mistralai-v2-0", template.GetID())
}

func TestStoredLLMProviderTemplate_GetID_DefaultsVersionWhenUnset(t *testing.T) {
	template := &StoredLLMProviderTemplate{
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: "openai"},
		},
	}
	assert.Equal(t, "openai-v1-0", template.GetID())
}

func TestMakeTemplateID(t *testing.T) {
	tests := []struct {
		name     string
		handle   string
		version  string
		expected string
	}{
		{name: "standard handle and version", handle: "openai", version: "v1.0", expected: "openai-v1-0"},
		{name: "uppercase version is lowercased", handle: "openai", version: "V2.0", expected: "openai-v2-0"},
		{name: "blank version defaults to v1.0", handle: "openai", version: "", expected: "openai-v1-0"},
		{name: "handle with surrounding whitespace is trimmed", handle: "  openai  ", version: "v1.0", expected: "openai-v1-0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, MakeTemplateID(tt.handle, tt.version))
		})
	}
}
