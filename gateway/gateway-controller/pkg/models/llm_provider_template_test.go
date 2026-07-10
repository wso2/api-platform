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
			assert.Equal(t, tt.expected, tt.template.GetHandle())
		})
	}
}

func newTemplate(name string, groupID, version, managedBy *string) *StoredLLMProviderTemplate {
	return &StoredLLMProviderTemplate{
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: name},
			Spec: api.LLMProviderTemplateData{
				GroupId:   groupID,
				Version:   version,
				ManagedBy: managedBy,
			},
		},
	}
}

func ptr(s string) *string { return &s }

func TestStoredLLMProviderTemplate_GetGroupID(t *testing.T) {
	// Explicit groupId is returned verbatim.
	tmpl := newTemplate("openai-v2", ptr("openai"), nil, nil)
	assert.Equal(t, "openai", tmpl.GetGroupID())

	// Falls back to metadata.name (the handle) when groupId is unset or blank.
	assert.Equal(t, "openai-v2", newTemplate("openai-v2", nil, nil, nil).GetGroupID())
	assert.Equal(t, "openai-v2", newTemplate("openai-v2", ptr("   "), nil, nil).GetGroupID())
}

func TestStoredLLMProviderTemplate_GetVersion(t *testing.T) {
	assert.Equal(t, "v2.0", newTemplate("openai", nil, ptr("v2.0"), nil).GetVersion())
	// Defaults to v1.0 when unset or blank.
	assert.Equal(t, DefaultTemplateVersion, newTemplate("openai", nil, nil, nil).GetVersion())
	assert.Equal(t, DefaultTemplateVersion, newTemplate("openai", nil, ptr(" "), nil).GetVersion())
}

func TestStoredLLMProviderTemplate_GetManagedBy(t *testing.T) {
	assert.Equal(t, "wso2", newTemplate("openai", nil, nil, ptr("wso2")).GetManagedBy())
	// Defaults to organization when unset or blank.
	assert.Equal(t, DefaultTemplateManagedBy, newTemplate("openai", nil, nil, nil).GetManagedBy())
	assert.Equal(t, DefaultTemplateManagedBy, newTemplate("openai", nil, nil, ptr("")).GetManagedBy())
}

// Two deployed versions of the same template carry distinct handles but share a
// groupId, so the AI workspace can present them as versions of one template.
func TestStoredLLMProviderTemplate_VersionsShareGroupID(t *testing.T) {
	v1 := newTemplate("openai-v1", ptr("openai"), ptr("v1.0"), ptr("organization"))
	v2 := newTemplate("openai-v2", ptr("openai"), ptr("v2.0"), ptr("organization"))

	assert.NotEqual(t, v1.GetHandle(), v2.GetHandle())
	assert.Equal(t, v1.GetGroupID(), v2.GetGroupID())
	assert.NotEqual(t, v1.GetVersion(), v2.GetVersion())
}
