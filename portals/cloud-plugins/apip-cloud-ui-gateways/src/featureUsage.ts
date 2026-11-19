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

/*
 * The API Platform console features this page reports, in the console's own
 * terms (`portals/api-control-plane/src/featureUsage/api-platform-taxonomy.yaml`):
 * the built-in Gateways page reports the same codes and dimensions, so a console
 * usage dashboard counts a cloud gateway like any other.
 */

export const GATEWAY_ADD_OPEN = 'apip.gateway.gateway-add-open';
export const GATEWAY_REGISTER_CLICK = 'apip.gateway.gateway-register-click';
export const GATEWAY_UPDATE_CLICK = 'apip.gateway.gateway-update-click';
export const GATEWAY_TOKEN_REGENERATE_CLICK = 'apip.gateway.gateway-token-regenerate-click';
export const GATEWAY_SETUP_COMMAND_COPY = 'apip.gateway.gateway-setup-command-copy';
export const GATEWAY_POLICIES_VIEW = 'apip.policies.gateway-policies-view';

/** The taxonomy's `install_method` for each install path the overview offers. */
export const INSTALL_METHOD_DIMENSION = {
  'Quick Start': 'quick-start',
  'Virtual Machine': 'virtual-machine',
  Docker: 'docker',
  Kubernetes: 'kubernetes',
} as const;

/** Where the add-gateway form was opened from: the taxonomy's `entry_point` for `GATEWAY_ADD_OPEN`. */
export type GatewayAddEntryPoint = 'gateways-list' | 'gateways-empty-state';
