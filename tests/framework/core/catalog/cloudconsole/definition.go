/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cloudconsole

import "github.com/wso2/api-platform/tests/framework/core/components"

const (
	// Name is the catalog name of the external APIP cloud target.
	Name = "cloud-console"

	// EndpointAPIPPlatformAPI is the apip-platform-api control-plane endpoint,
	// reached through the external CP gateway.
	EndpointAPIPPlatformAPI = "apip-platform-api"

	// EndpointGatewayControlPlane is where a self-hosted gateway's controller connects: the cloud
	// console's public host followed by /api-management/connect, the address the console's
	// gateway setup steps give a customer as APIP_GW_CONTROLLER_CONTROLPLANE_HOST.
	EndpointGatewayControlPlane = "gateway-control-plane"

	// gatewayConnectPath is the path on the cloud console host that the platform's route forwards,
	// with the prefix stripped, to apip-platform-api's gateway endpoints.
	gatewayConnectPath = "/api-management/connect"
)

// CloudConsole returns the externally hosted APIP cloud target.
func CloudConsole() *components.Definition {
	return &components.Definition{
		Name:  Name,
		Alias: Name,
		External: &components.ExternalSpec{
			Endpoints: []components.ExternalEndpoint{
				{Name: EndpointAPIPPlatformAPI},
				{Name: EndpointGatewayControlPlane},
			},
			RequiredParameters: []string{EnvironmentParameter},
			Resolve:            resolveEndpoints,
		},
	}
}
