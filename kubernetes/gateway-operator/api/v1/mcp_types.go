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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MCPUpstreamAuth carries upstream credential configuration for an MCP
// proxy (mirrors the management-API MCPProxyConfigData.upstream.auth shape).
type MCPUpstreamAuth struct {
	// Type identifies the auth scheme. Only api-key is currently supported.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=api-key
	Type string `json:"type"`

	// Header is the HTTP header to set on outbound requests.
	// +optional
	Header *string `json:"header,omitempty"`

	// Value sources the credential plaintext.
	// +kubebuilder:validation:Required
	Value SecretValueSource `json:"value"`
}

// MCPUpstream describes an MCP proxy upstream backend.
type MCPUpstream struct {
	// Url is the direct backend URL.
	// +optional
	Url *string `json:"url,omitempty"`

	// Ref is the name of a predefined upstream definition.
	// +optional
	Ref *string `json:"ref,omitempty"`

	// HostRewrite controls how the Host header is handled.
	// +optional
	// +kubebuilder:validation:Enum=auto;manual
	HostRewrite *string `json:"hostRewrite,omitempty"`

	// Auth configures upstream credentials.
	// +optional
	Auth *MCPUpstreamAuth `json:"auth,omitempty"`
}

// MCPPromptArgument describes one input argument for an MCP prompt.
type MCPPromptArgument struct {
	// Name is the argument name.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Title is an optional human-readable label.
	// +optional
	Title *string `json:"title,omitempty"`

	// Description is an optional argument description.
	// +optional
	Description *string `json:"description,omitempty"`

	// Required marks the argument as mandatory.
	// +optional
	Required *bool `json:"required,omitempty"`
}

// MCPPrompt mirrors the management-API MCPPrompt schema.
type MCPPrompt struct {
	// Name is the unique prompt identifier.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Title is an optional human-readable name.
	// +optional
	Title *string `json:"title,omitempty"`

	// Description is an optional description.
	// +optional
	Description *string `json:"description,omitempty"`

	// Arguments is the optional argument list.
	// +optional
	Arguments []MCPPromptArgument `json:"arguments,omitempty"`
}

// MCPResource mirrors the management-API MCPResource schema.
type MCPResource struct {
	// Uri is a unique identifier for the resource.
	// +kubebuilder:validation:Required
	Uri string `json:"uri"`

	// Name is a human-readable resource name.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Title is an optional human-readable label.
	// +optional
	Title *string `json:"title,omitempty"`

	// Description is an optional description.
	// +optional
	Description *string `json:"description,omitempty"`

	// MimeType is an optional MIME type.
	// +optional
	MimeType *string `json:"mimeType,omitempty"`

	// Size is the optional size in bytes.
	// +optional
	Size *int64 `json:"size,omitempty"`
}

// MCPTool mirrors the management-API MCPTool schema.
type MCPTool struct {
	// Name is the unique tool identifier.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Description is the human-readable functional description.
	// +kubebuilder:validation:Required
	Description string `json:"description"`

	// InputSchema is the JSON Schema for input parameters.
	// +kubebuilder:validation:Required
	InputSchema string `json:"inputSchema"`

	// Title is an optional human-readable label.
	// +optional
	Title *string `json:"title,omitempty"`

	// OutputSchema is the optional JSON Schema for the output.
	// +optional
	OutputSchema *string `json:"outputSchema,omitempty"`
}

// MCPProxyConfigData mirrors the management-API MCPProxyConfigData payload.
type MCPProxyConfigData struct {
	// DisplayName is a human-readable MCP proxy name.
	// +kubebuilder:validation:Required
	DisplayName string `json:"displayName"`

	// Version is the MCP proxy semantic version.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^v?([0-9]+)(\.[0-9]+)?(\.[0-9]+)?(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`
	Version string `json:"version"`

	// UpstreamDefinitions is the list of reusable upstream definitions (with optional
	// connect timeout) that upstream.ref can reference.
	// +optional
	UpstreamDefinitions []UpstreamDefinition `json:"upstreamDefinitions,omitempty"`

	// Upstream is the MCP backend.
	// +kubebuilder:validation:Required
	Upstream MCPUpstream `json:"upstream"`

	// Context is the base path for routes (must start with /).
	// +optional
	// +kubebuilder:validation:Pattern=`^/[a-zA-Z0-9\-._~!$&'()*+,;=:@%/]*[^/]$`
	Context *string `json:"context,omitempty"`

	// Vhost is the virtual host for routing.
	// +optional
	Vhost *string `json:"vhost,omitempty"`

	// SpecVersion is the MCP specification version.
	// +optional
	SpecVersion *string `json:"specVersion,omitempty"`

	// DeploymentState toggles whether the proxy is router-attached.
	// +optional
	// +kubebuilder:validation:Enum=deployed;undeployed
	DeploymentState *string `json:"deploymentState,omitempty"`

	// Prompts lists optional prompts exposed by this MCP proxy.
	// +optional
	Prompts []MCPPrompt `json:"prompts,omitempty"`

	// Resources lists optional MCP resources exposed by this proxy.
	// +optional
	Resources []MCPResource `json:"resources,omitempty"`

	// Tools lists optional tools exposed by this MCP proxy.
	// +optional
	Tools []MCPTool `json:"tools,omitempty"`

	// Policies are MCP proxy-level policies.
	// +optional
	Policies []Policy `json:"policies,omitempty"`

	// Resilience configures API-level backend/route timeouts applied to the traffic-forwarding
	// routes generated for this MCP proxy. Supported at the API level only. Because MCP transports
	// are long-lived streams, the route timeout defaults to disabled ("0s") for MCP when unset.
	// +optional
	Resilience *Resilience `json:"resilience,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:storageversion
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=mcps,singular=mcp,shortName=mcp

// Mcp is the Schema for the mcps API.
type Mcp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the MCP proxy configuration.
	// +kubebuilder:validation:Required
	Spec   MCPProxyConfigData `json:"spec"`
	Status ResourceStatus     `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// McpList contains a list of Mcp.
type McpList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Mcp `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Mcp{}, &McpList{})
}
