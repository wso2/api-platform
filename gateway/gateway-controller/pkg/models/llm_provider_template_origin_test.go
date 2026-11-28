/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

func templateWithAnnotations(annotations *map[string]string) *StoredLLMProviderTemplate {
	return &StoredLLMProviderTemplate{
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: "acme-llm", Annotations: annotations},
		},
	}
}

func TestStoredLLMProviderTemplate_IsFromControlPlane(t *testing.T) {
	controlPlane := map[string]string{AnnotationTemplateOrigin: string(OriginControlPlane)}
	gatewayAPI := map[string]string{AnnotationTemplateOrigin: string(OriginGatewayAPI)}
	unrelated := map[string]string{"example.com/team": "ai"}

	assert.True(t, templateWithAnnotations(&controlPlane).IsFromControlPlane())
	assert.False(t, templateWithAnnotations(&gatewayAPI).IsFromControlPlane(), "another origin is not the control plane")
	assert.False(t, templateWithAnnotations(&unrelated).IsFromControlPlane(), "other annotations alone do not mark it")
	assert.False(t, templateWithAnnotations(nil).IsFromControlPlane(), "a template with no annotations")

	var missing *StoredLLMProviderTemplate
	assert.False(t, missing.IsFromControlPlane(), "a nil template")
}
