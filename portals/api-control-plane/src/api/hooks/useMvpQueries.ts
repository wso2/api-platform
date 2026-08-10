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

import { useContext } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { ConsoleScopeContext } from '../../scope/ConsoleScopeContext';
import type {
  Api,
  ApiDetail,
  CreateApiInput,
  CreateApiKeyInput,
  CreateDevPortalInput,
  CreateGatewayInput,
  CreateProjectInput,
  DeployApiInput,
  GatewayDeployment,
  Project,
} from '../../types/domain';
import { useApiClient } from '../ApiClientProvider';

export const queryKeys = {
  organizations: ['organizations'] as const,
  organization: (orgHandle: string) => ['organization', orgHandle] as const,
  projects: (orgHandle: string) => ['projects', orgHandle] as const,
  project: (orgHandle: string, projectHandler: string) =>
    ['project', orgHandle, projectHandler] as const,
  components: (orgHandle: string, projectHandler: string) =>
    ['components', orgHandle, projectHandler] as const,
  component: (orgHandle: string, projectHandler: string, apiHandler: string) =>
    ['api', orgHandle, projectHandler, apiHandler] as const,
  componentDetail: (
    orgHandle: string,
    projectHandler: string,
    apiHandler: string
  ) => ['componentDetail', orgHandle, projectHandler, apiHandler] as const,
  environments: ['environments'] as const,
  deployments: (componentId: string) => ['deployments', componentId] as const,
  gatewayDeployments: (orgHandle: string, apiHandler: string) =>
    ['gatewayDeployments', orgHandle, apiHandler] as const,
  apiKeys: (orgHandle: string, apiHandler: string) =>
    ['apiKeys', orgHandle, apiHandler] as const,
  apiProxy: (componentId: string) => ['apiProxy', componentId] as const,
  gateways: (orgHandle: string) => ['gateways', orgHandle] as const,
  gateway: (orgHandle: string, gatewayId: string) =>
    ['gateway', orgHandle, gatewayId] as const,
  devPortals: (orgHandle: string) => ['devPortals', orgHandle] as const,
};

/**
 * Resolves scope identifiers for the data hooks: an explicit argument always
 * wins; otherwise the value falls back to the token-ready `activeScope` from
 * `ConsoleScopeContext`. Reading the context directly (nullable) — rather than
 * `useConsoleScope()` — lets `ConsoleScopeProvider` call these hooks with
 * explicit args while it is still building the scope (context value is null).
 */
const useScopeArgs = (
  orgHandle?: string,
  projectHandler?: string,
  apiHandler?: string
) => {
  const scope = useContext(ConsoleScopeContext);
  return {
    orgHandle: orgHandle ?? scope?.activeScope.orgHandle,
    projectHandler: projectHandler ?? scope?.activeScope.projectHandler,
    apiHandler: apiHandler ?? scope?.activeScope.apiHandler,
  };
};

export const useOrganizations = () => {
  const client = useApiClient();
  return useQuery({
    queryKey: queryKeys.organizations,
    queryFn: client.listOrganizations,
  });
};

export const useOrganization = (orgHandleArg?: string) => {
  const client = useApiClient();
  const { orgHandle } = useScopeArgs(orgHandleArg);
  return useQuery({
    queryKey: queryKeys.organization(orgHandle || ''),
    queryFn: () => {
      if (!orgHandle) throw new Error('orgHandle is required');
      return client.getOrganization(orgHandle);
    },
    enabled: !!orgHandle,
  });
};

export const useProjects = (orgHandleArg?: string) => {
  const client = useApiClient();
  const { orgHandle } = useScopeArgs(orgHandleArg);
  return useQuery({
    queryKey: queryKeys.projects(orgHandle || ''),
    queryFn: () => {
      if (!orgHandle) throw new Error('orgHandle is required to list projects');
      return client.listProjects(orgHandle);
    },
    enabled: !!orgHandle,
  });
};

export const useProject = (
  orgHandleArg?: string,
  projectHandlerArg?: string
) => {
  const client = useApiClient();
  const { orgHandle, projectHandler } = useScopeArgs(
    orgHandleArg,
    projectHandlerArg
  );
  return useQuery({
    queryKey: queryKeys.project(orgHandle || '', projectHandler || ''),
    queryFn: () => {
      if (!orgHandle || !projectHandler) {
        throw new Error('orgHandle and projectHandler are required');
      }
      return client.getProject(orgHandle, projectHandler);
    },
    enabled: !!orgHandle && !!projectHandler,
  });
};

