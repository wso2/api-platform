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

import type { GatewayStatus, GatewayType } from '../types';

/**
 * The gateway type as it is named on screen. Matches the built-in console's own
 * labels (`regular` is an API gateway), so the same gateway reads the same way
 * whichever portal shows it.
 */
export function gatewayTypeLabel(type: GatewayType): string {
  if (type === 'ai') return 'AI Gateway';
  if (type === 'event') return 'Event Gateway';
  return 'API Gateway';
}

/**
 * How a gateway's status reads on screen, and the severity it reads at.
 *
 * `inactive` is a warning rather than an error: a gateway is briefly
 * disconnected across a restart, and that is not something to alarm anyone
 * about. `failed` is the one real error — it will not resolve on its own.
 */
export function gatewayStatusLabel(status: GatewayStatus): string {
  switch (status) {
    case 'provisioning':
      return 'Provisioning';
    case 'failed':
      return 'Failed';
    case 'active':
      return 'Active';
    default:
      return 'Inactive';
  }
}

export function gatewayStatusColor(status: GatewayStatus): 'success' | 'error' | 'info' | 'warning' {
  switch (status) {
    case 'provisioning':
      return 'info';
    case 'failed':
      return 'error';
    case 'active':
      return 'success';
    default:
      return 'warning';
  }
}
