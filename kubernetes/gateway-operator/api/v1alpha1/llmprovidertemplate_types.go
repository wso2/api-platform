/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ExtractionIdentifier locates a value within an HTTP request or response.
// Mirrors the gateway-controller management API ExtractionIdentifier model.
type ExtractionIdentifier struct {
	// Identifier is a JSONPath expression or header name.
	// +kubebuilder:validation:Required
	Identifier string `json:"identifier"`

	// Location is where to look for the value.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=header;pathParam;payload;queryParam
	Location string `json:"location"`
}

// LLMProviderTemplateResourceMapping maps a resource path pattern to per-field
// extraction identifiers for a given LLM API resource.
type LLMProviderTemplateResourceMapping struct {
	// Resource is the resource path pattern this mapping applies to.
	// +kubebuilder:validation:Required
	Resource string `json:"resource"`

	// +optional
	CompletionTokens *ExtractionIdentifier `json:"completionTokens,omitempty"`
	// +optional
	PromptTokens *ExtractionIdentifier `json:"promptTokens,omitempty"`
	// +optional
	RemainingTokens *ExtractionIdentifier `json:"remainingTokens,omitempty"`
	// +optional
	RequestModel *ExtractionIdentifier `json:"requestModel,omitempty"`
	// +optional
	ResponseModel *ExtractionIdentifier `json:"responseModel,omitempty"`
	// +optional
	TotalTokens *ExtractionIdentifier `json:"totalTokens,omitempty"`
}

// LLMProviderTemplateResourceMappings lists per-resource extraction mappings.
type LLMProviderTemplateResourceMappings struct {
	// +optional
	Resources []LLMProviderTemplateResourceMapping `json:"resources,omitempty"`
}

// LLMProviderTemplateData mirrors the management-API LLMProviderTemplateData
// payload. The non-resource extractor fields apply when no resourceMappings
// entry matches the request path.
type LLMProviderTemplateData struct {
	// DisplayName is a human-readable LLM template name.
	// +kubebuilder:validation:Required
	DisplayName string `json:"displayName"`

	// +optional
	CompletionTokens *ExtractionIdentifier `json:"completionTokens,omitempty"`
	// +optional
	PromptTokens *ExtractionIdentifier `json:"promptTokens,omitempty"`
	// +optional
	RemainingTokens *ExtractionIdentifier `json:"remainingTokens,omitempty"`
	// +optional
	RequestModel *ExtractionIdentifier `json:"requestModel,omitempty"`
	// +optional
	ResourceMappings *LLMProviderTemplateResourceMappings `json:"resourceMappings,omitempty"`
	// +optional
	ResponseModel *ExtractionIdentifier `json:"responseModel,omitempty"`
	// +optional
	TotalTokens *ExtractionIdentifier `json:"totalTokens,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=llmprovidertemplates,singular=llmprovidertemplate,shortName=llmpt

// LlmProviderTemplate is the Schema for the llmprovidertemplates API.
type LlmProviderTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LLMProviderTemplateData `json:"spec,omitempty"`
	Status ResourceStatus          `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LlmProviderTemplateList contains a list of LlmProviderTemplate.
type LlmProviderTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LlmProviderTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LlmProviderTemplate{}, &LlmProviderTemplateList{})
}