export const useCreateProject = (orgHandleArg?: string) => {
  const client = useApiClient();
  const queryClient = useQueryClient();
  const { orgHandle = '' } = useScopeArgs(orgHandleArg);
  return useMutation({
    mutationFn: (input: CreateProjectInput) =>
      client.createProject(orgHandle, input),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.projects(orgHandle),
      });
    },
  });
};

export const useDeleteProject = (orgHandleArg?: string) => {
  const client = useApiClient();
  const queryClient = useQueryClient();
  const { orgHandle = '' } = useScopeArgs(orgHandleArg);
  return useMutation({
    mutationFn: (project: Project) => client.deleteProject(orgHandle, project),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.projects(orgHandle),
      });
    },
  });
};

export const useApis = (orgHandleArg?: string, projectHandlerArg?: string) => {
  const client = useApiClient();
  const { orgHandle, projectHandler } = useScopeArgs(
    orgHandleArg,
    projectHandlerArg
  );
  return useQuery({
    queryKey: queryKeys.components(orgHandle || '', projectHandler || ''),
    queryFn: () => {
      if (!orgHandle || !projectHandler) {
        throw new Error('orgHandle and projectHandler are required');
      }
      return client.listApis(orgHandle, projectHandler);
    },
    enabled: !!orgHandle && !!projectHandler,
  });
};

export const useApi = (
  orgHandleArg?: string,
  projectHandlerArg?: string,
  apiHandlerArg?: string
) => {
  const client = useApiClient();
  const { orgHandle, projectHandler, apiHandler } = useScopeArgs(
    orgHandleArg,
    projectHandlerArg,
    apiHandlerArg
  );
  return useQuery({
    queryKey: queryKeys.component(
      orgHandle || '',
      projectHandler || '',
      apiHandler || ''
    ),
    queryFn: () => {
      if (!orgHandle || !projectHandler || !apiHandler) {
        throw new Error(
          'orgHandle, projectHandler and apiHandler are required'
        );
      }
      return client.getApi(orgHandle, projectHandler, apiHandler);
    },
    enabled: !!orgHandle && !!projectHandler && !!apiHandler,
  });
};

export const useApiDetail = (
  orgHandleArg?: string,
  projectHandlerArg?: string,
  apiHandlerArg?: string
) => {
  const client = useApiClient();
  const { orgHandle, projectHandler, apiHandler } = useScopeArgs(
    orgHandleArg,
    projectHandlerArg,
    apiHandlerArg
  );
  return useQuery({
    queryKey: queryKeys.componentDetail(
      orgHandle || '',
      projectHandler || '',
      apiHandler || ''
    ),
    queryFn: () => {
      if (!orgHandle || !projectHandler || !apiHandler) {
        throw new Error(
          'orgHandle, projectHandler and apiHandler are required'
        );
      }
      return client.getApiDetail(orgHandle, projectHandler, apiHandler);
    },
    enabled: !!orgHandle && !!projectHandler && !!apiHandler,
  });
};

export const useUpdateApi = (
  orgHandleArg?: string,
  projectHandlerArg?: string
) => {
  const client = useApiClient();
  const queryClient = useQueryClient();
  const { orgHandle = '', projectHandler = '' } = useScopeArgs(
    orgHandleArg,
    projectHandlerArg
  );
  return useMutation({
    mutationFn: (detail: ApiDetail) =>
      client.updateApi(orgHandle, projectHandler, detail),
    onSuccess: (updated) => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.componentDetail(
          orgHandle,
          projectHandler,
          updated.handler
        ),
      });
      // Also invalidate the single-API read (breadcrumb label, capabilities,
      // Deploy/Test/Manage pages) — otherwise it serves stale data after edits.
      queryClient.invalidateQueries({
        queryKey: queryKeys.component(
          orgHandle,
          projectHandler,
          updated.handler
        ),
      });
      queryClient.invalidateQueries({
        queryKey: queryKeys.components(orgHandle, projectHandler),
      });
    },
  });
};

export const useDeleteApi = (
  orgHandleArg?: string,
  projectHandlerArg?: string
) => {
  const client = useApiClient();
  const queryClient = useQueryClient();
  const { orgHandle = '', projectHandler = '' } = useScopeArgs(
    orgHandleArg,
    projectHandlerArg
  );
  return useMutation({
    mutationFn: (api: Api) => client.deleteApi(orgHandle, projectHandler, api),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.components(orgHandle, projectHandler),
      });
    },
  });
};

