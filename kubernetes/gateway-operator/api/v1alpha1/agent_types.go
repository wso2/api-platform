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

// The types below mirror the gateway-controller management-API schemas
// AgentConfigData / A2AConfig and their children (api/management-openapi.yaml).
// The two representations are separately maintained with no compile-time link,
// so a change to either side must be mirrored in the other.
//
// Only structural constraints from the management-API schema are expressed
// here. The semantic A2A rules — mode-specific card requirements, Agent Card
// validation against the vendored A2A protocol model, transport/operation
// uniqueness, security consistency — are enforced by the gateway-controller at
// deploy time and surface on this CR's Programmed condition.

// AgentUpstreamAuth carries upstream credential configuration for an Agent
// (mirrors the management-API AgentConfigData.upstream.auth shape).
type AgentUpstreamAuth struct {
	// Type identifies the auth scheme. Mirrors the management-API
	// UpstreamAuth.auth.type enum; "none" removes upstream auth.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=api-key;other;none
	Type string `json:"type"`

	// Header is the HTTP header to set on outbound requests.
	// +optional
	Header *string `json:"header,omitempty"`

	// Value sources the credential plaintext. Optional because the
	// management-API field is optional and carries no credential for
	// type "none".
	// +optional
	Value *SecretValueSource `json:"value,omitempty"`
}

// AgentUpstream describes the backend A2A agent. The URL is the base the
// gateway forwards A2A operation traffic to, and — in public passthrough card
// mode — the origin of the standard /.well-known/agent-card.json document.
// Exactly one of url or ref must be set.
// +kubebuilder:validation:XValidation:rule="has(self.url) != has(self.ref)",message="exactly one of url or ref must be set"
type AgentUpstream struct {
	// Url is the direct backend URL.
	// +optional
	Url *string `json:"url,omitempty"`

	// Ref is the name of a predefined upstreamDefinition.
	// +optional
	Ref *string `json:"ref,omitempty"`

	// HostRewrite controls how the Host header is handled.
	// +optional
	// +kubebuilder:validation:Enum=auto;manual
	HostRewrite *string `json:"hostRewrite,omitempty"`

	// Auth configures upstream credentials.
	// +optional
	Auth *AgentUpstreamAuth `json:"auth,omitempty"`
}

// A2AProtocolBinding is an A2A protocol binding exposed at a transport's path
// prefix.
// +kubebuilder:validation:Enum=JSONRPC;"HTTP+JSON"
type A2AProtocolBinding string

const (
	// A2AProtocolBindingJSONRPC is the JSON-RPC 2.0 binding.
	A2AProtocolBindingJSONRPC A2AProtocolBinding = "JSONRPC"
	// A2AProtocolBindingHTTPJSON is the HTTP+JSON (REST) binding.
	A2AProtocolBindingHTTPJSON A2AProtocolBinding = "HTTP+JSON"
)

// A2AOperationName is a canonical A2A operation name. These names match the
// standard JSON-RPC and gRPC method names but identify the binding-independent
// A2A operation. The effective set is the one defined by the Agent's
// spec.a2a.protocolVersion; the values below are A2A 1.0's eleven operations,
// that being the only protocol version currently supported.
// +kubebuilder:validation:Enum=SendMessage;SendStreamingMessage;GetTask;ListTasks;CancelTask;SubscribeToTask;CreateTaskPushNotificationConfig;GetTaskPushNotificationConfig;ListTaskPushNotificationConfigs;DeleteTaskPushNotificationConfig;GetExtendedAgentCard
type A2AOperationName string

// A2ATransport is one A2A protocol binding exposed by the gateway and the path
// prefix at which that binding is served relative to spec.context.
type A2ATransport struct {
	// ProtocolBinding is the A2A protocol binding served at PathPrefix.
	// +kubebuilder:validation:Required
	ProtocolBinding A2AProtocolBinding `json:"protocolBinding"`

	// PathPrefix is the gateway-facing path prefix relative to spec.context.
	// The root value / means that no additional path segment is inserted. For
	// JSONRPC this is the endpoint path; for HTTP+JSON, canonical operation
	// paths are appended below it. Defaults to / in the gateway-controller when
	// unset.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=200
	// +kubebuilder:validation:Pattern=`^/(?:[A-Za-z0-9._~!$&()*+,;=:@%-]+(?:/[A-Za-z0-9._~!$&()*+,;=:@%-]+)*)?$`
	PathPrefix *string `json:"pathPrefix,omitempty"`
}

