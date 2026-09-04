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

import type { DeploymentStatus, Gateway } from '../types';

/** Maps onto Oxygen's own `color` prop so status coloring follows the active theme. */
export type Tone = 'success' | 'warning' | 'error' | 'default';

export type StatusTone = {
  label: string;
  tone: Tone;
};

/**
 * The card's status vocabulary, which is deliberately not the API's: a person
 * reads a gateway as active or suspended, not deployed or undeployed. `ARCHIVED`
 * shows as superseded because a newer deployment has replaced it.
 */
const STATUS_TONE: Record<DeploymentStatus, StatusTone> = {
  DEPLOYED: { label: 'Active', tone: 'success' },
  DEPLOYING: { label: 'Deploying', tone: 'warning' },
  UNDEPLOYED: { label: 'Suspended', tone: 'default' },
  UNDEPLOYING: { label: 'Stopping', tone: 'warning' },
  FAILED: { label: 'Failed', tone: 'error' },
  ARCHIVED: { label: 'Superseded', tone: 'default' },
  none: { label: 'Not deployed', tone: 'default' },
};

export function gatewayStatusTone(status: DeploymentStatus): StatusTone {
  return STATUS_TONE[status] ?? STATUS_TONE.none;
}

export function activeGatewayCount(gateways: Gateway[]): number {
  return gateways.filter((gateway) => gateway.status === 'DEPLOYED').length;
}

/**
 * Whether this environment can be promoted out of. It requires a gateway that is
 * actually serving — a failed or still-deploying gateway is not something to
 * promote — which mirrors the rule the API enforces on the promotion itself.
 */
export function hasAnyDeployment(gateways: Gateway[]): boolean {
  return gateways.some((gateway) => gateway.status === 'DEPLOYED');
}

/** Explanations for the `statusReason` codes the API returns on a failure. */
const STATUS_REASONS: Record<string, string> = {
  GATEWAY_PROCESSING_ERROR: 'The gateway could not process the deployment. Check its logs.',
  DEPLOYMENT_TIMEOUT: 'The deployment timed out; the gateway did not respond.',
};

/**
 * A readable explanation for a failure code, falling back to the raw code — it is
 * server data, so it reaches the user unchanged rather than guessed at.
 */
export function statusReasonText(reason: string): string {
  return STATUS_REASONS[reason] ?? reason;
}