export const useEnvironments = () => {
  const client = useApiClient();
  return useQuery({
    queryKey: queryKeys.environments,
    queryFn: client.listEnvironments,
  });
};

export const useDeployments = (componentId?: string) => {
  const client = useApiClient();
  return useQuery({
    queryKey: queryKeys.deployments(componentId || ''),
    queryFn: () => {
      if (!componentId) throw new Error('componentId is required');
      return client.listDeployments(componentId);
    },
    enabled: !!componentId,
  });
};

const isTransitioning = (deployments?: GatewayDeployment[]) =>
  Boolean(
    deployments?.some(
      (deployment) =>
        deployment.status === 'DEPLOYING' || deployment.status === 'UNDEPLOYING'
    )
  );

/**
 * Gateway deployments for an API (the platform deploy path). Polls while any
 * deployment is DEPLOYING/UNDEPLOYING — platform-api flips the status when the
 * gateway acknowledges — then stops.
 */
export const useGatewayDeployments = (api?: Api, orgHandleArg?: string) => {
  const client = useApiClient();
  const { orgHandle } = useScopeArgs(orgHandleArg);
  return useQuery({
    queryKey: queryKeys.gatewayDeployments(orgHandle || '', api?.handler || ''),
    queryFn: () => {
      if (!orgHandle || !api) throw new Error('orgHandle and api are required');
      return client.listGatewayDeployments(orgHandle, api);
    },
    enabled: !!orgHandle && !!api,
    refetchInterval: (query) =>
      isTransitioning(query.state.data) ? 4000 : false,
  });
};

/** Invalidates the acted-on API's gateway-deployment list after a mutation. */
const useInvalidateGatewayDeployments = (orgHandle: string) => {
  const queryClient = useQueryClient();
  return (api: Api) =>
    queryClient.invalidateQueries({
      queryKey: queryKeys.gatewayDeployments(orgHandle, api.handler),
    });
};

export const useDeployApi = (orgHandleArg?: string) => {
  const client = useApiClient();
  const { orgHandle = '' } = useScopeArgs(orgHandleArg);
  const invalidate = useInvalidateGatewayDeployments(orgHandle);
  return useMutation({
    mutationFn: ({ api, input }: { api: Api; input: DeployApiInput }) =>
      client.deployApi(orgHandle, api, input),
    onSuccess: (_deployment, { api }) => invalidate(api),
  });
};

export const useUndeployGatewayDeployment = (orgHandleArg?: string) => {
  const client = useApiClient();
  const { orgHandle = '' } = useScopeArgs(orgHandleArg);
  const invalidate = useInvalidateGatewayDeployments(orgHandle);
  return useMutation({
    mutationFn: ({
      api,
      deployment,
    }: {
      api: Api;
      deployment: GatewayDeployment;
    }) => client.undeployGatewayDeployment(orgHandle, api, deployment),
    onSuccess: (_deployment, { api }) => invalidate(api),
  });
};

/** API keys added for an API (masked; the plain value is never readable). */
export const useApiKeys = (api?: Api, orgHandleArg?: string) => {
  const client = useApiClient();
  const { orgHandle } = useScopeArgs(orgHandleArg);
  return useQuery({
    queryKey: queryKeys.apiKeys(orgHandle || '', api?.handler || ''),
    queryFn: () => {
      if (!orgHandle || !api) throw new Error('orgHandle and api are required');
      return client.listApiKeys(orgHandle, api);
    },
    enabled: !!orgHandle && !!api,
  });
};

export const useCreateApiKey = (orgHandleArg?: string) => {
  const client = useApiClient();
  const queryClient = useQueryClient();
  const { orgHandle = '' } = useScopeArgs(orgHandleArg);
  return useMutation({
    mutationFn: ({ api, input }: { api: Api; input: CreateApiKeyInput }) =>
      client.createApiKey(orgHandle, api, input),
    onSuccess: (_result, { api }) => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.apiKeys(orgHandle, api.handler),
      });
    },
  });
};

export const useRevokeApiKey = (orgHandleArg?: string) => {
  const client = useApiClient();
  const queryClient = useQueryClient();
  const { orgHandle = '' } = useScopeArgs(orgHandleArg);
  return useMutation({
    mutationFn: ({ api, keyName }: { api: Api; keyName: string }) =>
      client.revokeApiKey(orgHandle, api, keyName),
    onSuccess: (_result, { api }) => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.apiKeys(orgHandle, api.handler),
      });
    },
  });
};

