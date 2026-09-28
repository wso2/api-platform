/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package gatewaytranslator

import "github.com/wso2/api-platform/platform-api/internal/constants"

// platformToGatewayKind maps each control-plane artifact kind to the kind its
// deployment artifact carries on the gateway. Every kind is listed explicitly —
// including the ones whose name is the same on both sides — so a kind that is
// missing here is reported as unknown rather than passed through unchanged.
// AgentProxy is the one kind whose names differ (CP AgentProxy, gateway Agent).
var platformToGatewayKind = map[string]string{
	constants.RestApi:      constants.RestApi,
	constants.WebSubApi:    constants.WebSubApi,
	constants.WebBrokerApi: constants.WebBrokerApi,
	constants.MCPProxy:     constants.MCPProxy,
	constants.LLMProxy:     constants.LLMProxy,
	constants.LLMProvider:  constants.LLMProvider,
	constants.AgentProxy:   constants.GatewayKindAgent,
}

// gatewayToPlatformKind is the inverse of platformToGatewayKind.
var gatewayToPlatformKind = func() map[string]string {
	inverse := make(map[string]string, len(platformToGatewayKind))
	for platformKind, gatewayKind := range platformToGatewayKind {
		inverse[gatewayKind] = platformKind
	}
	return inverse
}()

// GatewayKindForPlatformKind returns the gateway artifact kind a control-plane
// kind is deployed as. ok is false for a kind with no registered mapping.
func GatewayKindForPlatformKind(platformKind string) (string, bool) {
	gatewayKind, ok := platformToGatewayKind[platformKind]
	return gatewayKind, ok
}

// PlatformKindForGatewayKind returns the control-plane kind a gateway artifact
// kind is stored as. ok is false for a kind with no registered mapping.
func PlatformKindForGatewayKind(gatewayKind string) (string, bool) {
	platformKind, ok := gatewayToPlatformKind[gatewayKind]
	return platformKind, ok
}

// ComputeDataVersionForGatewayKind is ComputeDataVersion for an artifact that
// arrives in the gateway's vocabulary (an import or an acknowledgement). The
// gateway kind is translated to the control-plane kind first, so a gateway Agent
// is versioned by the AgentProxy entry rather than falling through to the
// unknown-kind default. An unmapped gateway kind yields that default, exactly as
// ComputeDataVersion does for an unknown control-plane kind.
func ComputeDataVersionForGatewayKind(gatewayKind string, apiVersion string) PlatformDataVersion {
	platformKind, ok := PlatformKindForGatewayKind(gatewayKind)
	if !ok {
		return defaultPlatformDataVersion
	}
	return ComputeDataVersion(platformKind, apiVersion)
}
