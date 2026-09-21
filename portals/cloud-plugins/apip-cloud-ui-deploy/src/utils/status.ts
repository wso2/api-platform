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

/**
 * How many of an environment's gateways are serving this artifact right now. This is the
 * deployment, not the gateway: it answers "where is this running", which is what the page
 * is about, whereas a gateway being up only says where it could run.
 */
export function deployedGatewayCount(gateways: Gateway[]): number {
  return gateways.filter(
    (gateway) => gateway.status === 'DEPLOYED' || gateway.status === 'DEPLOYING'
  ).length;
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

/**
 * The statuses that mean a gateway is holding a build — on it, going on, or coming
 * off. The platform refuses to delete a build in any of them, so a page says so up
 * front instead of offering the action and having it rejected. Suspended and failed
 * deployments are deliberately absent: their builds ARE deletable, and they are the
 * ones automatic cleanup will not reclaim.
 */
const GATEWAY_HELD_STATUSES: DeploymentStatus[] = ['DEPLOYED', 'DEPLOYING', 'UNDEPLOYING'];

/**
 * Why each build cannot be deleted, by build id, naming the environment holding it so
 * the reason is actionable rather than just a refusal. The platform stays the
 * authority — a gateway may have claimed a build since the page last loaded — so a
 * refusal that comes back is surfaced as it is rather than predicted here.
 */
export function undeletableBuildReasons(environments: Environment[]): Record<string, string> {
  const reasons: Record<string, string> = {};
  environments.forEach((environment) => {
    environment.gateways.forEach((gateway) => {
      if (!gateway.buildId || !gateway.status) return;
      if (!GATEWAY_HELD_STATUSES.includes(gateway.status)) return;
      reasons[gateway.buildId] =
        `This build is on a gateway in ${environment.name}. Undeploy it before deleting the build.`;
    });
  });
  return reasons;
}
