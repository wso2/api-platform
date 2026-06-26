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

// Code generated from management-openapi.yaml WebSub/WebBroker schemas.
// These types are defined here (not in generated.go) because oapi-codegen
// only emits types reachable from operations, and the WebSub/WebBroker
// path operations have moved to event-gateway-controller.
package management

import "time"

// Defines values for WebBrokerApiApiVersion.
const (
	WebBrokerApiApiVersionGatewayApiPlatformWso2Comv1 WebBrokerApiApiVersion = "gateway.api-platform.wso2.com/v1"
)

// Defines values for WebBrokerApiKind.
const (
	WebBrokerApiKindWebBrokerApi WebBrokerApiKind = "WebBrokerApi"
)

// Defines values for WebSubAPIApiVersion.
const (
	WebSubAPIApiVersionGatewayApiPlatformWso2Comv1 WebSubAPIApiVersion = "gateway.api-platform.wso2.com/v1"
)

// Defines values for WebSubAPIKind.
const (
	WebSubAPIKindWebSubApi WebSubAPIKind = "WebSubApi"
)
type WebBrokerApi struct {
	// ApiVersion API specification version
	ApiVersion WebBrokerApiApiVersion `json:"apiVersion" yaml:"apiVersion"`

	// Kind API type
	Kind     WebBrokerApiKind `json:"kind" yaml:"kind"`
	Metadata Metadata         `json:"metadata" yaml:"metadata"`
	Spec     WebBrokerApiData `json:"spec" yaml:"spec"`

	// Status Server-managed lifecycle fields. Populated on responses.
	Status *ResourceStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

// WebBrokerApiApiVersion API specification version
type WebBrokerApiApiVersion string

// WebBrokerApiKind API type
type WebBrokerApiKind string

// WebBrokerApiAllChannelPolicies Protocol mediation policies applied to all channels
type WebBrokerApiAllChannelPolicies struct {
	// OnConnectionInit Group of policies
	OnConnectionInit *WebBrokerApiPolicyGroup `json:"on_connection_init,omitempty" yaml:"on_connection_init,omitempty"`

	// OnConsume Group of policies
	OnConsume *WebBrokerApiPolicyGroup `json:"on_consume,omitempty" yaml:"on_consume,omitempty"`

	// OnProduce Group of policies
	OnProduce *WebBrokerApiPolicyGroup `json:"on_produce,omitempty" yaml:"on_produce,omitempty"`
}

// WebBrokerApiBroker Message broker driver configuration
type WebBrokerApiBroker struct {
	// Name Broker driver name
	Name string `json:"name" yaml:"name"`

	// Properties Broker driver properties (e.g., bootstrap servers)
	Properties map[string]interface{} `json:"properties" yaml:"properties"`

	// Type Broker driver type
	Type string `json:"type" yaml:"type"`
}

// WebBrokerApiChannel WebSocket channel configuration with Kafka topic mapping
type WebBrokerApiChannel struct {
	// ConsumeFrom Configuration for consuming messages from Kafka to WebSocket
	ConsumeFrom *WebBrokerApiConsumeConfig `json:"consumeFrom,omitempty" yaml:"consumeFrom,omitempty"`

	// OnConnectionInit Group of policies
	OnConnectionInit *WebBrokerApiPolicyGroup `json:"on_connection_init,omitempty" yaml:"on_connection_init,omitempty"`

	// OnConsume Group of policies
	OnConsume *WebBrokerApiPolicyGroup `json:"on_consume,omitempty" yaml:"on_consume,omitempty"`

	// OnProduce Group of policies
	OnProduce *WebBrokerApiPolicyGroup `json:"on_produce,omitempty" yaml:"on_produce,omitempty"`

	// ProduceTo Configuration for producing messages from WebSocket to Kafka
	ProduceTo *WebBrokerApiProduceConfig `json:"produceTo,omitempty" yaml:"produceTo,omitempty"`
}

// WebBrokerApiConsumeConfig Configuration for consuming messages from Kafka to WebSocket
type WebBrokerApiConsumeConfig struct {
	// Topic Kafka topic to consume messages from
	Topic string `json:"topic" yaml:"topic"`
}

// WebBrokerApiData defines model for WebBrokerApiData.
type WebBrokerApiData struct {
	// AllChannels Protocol mediation policies applied to all channels
	AllChannels *WebBrokerApiAllChannelPolicies `json:"allChannels,omitempty" yaml:"allChannels,omitempty"`

	// Broker Message broker driver configuration
	Broker WebBrokerApiBroker `json:"broker" yaml:"broker"`

	// Channels Map of WebSocket channels for bidirectional streaming with Kafka (key is channel name)
	Channels map[string]WebBrokerApiChannel `json:"channels" yaml:"channels"`

	// Context Base path for all API routes (must start with /, no trailing slash)
	Context string `json:"context" yaml:"context"`

	// DeploymentState Desired deployment state - 'deployed' (default) or 'undeployed'. When set to 'undeployed', the API is removed from router traffic but configuration and policies are preserved for potential redeployment.
	DeploymentState *WebBrokerApiDataDeploymentState `json:"deploymentState,omitempty" yaml:"deploymentState,omitempty"`

	// DisplayName Human-readable API name (must be URL-friendly - only letters, numbers, spaces, hyphens, underscores, and dots allowed)
	DisplayName string `json:"displayName" yaml:"displayName"`

	// Receiver WebSocket receiver configuration
	Receiver WebBrokerApiReceiver `json:"receiver" yaml:"receiver"`

	// Version Semantic version of the API
	Version string `json:"version" yaml:"version"`

	// Vhosts Custom virtual hosts/domains for the API
	Vhosts *struct {
		// Main Custom virtual host/domain for production traffic
		Main string `json:"main" yaml:"main"`

		// Sandbox Custom virtual host/domain for sandbox traffic
		Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
	} `json:"vhosts,omitempty" yaml:"vhosts,omitempty"`
}

// WebBrokerApiDataDeploymentState Desired deployment state - 'deployed' (default) or 'undeployed'. When set to 'undeployed', the API is removed from router traffic but configuration and policies are preserved for potential redeployment.
type WebBrokerApiDataDeploymentState string

// WebBrokerApiPolicyGroup Group of policies
type WebBrokerApiPolicyGroup struct {
	// Policies List of policies to apply
	Policies *[]Policy `json:"policies,omitempty" yaml:"policies,omitempty"`
}

// WebBrokerApiProduceConfig Configuration for producing messages from WebSocket to Kafka
type WebBrokerApiProduceConfig struct {
	// Topic Kafka topic to produce messages to
	Topic string `json:"topic" yaml:"topic"`
}

// WebBrokerApiReceiver WebSocket receiver configuration
type WebBrokerApiReceiver struct {
	// Name Receiver name
	Name string `json:"name" yaml:"name"`

	// Properties Additional receiver properties
	Properties *map[string]interface{} `json:"properties,omitempty" yaml:"properties,omitempty"`

	// Type Receiver type
	Type string `json:"type" yaml:"type"`
}

// WebBrokerApiRequest defines model for WebBrokerApiRequest.
type WebBrokerApiRequest struct {
	// ApiVersion API specification version
	ApiVersion WebBrokerApiRequestApiVersion `json:"apiVersion" yaml:"apiVersion"`

	// Kind API type
	Kind     WebBrokerApiRequestKind `json:"kind" yaml:"kind"`
	Metadata Metadata                `json:"metadata" yaml:"metadata"`
	Spec     WebBrokerApiData        `json:"spec" yaml:"spec"`
}

// WebBrokerApiRequestApiVersion API specification version
type WebBrokerApiRequestApiVersion string

// WebBrokerApiRequestKind API type
type WebBrokerApiRequestKind string

// WebSubAPI defines model for WebSubAPI.
type WebSubAPI struct {
	// ApiVersion API specification version
	ApiVersion WebSubAPIApiVersion `json:"apiVersion" yaml:"apiVersion"`

	// Kind API type
	Kind     WebSubAPIKind  `json:"kind" yaml:"kind"`
	Metadata Metadata       `json:"metadata" yaml:"metadata"`
	Spec     WebhookAPIData `json:"spec" yaml:"spec"`

	// Status Server-managed lifecycle fields. Populated on responses.
	Status *ResourceStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

// WebSubAPIApiVersion API specification version
type WebSubAPIApiVersion string

// WebSubAPIKind API type
type WebSubAPIKind string

// WebSubAPIRequest defines model for WebSubAPIRequest.
type WebSubAPIRequest struct {
	// ApiVersion API specification version
	ApiVersion WebSubAPIRequestApiVersion `json:"apiVersion" yaml:"apiVersion"`

	// Kind API type
	Kind     WebSubAPIRequestKind `json:"kind" yaml:"kind"`
	Metadata Metadata             `json:"metadata" yaml:"metadata"`
	Spec     WebhookAPIData       `json:"spec" yaml:"spec"`
}

// WebSubAPIRequestApiVersion API specification version
type WebSubAPIRequestApiVersion string

// WebSubAPIRequestKind API type
type WebSubAPIRequestKind string

// WebSubAllChannelPolicies Policies applied to all channels, organized by event type.
type WebSubAllChannelPolicies struct {
	// OnMessageDelivery Policies for a single event type.
	OnMessageDelivery *WebSubEventPolicies `json:"on_message_delivery,omitempty" yaml:"on_message_delivery,omitempty"`

	// OnMessageReceived Policies for a single event type.
	OnMessageReceived *WebSubEventPolicies `json:"on_message_received,omitempty" yaml:"on_message_received,omitempty"`

	// OnSubscription Policies for a single event type.
	OnSubscription *WebSubEventPolicies `json:"on_subscription,omitempty" yaml:"on_subscription,omitempty"`

	// OnUnsubscription Policies for a single event type.
	OnUnsubscription *WebSubEventPolicies `json:"on_unsubscription,omitempty" yaml:"on_unsubscription,omitempty"`
}

// WebSubChannel A single channel definition with optional per-channel policy overrides.
type WebSubChannel struct {
	// OnMessageDelivery Policies for a single event type.
	OnMessageDelivery *WebSubEventPolicies `json:"on_message_delivery,omitempty" yaml:"on_message_delivery,omitempty"`

	// OnMessageReceived Policies for a single event type.
	OnMessageReceived *WebSubEventPolicies `json:"on_message_received,omitempty" yaml:"on_message_received,omitempty"`

	// OnSubscription Policies for a single event type.
	OnSubscription *WebSubEventPolicies `json:"on_subscription,omitempty" yaml:"on_subscription,omitempty"`

	// OnUnsubscription Policies for a single event type.
	OnUnsubscription *WebSubEventPolicies `json:"on_unsubscription,omitempty" yaml:"on_unsubscription,omitempty"`
}

// WebSubEventPolicies Policies for a single event type.
type WebSubEventPolicies struct {
	// Policies List of policies applied for this event type.
	Policies *[]Policy `json:"policies,omitempty" yaml:"policies,omitempty"`
}

// WebhookAPIData defines model for WebhookAPIData.
type WebhookAPIData struct {
	// AllChannels Policies applied to all channels, organized by event type.
	AllChannels *WebSubAllChannelPolicies `json:"allChannels,omitempty" yaml:"allChannels,omitempty"`

	// Channels Per-channel configuration keyed by channel name. Each key is a channel name and defines policies applied only to that channel.
	Channels *map[string]WebSubChannel `json:"channels,omitempty" yaml:"channels,omitempty"`

	// Context Base path for all API routes (must start with /, no trailing slash)
	Context string `json:"context" yaml:"context"`

	// DeploymentState Desired deployment state - 'deployed' (default) or 'undeployed'. When set to 'undeployed', the API is removed from router traffic but configuration, API keys, and policies are preserved for potential redeployment.
	DeploymentState *WebhookAPIDataDeploymentState `json:"deploymentState,omitempty" yaml:"deploymentState,omitempty"`

	// DisplayName Human-readable API name (must be URL-friendly - only letters, numbers, spaces, hyphens, underscores, and dots allowed)
	DisplayName string `json:"displayName" yaml:"displayName"`

	// Version Semantic version of the API
	Version string `json:"version" yaml:"version"`

	// Vhosts Custom virtual hosts/domains for the API
	Vhosts *struct {
		// Main Custom virtual host/domain for production traffic
		Main string `json:"main" yaml:"main"`

		// Sandbox Custom virtual host/domain for sandbox traffic
		Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
	} `json:"vhosts,omitempty" yaml:"vhosts,omitempty"`
}

// WebhookAPIDataDeploymentState Desired deployment state - 'deployed' (default) or 'undeployed'. When set to 'undeployed', the API is removed from router traffic but configuration, API keys, and policies are preserved for potential redeployment.
type WebhookAPIDataDeploymentState string

// WebhookSecretCreationRequest defines model for WebhookSecretCreationRequest.
type WebhookSecretCreationRequest struct {
	// DisplayName Human-readable label for this secret (used to derive the immutable name slug).
	DisplayName string `json:"displayName" yaml:"displayName"`
}

// WebhookSecretCreationResponse defines model for WebhookSecretCreationResponse.
type WebhookSecretCreationResponse struct {
	Message string `json:"message" yaml:"message"`

	// Secret The generated plaintext secret value (whsec_ prefix + 64 hex chars).
	// Returned exactly once — store it immediately as it will not be retrievable again.
	Secret string `json:"secret" yaml:"secret"`
	Status string `json:"status" yaml:"status"`

	// WebhookSecret Metadata for an HMAC secret. The plaintext value is never included.
	WebhookSecret *WebhookSecretInfo `json:"webhookSecret,omitempty" yaml:"webhookSecret,omitempty"`
}

// WebhookSecretInfo Metadata for an HMAC secret. The plaintext value is never included.
type WebhookSecretInfo struct {
	CreatedAt *time.Time `json:"createdAt,omitempty" yaml:"createdAt,omitempty"`

	// DisplayName Human-readable label.
	DisplayName *string `json:"displayName,omitempty" yaml:"displayName,omitempty"`

	// Name URL-safe slug (immutable, used as path parameter for regenerate/delete).
	Name      *string                  `json:"name,omitempty" yaml:"name,omitempty"`
	Status    *WebhookSecretInfoStatus `json:"status,omitempty" yaml:"status,omitempty"`
	UpdatedAt *time.Time               `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
}

// WebhookSecretInfoStatus defines model for WebhookSecretInfo.Status.
type WebhookSecretInfoStatus string

// WebhookSecretListResponse defines model for WebhookSecretListResponse.
type WebhookSecretListResponse struct {
	Secrets *[]WebhookSecretInfo `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Status  *string              `json:"status,omitempty" yaml:"status,omitempty"`

	// TotalCount Total number of active secrets for this API
	TotalCount *int `json:"totalCount,omitempty" yaml:"totalCount,omitempty"`
}
