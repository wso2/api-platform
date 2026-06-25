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
	"k8s.io/apimachinery/pkg/runtime"
)

// HTTPMethod names an allowed HTTP verb for LLM access-control and policy paths.
// +kubebuilder:validation:Enum=GET;POST;PUT;PATCH;DELETE;HEAD;OPTIONS;*
type HTTPMethod string

// RouteException defines a path/method exception used by LLM access control.
type RouteException struct {
	// Path is the exception path pattern.
	// +kubebuilder:validation:Required
	Path string `json:"path"`

	// Methods is the list of HTTP methods covered by this exception.
	// +kubebuilder:validation:Required
	Methods []HTTPMethod `json:"methods"`
}

// LLMAccessControl configures path-level access control for an LLM provider.
type LLMAccessControl struct {
	// Mode selects the default access policy.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=allow_all;deny_all
	Mode string `json:"mode"`

	// Exceptions are paths that override the default mode.
	// +optional
	Exceptions []RouteException `json:"exceptions,omitempty"`
}

// LLMUpstreamAuth carries upstream credential information for an LLM
// provider. The plaintext credential may either be inlined via Value or
// loaded from a Kubernetes Secret via ValueFrom.
type LLMUpstreamAuth struct {
	// Type identifies the auth scheme. Only api-key is currently supported.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=api-key
	Type string `json:"type"`

	// Header is the HTTP header to set on outbound requests when the auth
	// type uses headers (e.g. api-key).
	// +optional
	Header *string `json:"header,omitempty"`

	// Value sources the credential. Exactly one of value or valueFrom must
	// be set.
	// +kubebuilder:validation:Required
	Value SecretValueSource `json:"value"`
}

// LLMProviderUpstream describes the upstream backend for an LLM provider.
type LLMProviderUpstream struct {
	// Url is the direct backend URL.
	// +optional
	Url *string `json:"url,omitempty"`

	// Ref selects a predefined upstream definition by name.
	// +optional
	Ref *string `json:"ref,omitempty"`

	// HostRewrite controls how the Host header is handled when routing.
	// +optional
	// +kubebuilder:validation:Enum=auto;manual
	HostRewrite *string `json:"hostRewrite,omitempty"`

	// Auth configures upstream credentials.
	// +optional
	Auth *LLMUpstreamAuth `json:"auth,omitempty"`
}

// LLMProviderConfigData mirrors the management-API LLMProviderConfigData.
type LLMProviderConfigData struct {
	// DisplayName is a human-readable LLM provider name.
	// +kubebuilder:validation:Required
	DisplayName string `json:"displayName"`

	// Version is the semantic version of the LLM provider.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^v?([0-9]+)(\.[0-9]+)?(\.[0-9]+)?(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`
	Version string `json:"version"`

	// Template is the LlmProviderTemplate name to apply.
	// +kubebuilder:validation:Required
	Template string `json:"template"`

	// AccessControl configures path-level access control.
	// +kubebuilder:validation:Required
	AccessControl LLMAccessControl `json:"accessControl"`

	// Upstream configures the LLM upstream.
	// +kubebuilder:validation:Required
	Upstream LLMProviderUpstream `json:"upstream"`

	// Context is the base path for all routes (must start with /).
	// +optional
	// +kubebuilder:validation:Pattern=`^/[a-zA-Z0-9\-._~!$&'()*+,;=:@%/]*[^/]$`
	Context *string `json:"context,omitempty"`

	// Vhost is the virtual host for routing.
	// +optional
	Vhost *string `json:"vhost,omitempty"`

	// DeploymentState toggles whether the provider is router-attached.
	// +optional
	// +kubebuilder:validation:Enum=deployed;undeployed
	DeploymentState *string `json:"deploymentState,omitempty"`

	// Policies is the list of policies applied to this LLM provider.
	// +optional
	Policies []LLMPolicy `json:"policies,omitempty"`

	// Resilience configures API-level backend/route timeouts applied to all routes
	// generated for this LLM provider. Supported at the API level only.
	// +optional
	Resilience *Resilience `json:"resilience,omitempty"`
}

// LLMPolicyPath defines a path/methods combination together with policy
// parameters; mirrors the management-API LLMPolicyPath.
type LLMPolicyPath struct {
	// Path is the route path pattern.
	// +kubebuilder:validation:Required
	Path string `json:"path"`

	// Methods is the list of HTTP methods.
	// +kubebuilder:validation:Required
	Methods []HTTPMethod `json:"methods"`

	// Params is a free-form parameter object specific to the policy.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Params *runtime.RawExtension `json:"params,omitempty"`
}

// LLMPolicy describes a single LLM-level policy attachment.
type LLMPolicy struct {
	// Name is the name of the policy.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Version is the policy version (e.g. v1).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^v\d+$`
	Version string `json:"version"`

	// Paths lists path/method/params triples for this policy.
	// +kubebuilder:validation:Required
	Paths []LLMPolicyPath `json:"paths"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=llmproviders,singular=llmprovider,shortName=llmp

// LlmProvider is the Schema for the llmproviders API.
type LlmProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LLMProviderConfigData `json:"spec,omitempty"`
	Status ResourceStatus        `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LlmProviderList contains a list of LlmProvider.
type LlmProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LlmProvider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LlmProvider{}, &LlmProviderList{})
}
