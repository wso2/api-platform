/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package constants

// ValidLifecycleStates Valid lifecycle states
var ValidLifecycleStates = map[string]bool{
	"STAGED":     true,
	"CREATED":    true,
	"PUBLISHED":  true,
	"DEPRECATED": true,
	"RETIRED":    true,
	"BLOCKED":    true,
}

// ValidAPITypes Valid API types
var ValidAPITypes = map[string]bool{
	"HTTP":       true,
	"WS":         true,
	"SOAPTOREST": true,
	"SOAP":       true,
	"GRAPHQL":    true,
	"WEBSUB":     true,
	"SSE":        true,
	"WEBHOOK":    true,
	"ASYNC":      true,
}

// ValidTransports Valid transport protocols
var ValidTransports = map[string]bool{
	"http":  true,
	"https": true,
	"ws":    true,
	"wss":   true,
}

// Gateway Functionality Type Constants
const (
	GatewayFunctionalityTypeRegular = "regular"
	GatewayFunctionalityTypeAI      = "ai"
	GatewayFunctionalityTypeEvent   = "event"
)

// ValidGatewayFunctionalityType Valid gateway functionality types
var ValidGatewayFunctionalityType = map[string]bool{
	GatewayFunctionalityTypeRegular: true,
	GatewayFunctionalityTypeAI:      true,
	GatewayFunctionalityTypeEvent:   true,
}

// DefaultGatewayFunctionalityType Default gateway functionality type for new gateways
const DefaultGatewayFunctionalityType = GatewayFunctionalityTypeRegular

// API Type Constants
const (
	APITypeHTTP       = "HTTP"
	APITypeWS         = "WS"
	APITypeSOAPToREST = "SOAPTOREST"
	APITypeSOAP       = "SOAP"
	APITypeGraphQL    = "GRAPHQL"
	APITypeWebSub     = "WEBSUB"
	APITypeSSE        = "SSE"
	APITypeWebhook    = "WEBHOOK"
	APITypeAsync      = "ASYNC"
)

// API SubType Constants
const (
	APISubTypeHTTP      = "REST"
	APISubTypeGraphQL   = "GQL"
	APISubTypeAsync     = "ASYNC"
	APISubTypeWebSocket = "WEBSOCKET"
	APISubTypeSOAP      = "SOAP"
)

// Artifact Type Constants
const (
	ArtifactTypeAPI        = "API"
	ArtifactTypeMCP        = "MCP"
	ArtifactTypeAPIProduct = "API_PRODUCT"
)

// ValidArtifactTypes Valid artifact types deployed to gateways
var ValidArtifactTypes = map[string]bool{
	ArtifactTypeAPI:        true,
	ArtifactTypeMCP:        true,
	ArtifactTypeAPIProduct: true,
}

// Constants for association types
const (
	AssociationTypeGateway   = "gateway"
	AssociationTypeDevPortal = "dev_portal"
)

// Deployment limit constants
const (
	// DeploymentLimitBuffer is the buffer added to MaxPerAPIGateway for hard limit enforcement
	DeploymentLimitBuffer = 5
)
