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

package management

// WebBrokerApi defines model for WebBrokerApi.
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

// WebBrokerApiData defines spec for WebBrokerApi.
type WebBrokerApiData struct {
	// Context Base path for all API routes (must start with /, no trailing slash)
	Context string `json:"context" yaml:"context"`

	// DeploymentState Desired deployment state - 'deployed' (default) or 'undeployed'. When set to 'undeployed', the API is removed from router traffic but configuration, API keys, and policies are preserved for potential redeployment.
	DeploymentState *WebBrokerApiDataDeploymentState `json:"deploymentState,omitempty" yaml:"deploymentState,omitempty"`

	// DisplayName Human-readable API name (must be URL-friendly - only letters, numbers, spaces, hyphens, underscores, and dots allowed)
	DisplayName string `json:"displayName" yaml:"displayName"`

	// Receiver Receiver configuration - protocol adapter for web-friendly clients (WebSocket, SSE)
	Receiver WebBrokerApiReceiver `json:"receiver" yaml:"receiver"`

	// BrokerDriver Broker driver configuration - message broker adapter (Kafka, MQTT, AMQP)
	BrokerDriver WebBrokerApiBrokerDriver `json:"brokerDriver" yaml:"brokerDriver"`

	// AllChannelPolicies Protocol mediation policies applied to all channels
	AllChannelPolicies *WebBrokerApiPolicies `json:"allChannelPolicies,omitempty" yaml:"allChannelPolicies,omitempty"`

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

// WebBrokerApiDataDeploymentState Desired deployment state
type WebBrokerApiDataDeploymentState string

// WebBrokerApiReceiver Receiver configuration for protocol adapter
type WebBrokerApiReceiver struct {
	// Type Receiver type (websocket, sse)
	Type string `json:"type" yaml:"type"`

	// Properties Receiver-specific configuration properties
	Properties *map[string]interface{} `json:"properties,omitempty" yaml:"properties,omitempty"`
}

// WebBrokerApiBrokerDriver Broker driver configuration
type WebBrokerApiBrokerDriver struct {
	// Type Broker driver type (kafka, mqtt, amqp)
	Type string `json:"type" yaml:"type"`

	// Properties Broker-specific configuration properties (topic, bootstrap.servers, etc.)
	Properties map[string]interface{} `json:"properties" yaml:"properties"`
}

// WebBrokerApiPolicies Protocol mediation policies
type WebBrokerApiPolicies struct {
	// OnConnectionInit Policies applied during WebSocket handshake
	OnConnectionInit *WebBrokerApiConnectionInitPolicies `json:"onConnectionInit,omitempty" yaml:"onConnectionInit,omitempty"`

	// OnProduce Policies applied when client sends message to broker
	OnProduce *[]Policy `json:"onProduce,omitempty" yaml:"onProduce,omitempty"`

	// OnConsume Policies applied when broker message delivered to client
	OnConsume *[]Policy `json:"onConsume,omitempty" yaml:"onConsume,omitempty"`
}

// WebBrokerApiConnectionInitPolicies Connection initialization policies
type WebBrokerApiConnectionInitPolicies struct {
	// Request Policies applied before WebSocket upgrade
	Request *[]Policy `json:"request,omitempty" yaml:"request,omitempty"`

	// Response Policies applied after WebSocket upgrade
	Response *[]Policy `json:"response,omitempty" yaml:"response,omitempty"`
}