// A2AOperationConfig is the configuration for one standard A2A operation,
// identified by its canonical operation name.
type A2AOperationConfig struct {
	// Name is the canonical A2A operation name.
	// +kubebuilder:validation:Required
	Name A2AOperationName `json:"name"`

	// Policies are applied after spec.a2a.operationConfigs.policies when this
	// operation is selected.
	// +optional
	Policies []Policy `json:"policies,omitempty"`

	// Resilience overrides the Agent-level timeouts for this operation's route.
	// +optional
	Resilience *Resilience `json:"resilience,omitempty"`
}

// A2AOperationConfigs carries transport exposure and common or
// operation-specific configuration for A2A operations. These policies and
// transports do not apply to public Agent Card serving.
type A2AOperationConfigs struct {
	// Transports are the ordered A2A protocol bindings and their gateway-facing
	// path prefixes. This is runtime routing configuration, not Agent Card
	// transformation or transport conversion. MaxItems mirrors the
	// management-API bound: there are two bindings and a binding may appear at
	// most once.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=2
	Transports []A2ATransport `json:"transports"`

	// Policies are applied to every A2A operation, before operation-level
	// policies. They do not apply to the public Agent Card discovery route.
	// +optional
	Policies []Policy `json:"policies,omitempty"`

	// Operations is optional per-operation configuration keyed by canonical A2A
	// operation name. It is not an allowlist: unlisted standard operations
	// still receive spec.a2a.operationConfigs.policies. MaxItems mirrors the
	// management-API bound implied by a closed eleven-value operation set whose
	// entries must be unique; it grows with any future protocol version that
	// defines more operations.
	// +optional
	// +kubebuilder:validation:MaxItems=11
	Operations []A2AOperationConfig `json:"operations,omitempty"`
}

