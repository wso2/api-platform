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

package gateway

import (
	"fmt"
	"testing"

	"github.com/wso2/api-platform/cli/utils"
)

func TestGetResourceHandler_KnownKinds(t *testing.T) {
	tests := []struct {
		kind          string
		wantCreate    string
		wantGetUpdate string
	}{
		{ResourceKindRestAPI, utils.GatewayAPIsPath, utils.GatewayAPIByIDPath},
		{ResourceKindMCP, utils.GatewayMCPProxiesPath, utils.GatewayMCPProxyByIDPath},
		{ResourceKindLLMProvider, utils.GatewayLLMProvidersPath, utils.GatewayLLMProviderByIDPath},
		{ResourceKindLLMProxy, utils.GatewayLLMProxiesPath, utils.GatewayLLMProxyByIDPath},
		{ResourceKindGraphQLAPI, utils.GatewayGraphQLAPIsPath, utils.GatewayGraphQLAPIByIDPath},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			handler := GetResourceHandler(tt.kind)
			if handler == nil {
				t.Fatalf("GetResourceHandler(%q) = nil, want a handler", tt.kind)
			}
			if got := handler.CreateEndpoint(); got != tt.wantCreate {
				t.Errorf("CreateEndpoint() = %q, want %q", got, tt.wantCreate)
			}
			wantByID := fmt.Sprintf(tt.wantGetUpdate, "my-handle")
			if got := handler.GetEndpoint("my-handle"); got != wantByID {
				t.Errorf("GetEndpoint() = %q, want %q", got, wantByID)
			}
			if got := handler.UpdateEndpoint("my-handle"); got != wantByID {
				t.Errorf("UpdateEndpoint() = %q, want %q", got, wantByID)
			}
		})
	}
}

func TestGetResourceHandler_UnknownKind(t *testing.T) {
	if handler := GetResourceHandler("SomethingUnsupported"); handler != nil {
		t.Errorf("GetResourceHandler(unknown) = %v, want nil", handler)
	}
}
