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

import type { Gateway } from '../../../../types/domain';

/**
 * Client-side mock environments. Until the dedicated environment service exists,
 * gateways are grouped under these for the listing UX. Replace
 * `environmentForGateway` with the real gateway↔environment association later.
 */
export type GatewayEnvironment = {
  id: string;
  name: string;
};

export const MOCK_ENVIRONMENTS: GatewayEnvironment[] = [
  { id: 'development', name: 'Development' },
  { id: 'production', name: 'Production' },
];

/**
 * Deterministically buckets a gateway into a mock environment by a stable hash
 * of its id (demo placeholder — no backend environment data yet).
 */
export const environmentForGateway = (gateway: Gateway): GatewayEnvironment => {
  const hash = [...gateway.id].reduce((sum, ch) => sum + ch.charCodeAt(0), 0);
  return MOCK_ENVIRONMENTS[hash % MOCK_ENVIRONMENTS.length];
};

export type GatewayEnvironmentGroup = {
  environment: GatewayEnvironment;
  gateways: Gateway[];
};

/** Groups gateways under their (mock) environment, preserving env order. */
export const groupGatewaysByEnvironment = (
  gateways: Gateway[]
): GatewayEnvironmentGroup[] => {
  const byEnv = new Map<string, Gateway[]>();
  for (const gateway of gateways) {
    const env = environmentForGateway(gateway);
    const list = byEnv.get(env.id) || [];
    list.push(gateway);
    byEnv.set(env.id, list);
  }
  return MOCK_ENVIRONMENTS.filter((env) => byEnv.has(env.id)).map((env) => ({
    environment: env,
    gateways: byEnv.get(env.id) || [],
  }));
};
