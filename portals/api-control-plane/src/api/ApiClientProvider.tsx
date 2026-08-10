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

import { createContext, type ReactNode, useContext } from 'react';

import {
  createApi,
  createApiKey,
  createDevPortal,
  createGateway,
  createGatewayToken,
  createProject,
  deleteApi,
  deleteDevPortal,
  deleteGatewayDeployment,
  deleteProject,
  deployApi,
  getApi,
  getApiDetail,
  getApiProxy,
  getDevPortal,
  getGateway,
  getOrganization,
  getProject,
  listApiKeys,
  listApis,
  listDeployments,
  listDevPortals,
  listEnvironments,
  listGatewayDeployments,
  listGateways,
  listOrganizations,
  listProjects,
  restoreGatewayDeployment,
  revokeApiKey,
  undeployGatewayDeployment,
  updateApi,
  updateDevPortal,
} from './mvpApi';
import {
  getPolicyDefinition,
  listPolicies,
  listPolicyCategories,
} from './policyHub/policyHubClient';

/**
 * The real data-access surface. Every backend call the app makes flows through
 * one of these functions; the React Query hooks resolve them from context (see
 * `useApiClient`) rather than importing them directly, so the transport can be
 * swapped or stubbed (tests) at a single seam.
 */
export const realApiClient = {
  // APIs / components
  listApis,
  getApi,
  getApiDetail,
  createApi,
  updateApi,
  deleteApi,
  // projects / organizations
  listProjects,
  getProject,
  createProject,
  deleteProject,
  listOrganizations,
  getOrganization,
  // gateways
  listGateways,
  getGateway,
  createGateway,
  createGatewayToken,
  // dev portals
  listDevPortals,
  getDevPortal,
  createDevPortal,
  updateDevPortal,
  deleteDevPortal,
  // gateway deployments (the deploy path)
  listGatewayDeployments,
  deployApi,
  undeployGatewayDeployment,
  restoreGatewayDeployment,
  deleteGatewayDeployment,
  // API keys
  listApiKeys,
  createApiKey,
  revokeApiKey,
  // derived data
  listEnvironments,
  listDeployments,
  getApiProxy,
  // policy hub
  listPolicies,
  listPolicyCategories,
  getPolicyDefinition,
};

/** Typed surface, derived from the real client so it never drifts. */
export type ApiClient = typeof realApiClient;

const ApiClientContext = createContext<ApiClient>(realApiClient);

/**
 * Provides the API client to the hook layer. Defaults to the real client;
 * pass `value` (e.g. a partial of stubs merged over the real client) to inject
 * a different implementation in tests or alternate transports.
 */
export function ApiClientProvider({
  value,
  children,
}: {
  value?: ApiClient;
  children: ReactNode;
}) {
  return (
    <ApiClientContext.Provider value={value ?? realApiClient}>
      {children}
    </ApiClientContext.Provider>
  );
}

/** Resolve the API client inside a hook. */
export const useApiClient = (): ApiClient => useContext(ApiClientContext);