// A2ACardSigning is the optional signing configuration for a managed Agent
// Card. Passthrough cards cannot configure gateway signing. Agent authors only
// enable or disable signing: the active key, its key identifier, and the JWS
// algorithm are selected from administrator-owned gateway system configuration
// at signing time.
type A2ACardSigning struct {
	// Enabled selects whether the gateway signs the managed card it serves,
	// using the active Agent Card signing key configured by the gateway
	// administrator.
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`
}

// A2APublicAgentCard configures public Agent Card serving. Mode selects whether
// the card is proxied unchanged from the upstream (passthrough) or validated,
// stored, and served by the gateway (managed). Mode-specific rules are enforced
// by the gateway-controller at deploy time, not by this schema: managed
// requires content; passthrough accepts neither content nor signing.
type A2APublicAgentCard struct {
	// Mode selects how the public Agent Card is produced.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=managed;passthrough
	Mode string `json:"mode"`

	// Path is the exact gateway-facing Agent Card path relative to
	// spec.context. When omitted the gateway-controller uses
	// /.well-known/agent-card.json. A custom path replaces that default route
	// rather than creating an additional alias. In passthrough mode this does
	// not change the upstream discovery path.
	// +optional
	// +kubebuilder:validation:MinLength=2
	// +kubebuilder:validation:MaxLength=200
	// +kubebuilder:validation:Pattern=`^/[A-Za-z0-9._~!$&()*+,;=:@%-]+(?:/[A-Za-z0-9._~!$&()*+,;=:@%-]+)*$`
	Path *string `json:"path,omitempty"`

	// Policies are applied only to public Agent Card serving.
	// +optional
	Policies []Policy `json:"policies,omitempty"`

	// Content is the complete Agent Card as a structured object, preserved as
	// supplied — the gateway never rewrites it, so extension fields survive.
	// The gateway-controller validates it against the Agent Card model for
	// spec.a2a.protocolVersion.
	// +optional
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	Content *runtime.RawExtension `json:"content,omitempty"`

	// Signing configures gateway signing of a managed card.
	// +optional
	Signing *A2ACardSigning `json:"signing,omitempty"`
}

// A2AProtectedAgentCard configures the authenticated extended Agent Card. It is
// served through the canonical GetExtendedAgentCard operation and uses that
// operation's policy chain, not public Agent Card policies, so it has no custom
// path and no local policy list: it is an A2A operation rather than a document
// at a location.
//
// When this block is present the gateway requires the request to have been
// authenticated by a policy in the Agent's own chain before the card is returned
// or proxied, in either mode. Leaving it out is not the same as configuring
// passthrough — an Agent without it keeps the behaviour it shipped with, where
// GetExtendedAgentCard is proxied to the upstream with no gateway-added guard.
//
// Mode-specific rules are enforced by the gateway-controller at deploy time, not
// by this schema: managed requires content; passthrough accepts neither content
// nor signing. A managed public card must also declare
// capabilities.extendedAgentCard: true.
type A2AProtectedAgentCard struct {
	// Mode selects how the protected Agent Card is produced. managed serves the
	// supplied content from the gateway and never reaches the upstream;
	// passthrough forwards the authenticated request and proxies the upstream's
	// own response unchanged.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=managed;passthrough
	Mode string `json:"mode"`

	// Content is the complete Agent Card as a structured object, preserved as
	// supplied. Required in managed mode and rejected in passthrough mode.
	// +optional
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	Content *runtime.RawExtension `json:"content,omitempty"`

	// Signing configures gateway signing of a managed card. The two card
	// representations are signed independently.
	// +optional
	Signing *A2ACardSigning `json:"signing,omitempty"`
}

// A2AAgentCard is the public Agent Card configuration and the optional
// protected Agent Card configuration for the authenticated A2A
// GetExtendedAgentCard operation.
type A2AAgentCard struct {
	// Public configures public Agent Card serving.
	// +kubebuilder:validation:Required
	Public A2APublicAgentCard `json:"public"`

	// Protected configures the authenticated extended Agent Card. Omitting it
	// leaves GetExtendedAgentCard proxied to the upstream unguarded, which is not
	// the same as configuring passthrough.
	// +optional
	Protected *A2AProtectedAgentCard `json:"protected,omitempty"`
}

// A2AConfig is the A2A-specific agent configuration.
type A2AConfig struct {
	// ProtocolVersion is the A2A protocol version exposed by the gateway. It
	// selects the Agent's operation set and HTTP+JSON bindings, the Agent Card
	// model its managed card is validated against, and the field-presence rules
	// used to sign that card. An Agent exposes exactly one version and the
	// gateway performs no protocol-version conversion.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum="1.0"
	ProtocolVersion string `json:"protocolVersion"`

	// OperationConfigs carries transport exposure and operation policies.
	// +kubebuilder:validation:Required
	OperationConfigs A2AOperationConfigs `json:"operationConfigs"`

	// AgentCard configures Agent Card serving.
	// +kubebuilder:validation:Required
	AgentCard A2AAgentCard `json:"agentCard"`
}

// AgentConfigData mirrors the management-API AgentConfigData payload.
type AgentConfigData struct {
	// DisplayName is a human-readable agent name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=100
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9\-_\. ]+$`
	DisplayName string `json:"displayName"`

	// Version is the agent semantic version.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^v\d+\.\d+$`
	Version string `json:"version"`

	// Context is the gateway context path for the agent (must start with /, no
	// trailing slash). Optional: when omitted the agent is served at the root of
	// its virtual host, which is where an A2A client probes for
	// /.well-known/agent-card.json during cold discovery. Every A2A route the
	// gateway generates is relative to this value.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=200
	// +kubebuilder:validation:Pattern=`^\/([a-zA-Z0-9_\-\/]*[^\/])?$`
	Context *string `json:"context,omitempty"`

	// Vhost is the virtual host used for routing. Wildcards are only allowed in
	// the left-most label (e.g. *.example.com).
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^(\*\.|[a-zA-Z0-9](?:[a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`
	Vhost *string `json:"vhost,omitempty"`

	// UpstreamDefinitions is the list of reusable upstream definitions (with
	// optional connect timeout) that upstream.ref can reference.
	// +optional
	UpstreamDefinitions []UpstreamDefinition `json:"upstreamDefinitions,omitempty"`

	// Upstream is the backend A2A agent.
	// +kubebuilder:validation:Required
	Upstream AgentUpstream `json:"upstream"`

	// DeploymentState toggles whether the agent is router-attached.
	// +optional
	// +kubebuilder:validation:Enum=deployed;undeployed
	DeploymentState *string `json:"deploymentState,omitempty"`

	// Resilience configures Agent-level backend/route timeouts applied to the
	// traffic-forwarding routes generated for this Agent. Because A2A streaming
	// operations are long-lived streams, the route timeout defaults to disabled
	// ("0s") for the JSON-RPC route and for streaming HTTP+JSON routes unless a
	// timeout is set here (unlike RestApi/LLM, which fall back to the gateway's
	// global route timeout); the idle timeout remains the liveness guard.
	// +optional
	Resilience *Resilience `json:"resilience,omitempty"`

	// A2A is the A2A-specific agent configuration.
	// +kubebuilder:validation:Required
	A2A A2AConfig `json:"a2a"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=agents,singular=agent

// Agent is the Schema for the agents API.
type Agent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the Agent configuration.
	// +kubebuilder:validation:Required
	Spec   AgentConfigData `json:"spec"`
	Status ResourceStatus  `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AgentList contains a list of Agent.
type AgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Agent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Agent{}, &AgentList{})
}
