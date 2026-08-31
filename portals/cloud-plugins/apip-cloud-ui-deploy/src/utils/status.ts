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

import type { Gateway, GatewayStatus } from '../types';

/** Maps onto Oxygen/MUI's own `color` prop (Chip, Alert, etc.) so status coloring always follows the active theme instead of hardcoded hex. */
export type Tone = 'success' | 'warning' | 'error' | 'default';

export type StatusTone = {
  label: string;
  tone: Tone;
};

const GATEWAY_STATUS_TONE: Record<GatewayStatus, StatusTone> = {
  active: { label: 'Active', tone: 'success' },
  failed: { label: 'Failed', tone: 'error' },
  deploying: { label: 'Deploying', tone: 'warning' },
  none: { label: 'Not deployed', tone: 'default' },
};

export function gatewayStatusTone(status: GatewayStatus): StatusTone {
  return GATEWAY_STATUS_TONE[status];
}

export function activeGatewayCount(gateways: Gateway[]): number {
  return gateways.filter((gateway) => gateway.status === 'active').length;
}

/** Whether anything has ever been deployed in this environment — gates the Promote button. */
export function hasAnyDeployment(gateways: Gateway[]): boolean {
  return gateways.some((gateway) => gateway.status !== 'none');
}
