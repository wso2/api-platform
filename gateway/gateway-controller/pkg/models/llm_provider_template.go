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
const DefaultTemplateManagedBy = "customer"

// StoredLLMProviderTemplate represents the LLM provider template stored in the database and in-memory
type StoredLLMProviderTemplate struct {
	UUID          string                  `json:"uuid"`
	Configuration api.LLMProviderTemplate `json:"configuration"`
	CreatedAt     time.Time               `json:"createdAt"`
	UpdatedAt     time.Time               `json:"updatedAt"`
}

// GetGroupVersionID returns the template's stable family-grouping identifier
// (the version-independent identifier shared by every version of this
// template, e.g. "openai"). Falls back to metadata.name when the explicit
// spec.groupVersionId field is unset, for templates created before that
// field existed.
func (t *StoredLLMProviderTemplate) GetGroupVersionID() string {
	if t.Configuration.Spec.GroupVersionId != nil {
		if v := strings.TrimSpace(*t.Configuration.Spec.GroupVersionId); v != "" {
			return v
		}
	}
	return t.Configuration.Metadata.Name
}

func (t *StoredLLMProviderTemplate) GetID() string {
	return MakeTemplateID(t.GetGroupVersionID(), t.GetVersion())
}

func MakeTemplateID(groupVersionID, version string) string {
	h := strings.TrimSpace(groupVersionID)
	v := strings.ToLower(strings.TrimSpace(version))
	if v == "" {
		v = strings.ToLower(DefaultTemplateVersion)
	}
	return h + "-" + strings.ReplaceAll(v, ".", "-")
}

// GetVersion returns the template content version, defaulting to v1.0 when unset.
func (t *StoredLLMProviderTemplate) GetVersion() string {
	if t.Configuration.Spec.Version != nil {
		if v := strings.TrimSpace(*t.Configuration.Spec.Version); v != "" {
			return v
		}
	}
	return DefaultTemplateVersion
}

// GetManagedBy returns the template's managedBy origin, defaulting to "customer" when unset.
func (t *StoredLLMProviderTemplate) GetManagedBy() string {
	if t.Configuration.Spec.ManagedBy != nil {
		if p := strings.TrimSpace(*t.Configuration.Spec.ManagedBy); p != "" {
			return p
		}
	}
	return DefaultTemplateManagedBy
}
