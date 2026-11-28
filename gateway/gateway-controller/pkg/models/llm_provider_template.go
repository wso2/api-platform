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
	"strings"
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

// DefaultTemplateVersion is used when a template does not declare a version.
const DefaultTemplateVersion = "v1.0"

// DefaultTemplateManagedBy is used when a template does not declare managedBy.
const DefaultTemplateManagedBy = "organization"

// StoredLLMProviderTemplate represents the LLM provider template stored in the database and in-memory
type StoredLLMProviderTemplate struct {
	UUID          string                  `json:"uuid"`
	Configuration api.LLMProviderTemplate `json:"configuration"`
	CreatedAt     time.Time               `json:"createdAt"`
	UpdatedAt     time.Time               `json:"updatedAt"`
}

// AnnotationTemplateOrigin marks a template the gateway fetched from the control plane. The gateway sets it itself
// when it saves a fetched template, and refreshes only templates that carry it, so a template it ships or one
// applied through the management API or the operator is never replaced by a control-plane fetch.
const AnnotationTemplateOrigin = "gateway.api-platform.wso2.com/origin"

// GetHandle returns the template handle
func (t *StoredLLMProviderTemplate) GetHandle() string {
	return t.Configuration.Metadata.Name
}

// IsFromControlPlane reports whether the template was fetched from the control plane.
func (t *StoredLLMProviderTemplate) IsFromControlPlane() bool {
	if t == nil || t.Configuration.Metadata.Annotations == nil {
		return false
	}
	return (*t.Configuration.Metadata.Annotations)[AnnotationTemplateOrigin] == string(OriginControlPlane)
}

func (t *StoredLLMProviderTemplate) GetGroupID() string {
	if t.Configuration.Spec.GroupId != nil {
		if v := strings.TrimSpace(*t.Configuration.Spec.GroupId); v != "" {
			return v
		}
	}
	return t.Configuration.Metadata.Name
}

func (t *StoredLLMProviderTemplate) GetVersion() string {
	if t.Configuration.Spec.Version != nil {
		if v := strings.TrimSpace(*t.Configuration.Spec.Version); v != "" {
			return v
		}
	}
	return DefaultTemplateVersion
}

func (t *StoredLLMProviderTemplate) GetManagedBy() string {
	if t.Configuration.Spec.ManagedBy != nil {
		if p := strings.TrimSpace(*t.Configuration.Spec.ManagedBy); p != "" {
			return p
		}
	}
	return DefaultTemplateManagedBy
}
