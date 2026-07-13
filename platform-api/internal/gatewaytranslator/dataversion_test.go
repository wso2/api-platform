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

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wso2/api-platform/platform-api/internal/constants"
)

func TestComputeDataVersion(t *testing.T) {
	tests := []struct {
		name       string
		kind       string
		apiVersion string
		want       PlatformDataVersion
	}{
		{"llm provider is 1.1", constants.LLMProvider, constants.GatewayApiVersion, "1.1"},
		{"llm proxy is 1.1", constants.LLMProxy, constants.GatewayApiVersion, "1.1"},
		{"rest api is 1.0", constants.RestApi, constants.GatewayApiVersion, "1.0"},
		{"mcp proxy is 1.0", constants.MCPProxy, constants.GatewayApiVersion, "1.0"},
		{"websub api is 1.0", constants.WebSubApi, constants.GatewayApiVersion, "1.0"},
		{"webbroker api is 1.0", constants.WebBrokerApi, constants.GatewayApiVersion, "1.0"},
		{"legacy v1alpha1 llm provider is still major 1", constants.LLMProvider, constants.GatewayApiVersionV1Alpha1, "1.1"},
		{"empty apiVersion falls back to 1.0", constants.LLMProvider, "", "1.0"},
		{"unparseable apiVersion falls back to 1.0", constants.LLMProvider, "not-a-version", "1.0"},
		{"unknown kind defaults minor to 0", "SomeFutureKind", constants.GatewayApiVersion, "1.0"},
		// Regression: an unknown kind must fall back to the default "1.0" outright,
		// not "<major>.0" for whatever major the apiVersion happens to carry — a
		// v2 gateway apiVersion with an unrecognized kind must not compute "2.0".
		{"unknown kind with non-v1 apiVersion still defaults to 1.0", "SomeFutureKind", "gateway.api-platform.wso2.com/v2", "1.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ComputeDataVersion(tt.kind, tt.apiVersion))
		})
	}
}

func TestGatewayDataVersionForGateway(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    GatewayDataVersion
	}{
		// Empty/blank/unreported version must NOT down-convert: an unversioned
		// gateway is assumed to be latest (v1). This is the regression guard for
		// the e2e failure where the gateway registered without a version and every
		// REST/MCP/WebSub artifact was wrongly shipped as v1alpha1 (→ no route → 404).
		{"empty is latest v1", "", GatewayDataVersionV1},
		{"blank is latest v1", "   ", GatewayDataVersionV1},
		// A gateway that positively reports an old version still down-converts.
		{"reported 1.1 is v1alpha1", "1.1", GatewayDataVersionV1Alpha1},
		{"reported 1.1.9 is v1alpha1", "1.1.9", GatewayDataVersionV1Alpha1},
		{"reported 1.2 is v1", "1.2", GatewayDataVersionV1},
		{"reported 1.2.0 is v1", "1.2.0", GatewayDataVersionV1},
		{"reported 1.3.0 is v1", "1.3.0", GatewayDataVersionV1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, GatewayDataVersionForGateway(tt.version))
		})
	}
}

func TestTargetGatewayDataVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    GatewayDataVersion
	}{
		{"1.2.0 is v1", "1.2.0", GatewayDataVersionV1},
		{"1.3.0 is v1", "1.3.0", GatewayDataVersionV1},
		{"1.1.9 is v1alpha1", "1.1.9", GatewayDataVersionV1Alpha1},
		{"1.1.0 is v1alpha1", "1.1.0", GatewayDataVersionV1Alpha1},
		{"empty is v1alpha1", "", GatewayDataVersionV1Alpha1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, TargetGatewayDataVersion(ParseVersion(tt.version)))
		})
	}
}
