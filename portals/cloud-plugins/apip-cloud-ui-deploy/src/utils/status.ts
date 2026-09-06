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

import type { DeploymentStatus, Environment, Gateway } from '../types';

/** Maps onto Oxygen/MUI's own `color` prop (Chip, Alert, etc.) so status coloring always follows the active theme instead of hardcoded hex. */
export type Tone = 'success' | 'warning' | 'error' | 'default';

export type StatusTone = {
  label: string;
  tone: Tone;
};

/**
 * The card's status vocabulary, which is deliberately not the API's: a person
 * reads a gateway as active or suspended, not deployed or undeployed. `ARCHIVED`
 * shows as superseded, because a newer deployment has replaced it.
 */
const DEPLOYMENT_STATUS_TONE: Record<DeploymentStatus, StatusTone> = {
  DEPLOYED: { label: 'Active', tone: 'success' },
  DEPLOYING: { label: 'Deploying', tone: 'warning' },
  UNDEPLOYED: { label: 'Suspended', tone: 'default' },
  UNDEPLOYING: { label: 'Stopping', tone: 'warning' },
  FAILED: { label: 'Failed', tone: 'error' },
  ARCHIVED: { label: 'Superseded', tone: 'default' },
  NOT_DEPLOYED: { label: 'Not deployed', tone: 'default' },
};

export function gatewayStatusTone(status: DeploymentStatus): StatusTone {
  return DEPLOYMENT_STATUS_TONE[status] ?? DEPLOYMENT_STATUS_TONE.NOT_DEPLOYED;
}

/**
 * How many of an environment's gateways are up. This counts gateway health, not
 * deployments: it answers "is there anywhere to deploy to", which is what gates
 * Deploy and Promote.
 */
export function activeGatewayCount(gateways: Gateway[]): number {
  return gateways.filter((gateway) => gateway.health === 'active').length;
}

/** Whether anything has ever been deployed in this environment — gates the Promote button. */
export function hasAnyDeployment(gateways: Gateway[]): boolean {
  return gateways.some((gateway) => gateway.status !== 'NOT_DEPLOYED');
}

/** Whether a deployment is mid-transition, which is what keeps the view polling. */
export function isSettling(environments: Environment[]): boolean {
  return environments.some((environment) =>
    environment.gateways.some(
      (gateway) => gateway.status === 'DEPLOYING' || gateway.status === 'UNDEPLOYING'
    )
  );
}
