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

import type { RestApi } from '@/api/resources/restApis';
import { useDeployments } from '@/api/resources/restApis/deployments/deployments.hooks';

/**
 * Display helpers for the spec's `RESTAPI` shape — the presentation half of
 * `api/resources/restApis`, kept out of the resource layer so that layer stays
 * transport + cache only.
 *
 * Everything here is derived from spec fields (`kind`, `lifeCycleStatus`,
 * `operations`, `readOnly`) or from the API's deployments, so a spec change
 * surfaces as a typecheck failure here rather than as a wrong label.
 */

export type ChipColor = 'default' | 'error' | 'info' | 'secondary' | 'success' | 'warning';

/* -------------------------------------------------------------------------- */
/* Kind                                                                       */
/* -------------------------------------------------------------------------- */

/**
 * `kind` is an open string in the spec (default `RestApi`), not an enum, so this
 * is a lookup with a humanizing fallback rather than an exhaustive record — a
 * kind the console has never heard of still renders as words, not as raw
 * PascalCase.
 */
const KIND_LABEL: Record<string, string> = {
  AsyncApi: 'Async API',
  GraphQL: 'GraphQL API',
  LlmProxy: 'LLM Proxy',
  McpProxy: 'MCP Proxy',
  RestApi: 'REST API',
};

export const apiKindLabel = (kind?: string): string => {
  if (!kind) return 'API';
  return KIND_LABEL[kind] ?? kind.replace(/([a-z0-9])([A-Z])/g, '$1 $2');
};

/**
 * Monogram for the card avatar: the first two letters of the API's name.
 *
 * Deliberately not word initials — "Echo Header API" reads as `EC`, not `EHA`.
 * Two adjacent letters of the actual name are easier to match back to the title
 * underneath it, and every API has at least one word while few have three.
 */
export const apiInitials = (displayName?: string): string =>
  (displayName ?? '')
    .replace(/[^\p{L}\p{N}]/gu, '')
    .slice(0, 2)
    .toUpperCase();

/* -------------------------------------------------------------------------- */
/* Lifecycle                                                                  */
/* -------------------------------------------------------------------------- */

export type LifeCycleStatus = NonNullable<RestApi['lifeCycleStatus']>;

/**
 * One tone per lifecycle state, so the chip's colour carries the status on its
 * own — scanning a grid should not require reading every label.
 *
 * The tones follow the palette's own semantics rather than being picked by
 * hue: `info` for the neutral-but-live states before publication, `success`
 * once traffic is served, `warning` for a state callers must migrate off,
 * `error` where calls are refused. `RETIRED` is the deliberate exception and
 * stays neutral — an end-of-life API should recede, and a sixth saturated tone
 * would compete with the states that still need attention.
 */
const LIFECYCLE_META: Record<LifeCycleStatus, { color: ChipColor; label: string }> = {
  BLOCKED: { color: 'error', label: 'Blocked' },
  CREATED: { color: 'info', label: 'Created' },
  DEPRECATED: { color: 'warning', label: 'Deprecated' },
  PUBLISHED: { color: 'success', label: 'Published' },
  RETIRED: { color: 'default', label: 'Retired' },
  STAGED: { color: 'secondary', label: 'Staged' },
};

export const lifecycleMeta = (status?: LifeCycleStatus): { color: ChipColor; label: string } =>
  status ? LIFECYCLE_META[status] : { color: 'default', label: 'Unknown' };

/* -------------------------------------------------------------------------- */
/* Deployment state                                                           */
/* -------------------------------------------------------------------------- */

export type ApiDeploymentState = 'ACTIVE' | 'SETTLING' | 'FAILED' | 'NONE';

const DEPLOYMENT_META: Record<ApiDeploymentState, { color: ChipColor; label: string }> = {
  ACTIVE: { color: 'success', label: 'Active' },
  FAILED: { color: 'error', label: 'Failed' },
  NONE: { color: 'default', label: 'Not deployed' },
  SETTLING: { color: 'warning', label: 'Deploying' },
};

export const deploymentMeta = (state: ApiDeploymentState): { color: ChipColor; label: string } =>
  DEPLOYMENT_META[state];

/**
 * Runtime state of one API, derived from its deployments.
 *
 * A transition outranks a live deployment: an API that is deployed on one
 * gateway while a second is still settling reads as "Deploying", because that
 * is the state the user is waiting on. The underlying query polls itself until
 * nothing is transitioning, so the card settles without the caller doing
 * anything.
 */
export const useApiDeploymentState = (
  restApiId?: string,
): {
  gatewayIds: string[];
  isPending: boolean;
  state: ApiDeploymentState;
} => {
  const query = useDeployments(restApiId);
  const deployments = query.data?.list ?? [];

  const deployed = deployments.filter((deployment) => deployment.status === 'DEPLOYED');
  const settling = deployments.some(
    (deployment) => deployment.status === 'DEPLOYING' || deployment.status === 'UNDEPLOYING',
  );
  const failed = deployments.some((deployment) => deployment.status === 'FAILED');

  const state: ApiDeploymentState = settling
    ? 'SETTLING'
    : deployed.length > 0
      ? 'ACTIVE'
      : failed
        ? 'FAILED'
        : 'NONE';

  return {
    gatewayIds: [...new Set(deployed.map((deployment) => deployment.gatewayId))],
    isPending: query.isPending,
    state,
  };
};
