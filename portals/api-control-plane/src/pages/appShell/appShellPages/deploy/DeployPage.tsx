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

import { AppPage } from '@/components/AppPage';
import {
  useDeleteDeployment,
  useDeployApi,
  useDeployments,
  useRestoreDeployment,
  useUndeployDeployment,
} from '@/api/resources/restApis/deployments';
import { useRestApi } from '@/api/resources/restApis';
import { routes } from '@/routes/paths';
import { ScopeGate } from '@/scope/ScopeGate';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { ApiDeployView, type DeployMutationHooks } from './components/ApiDeployView';
import type { UseDeployMutationHook } from './components/GatewayDeployCard';
import type { UseDeploymentMutationHook } from './components/GatewayDeploymentSelector';

/** Adapts REST's `{ restApiId, ... }` mutations to the shared deploy-card
 * subtree's uniform `{ apiId, ... }` shape — see `GatewayDeployCard` for why
 * this lives at the caller rather than in the shared component. */
const useDeployForCard: UseDeployMutationHook = () => {
  const mutation = useDeployApi();
  return {
    isPending: mutation.isPending,
    mutate: ({ apiId, body }, options) => mutation.mutate({ restApiId: apiId, body }, options),
  };
};

const useUndeployForCard: UseDeploymentMutationHook = () => {
  const mutation = useUndeployDeployment();
  return {
    isPending: mutation.isPending,
    mutate: ({ apiId, deploymentId }, options) =>
      mutation.mutate({ restApiId: apiId, deploymentId }, options),
  };
};

const useRestoreForCard: UseDeploymentMutationHook = () => {
  const mutation = useRestoreDeployment();
  return {
    isPending: mutation.isPending,
    mutate: ({ apiId, deploymentId }, options) =>
      mutation.mutate({ restApiId: apiId, deploymentId }, options),
  };
};

const useDeleteForCard: UseDeploymentMutationHook = () => {
  const mutation = useDeleteDeployment();
  return {
    isPending: mutation.isPending,
    mutate: ({ apiId, deploymentId }, options) =>
      mutation.mutate({ restApiId: apiId, deploymentId }, options),
  };
};

const REST_DEPLOY_MUTATIONS: DeployMutationHooks = {
  useDelete: useDeleteForCard,
  useDeploy: useDeployForCard,
  useRestore: useRestoreForCard,
  useUndeploy: useUndeployForCard,
};

/** A REST API's Deploy page: the shared `ApiDeployView` over REST's data. */
export function DeployPage() {
  return (
    <AppPage>
      <ScopeGate
        graphqlTo={routes.graphqlApiDeploy}
        prompt="Deployments are made for a single API."
        requires="api"
        to={routes.apiDeploy}
      >
        <Deploy />
      </ScopeGate>
    </AppPage>
  );
}

function Deploy() {
  const { params } = useConsoleScope();
  const apiQuery = useRestApi(params.apiHandler);
  // The deployments query is gated on the handle rather than on the loaded API:
  // both are keyed by the same value, so waiting for the API detail would delay
  // this request for no benefit.
  const deploymentsQuery = useDeployments(params.apiHandler);

  return (
    <ApiDeployView
      api={apiQuery.data}
      apiId={apiQuery.data?.id ?? params.apiHandler ?? ''}
      apiPending={apiQuery.isPending}
      deployments={deploymentsQuery.data?.list ?? []}
      deploymentsFetching={deploymentsQuery.isFetching}
      mutations={REST_DEPLOY_MUTATIONS}
      onRefreshDeployments={() => void deploymentsQuery.refetch()}
      orgHandle={params.orgHandle ?? ''}
      // A gateway-created API is owned by that gateway: deployments are shown,
      // but every action that changes them is disabled.
      readOnly={apiQuery.data?.readOnly === true}
    />
  );
}
