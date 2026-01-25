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

package xdsclient

import (
	"time"
)

const (
	// PolicyChainTypeURL is the custom type URL for policy chain configurations
	PolicyChainTypeURL = "api-platform.wso2.org/v1.PolicyChainConfig"

	// APIKeyStateTypeURL is the custom type URL for API key state (state-of-the-world approach)
	APIKeyStateTypeURL = "api-platform.wso2.org/v1.APIKeyState"

	// LazyResourceTypeURL is the custom type URL for lazy resources
	LazyResourceTypeURL = "api-platform.wso2.org/v1.LazyResources"

	// Default configuration values
	DefaultNodeID                = "policy-engine"
	DefaultCluster               = "policy-engine-cluster"
	DefaultConnectTimeout        = 10 * time.Second
	DefaultRequestTimeout        = 5 * time.Second
	DefaultMaxReconnectDelay     = 60 * time.Second
	DefaultInitialReconnectDelay = 1 * time.Second
)

// ClientState represents the current state of the xDS client
type ClientState int

const (
	StateDisconnected ClientState = iota
	StateConnecting
	StateConnected
	StateReconnecting
	StateStopped
)

func (s ClientState) String() string {
	switch s {
	case StateDisconnected:
		return "Disconnected"
	case StateConnecting:
		return "Connecting"
	case StateConnected:
		return "Connected"
	case StateReconnecting:
		return "Reconnecting"
	case StateStopped:
		return "Stopped"
	default:
		return "Unknown"
	}
}
