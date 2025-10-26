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

// Gateway Type Constants
const (
	GatewayTypeRegular = "regular"
	GatewayTypeAI      = "ai"
	GatewayTypeEvent   = "event"
)

// ValidGatewayTypes Valid gateway types
var ValidGatewayTypes = map[string]bool{
	GatewayTypeRegular: true,
	GatewayTypeAI:      true,
	GatewayTypeEvent:   true,
}

// DefaultGatewayType Default gateway type for new gateways
const DefaultGatewayType = GatewayTypeRegular