export const useDeleteGatewayDeployment = (orgHandleArg?: string) => {
  const client = useApiClient();
  const { orgHandle = '' } = useScopeArgs(orgHandleArg);
  const invalidate = useInvalidateGatewayDeployments(orgHandle);
  return useMutation({
    mutationFn: ({
      api,
      deployment,
    }: {
      api: Api;
      deployment: GatewayDeployment;
    }) => client.deleteGatewayDeployment(orgHandle, api, deployment),
    onSuccess: (_result, { api }) => invalidate(api),
  });
};

export const useRestoreGatewayDeployment = (orgHandleArg?: string) => {
  const client = useApiClient();
  const { orgHandle = '' } = useScopeArgs(orgHandleArg);
  const invalidate = useInvalidateGatewayDeployments(orgHandle);
  return useMutation({
    mutationFn: ({
      api,
      deployment,
    }: {
      api: Api;
      deployment: GatewayDeployment;
    }) => client.restoreGatewayDeployment(orgHandle, api, deployment),
    onSuccess: (_deployment, { api }) => invalidate(api),
  });
};

export const useApiProxy = (componentId?: string) => {
  const client = useApiClient();
  return useQuery({
    queryKey: queryKeys.apiProxy(componentId || ''),
    queryFn: () => {
      if (!componentId) throw new Error('componentId is required');
      return client.getApiProxy(componentId);
    },
    enabled: !!componentId,
  });
};

export const useGateways = (orgHandleArg?: string) => {
  const client = useApiClient();
  const { orgHandle } = useScopeArgs(orgHandleArg);
  return useQuery({
    queryKey: queryKeys.gateways(orgHandle || ''),
    queryFn: () => {
      if (!orgHandle) throw new Error('orgHandle is required to list gateways');
      return client.listGateways(orgHandle);
    },
    enabled: !!orgHandle,
  });
};

export const useGateway = (
  orgHandleArg?: string,
  gatewayId?: string,
  options?: { poll?: boolean }
) => {
  const client = useApiClient();
  const { orgHandle } = useScopeArgs(orgHandleArg);
  return useQuery({
    queryKey: queryKeys.gateway(orgHandle || '', gatewayId || ''),
    queryFn: () => {
      if (!orgHandle || !gatewayId) {
        throw new Error('orgHandle and gatewayId are required');
      }
      return client.getGateway(orgHandle, gatewayId);
    },
    enabled: !!orgHandle && !!gatewayId,
    // While waiting for a self-hosted gateway to connect, poll until it is
    // active, then stop.
    refetchInterval: options?.poll
      ? (query) => (query.state.data?.isActive ? false : 5000)
      : false,
  });
};

export const useCreateGateway = (orgHandleArg?: string) => {
  const client = useApiClient();
  const queryClient = useQueryClient();
  const { orgHandle = '' } = useScopeArgs(orgHandleArg);
  return useMutation({
    mutationFn: (input: CreateGatewayInput) =>
      client.createGateway(orgHandle, input),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.gateways(orgHandle),
      });
    },
  });
};

export const useCreateGatewayToken = (
  orgHandleArg?: string,
  gatewayId = ''
) => {
  const client = useApiClient();
  const { orgHandle = '' } = useScopeArgs(orgHandleArg);
  return useMutation({
    mutationFn: () => client.createGatewayToken(orgHandle, gatewayId),
  });
};

export const useDevPortals = (orgHandleArg?: string) => {
  const client = useApiClient();
  const { orgHandle } = useScopeArgs(orgHandleArg);
  return useQuery({
    queryKey: queryKeys.devPortals(orgHandle || ''),
    queryFn: () => {
      if (!orgHandle) {
        throw new Error('orgHandle is required to list dev portals');
      }
      return client.listDevPortals();
    },
    enabled: !!orgHandle,
  });
};

export const useCreateDevPortal = (orgHandleArg?: string) => {
  const client = useApiClient();
  const queryClient = useQueryClient();
  const { orgHandle = '' } = useScopeArgs(orgHandleArg);
  return useMutation({
    mutationFn: (input: CreateDevPortalInput) => client.createDevPortal(input),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.devPortals(orgHandle),
      });
    },
  });
};

export const useCreateApi = (
  orgHandleArg?: string,
  projectHandlerArg?: string
) => {
  const client = useApiClient();
  const queryClient = useQueryClient();
  const { orgHandle = '', projectHandler = '' } = useScopeArgs(
    orgHandleArg,
    projectHandlerArg
  );
  return useMutation({
    mutationFn: (input: CreateApiInput) =>
      client.createApi(orgHandle, projectHandler, input),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.components(orgHandle, projectHandler),
      });
    },
  });
};
