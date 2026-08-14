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

import type { DeploymentResponse, DeploymentStatus } from './types';

/**
 * Deployment statuses the Platform API counts as "active" when deciding whether
 * a gateway-created artifact may be deleted.
 */
export const ACTIVE_DEPLOYMENT_STATUSES: readonly DeploymentStatus[] = [
  'DEPLOYED',
  'DEPLOYING',
  'UNDEPLOYING',
];

/** True when any deployment in `deployments` is in a delete-blocking state. */
export function hasActiveDeployment(
  deployments: DeploymentResponse[] | undefined | null
): boolean {
  return (deployments ?? []).some((deployment) =>
    (ACTIVE_DEPLOYMENT_STATUSES as readonly string[]).includes(
      String(deployment?.status ?? '').toUpperCase()
    )
  );
}

/** Count of deployments in a delete-blocking state, for tooltip copy. */
export function countActiveDeployments(
  deployments: DeploymentResponse[] | undefined | null
): number {
  return (deployments ?? []).filter((deployment) =>
    (ACTIVE_DEPLOYMENT_STATUSES as readonly string[]).includes(
      String(deployment?.status ?? '').toUpperCase()
    )
  ).length;
}

/**
 * Tooltip reason for a gateway-created artifact that still has active
 * deployments. `artifactType` is the user-facing kind ("LLM Provider",
 * "App LLM Proxy", "MCP Proxy").
 */
export function activeDeploymentDeleteBlockedReason(
  artifactType: string,
  activeCount: number
): string {
  const gatewayLabel = activeCount === 1 ? 'gateway' : 'gateways';
  const countLabel = activeCount > 0 ? `${activeCount} ${gatewayLabel}` : 'a gateway';
  return (
    `This ${artifactType} was created from a gateway and is still deployed on ${countLabel}. ` +
    `Undeploy it from the ${gatewayLabel} first, then delete it here.`
  );
}

/**
 * Tooltip reason for an LLM Provider that still has App LLM Proxies built on
 * it. Deleting the provider would orphan them, so they have to go first.
 */
export function linkedProxiesDeleteBlockedReason(linkedProxyCount: number): string {
  const proxyLabel = linkedProxyCount === 1 ? 'App LLM Proxy' : 'App LLM Proxies';
  const usageVerb = linkedProxyCount === 1 ? 'is' : 'are';
  return (
    `${linkedProxyCount} ${proxyLabel} ${usageVerb} using this LLM Provider. ` +
    `Delete or repoint ${linkedProxyCount === 1 ? 'it' : 'them'} before deleting the provider.`
  );
}
