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

// LLMProxyProvider references a deployed LlmProvider that this proxy fronts.
type LLMProxyProvider struct {
	// Id is the LlmProvider handle (metadata.name).
	// +kubebuilder:validation:Required
	Id string `json:"id"`

	// Auth optionally overrides upstream credentials when calling the
	// referenced LLM provider.
	// +optional
	Auth *LLMUpstreamAuth `json:"auth,omitempty"`
}

// LLMProxyAdditionalProvider references an additional LlmProvider that this
// proxy can route to by policy-selected upstream name.
type LLMProxyAdditionalProvider struct {
	// Id is the LlmProvider handle (metadata.name).
	// +kubebuilder:validation:Required
	Id string `json:"id"`

	// As is the logical upstream name used by policies. Defaults to Id.
	// +optional
	As *string `json:"as,omitempty"`

	// Auth optionally configures credentials for proxy-to-provider loopback
	// calls when the referenced provider is protected by an auth policy.
	// +optional
	Auth *LLMUpstreamAuth `json:"auth,omitempty"`
}

// LLMProxyConfigData mirrors the management-API LLMProxyConfigData payload.
type LLMProxyConfigData struct {
	// DisplayName is a human-readable LLM proxy name.
	// +kubebuilder:validation:Required
	DisplayName string `json:"displayName"`

	// Version is the semantic version of the LLM proxy.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^v?([0-9]+)(\.[0-9]+)?(\.[0-9]+)?(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`
	Version string `json:"version"`

	// Provider references the deployed LLM provider this proxy fronts.
	// +kubebuilder:validation:Required
	Provider LLMProxyProvider `json:"provider"`

	// AdditionalProviders are extra LLM providers attached as selectable
	// upstreams for multi-provider routing.
	// +optional
	AdditionalProviders []LLMProxyAdditionalProvider `json:"additionalProviders,omitempty"`

	// Context is the base path for routes (must start with /).
	// +optional
	// +kubebuilder:validation:Pattern=`^/[a-zA-Z0-9\-._~!$&'()*+,;=:@%/]*[^/]$`
	Context *string `json:"context,omitempty"`

	// Vhost is the virtual host for routing.
	// +optional
	Vhost *string `json:"vhost,omitempty"`

	// DeploymentState toggles whether the proxy is router-attached.
	// +optional
	// +kubebuilder:validation:Enum=deployed;undeployed
	DeploymentState *string `json:"deploymentState,omitempty"`

	// Policies is the list of policies applied to this LLM proxy.
	// +optional
	Policies []LLMPolicy `json:"policies,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=llmproxies,singular=llmproxy,shortName=llmpx

// LlmProxy is the Schema for the llmproxies API.
type LlmProxy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the LLM proxy configuration payload.
	// +kubebuilder:validation:Required
	Spec   LLMProxyConfigData `json:"spec"`
	Status ResourceStatus     `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LlmProxyList contains a list of LlmProxy.
type LlmProxyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LlmProxy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LlmProxy{}, &LlmProxyList{})
}
