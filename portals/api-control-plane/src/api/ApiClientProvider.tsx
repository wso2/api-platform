import { createContext, type ReactNode, useContext } from 'react';

import {
  createApi,
  createApiKey,
  createGateway,
  createGatewayToken,
  createProject,
  deleteApi,
  deleteGatewayDeployment,
  deleteProject,
  deployApi,
  getApi,
  getApiDetail,
  getApiProxy,
  getGateway,
  getOrganization,
  getProject,
  listApiKeys,
  listApis,
  listDeployments,
  listEnvironments,
  listGatewayDeployments,
  listGateways,
  listOrganizations,
  listProjects,
  restoreGatewayDeployment,
  revokeApiKey,
  undeployGatewayDeployment,
  updateApi,
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
