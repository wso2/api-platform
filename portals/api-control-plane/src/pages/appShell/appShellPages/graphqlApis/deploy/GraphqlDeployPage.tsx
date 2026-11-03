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

import { defineMessages, useIntl } from 'react-intl';
import { useParams } from 'react-router-dom';

import { AppPage } from '@/components/AppPage';
import { useGraphQLApi } from '@/api/resources/graphqlApis';
import {
  useDeleteDeployment,
  useDeployApi,
  useDeployments,
  useRestoreDeployment,
  useUndeployDeployment,
} from '@/api/resources/graphqlApis/deployments';
import { ErrorState } from '@/components/StateViews';
import { ApiDeployView, type DeployMutationHooks } from '../../deploy/components/ApiDeployView';
import type { UseDeployMutationHook } from '../../deploy/components/GatewayDeployCard';
import type { UseDeploymentMutationHook } from '../../deploy/components/GatewayDeploymentSelector';

/** Adapts GraphQL's `{ graphqlApiId, ... }` mutations to the shared deploy-card
 * subtree's uniform `{ apiId, ... }` shape — see `GatewayDeployCard` for why
 * this lives at the caller rather than in the shared component. */
const useDeployForCard: UseDeployMutationHook = () => {
  const mutation = useDeployApi();
  return {
    isPending: mutation.isPending,
    mutate: ({ apiId, body }, options) => mutation.mutate({ graphqlApiId: apiId, body }, options),
  };
};

const useUndeployForCard: UseDeploymentMutationHook = () => {
  const mutation = useUndeployDeployment();
  return {
    isPending: mutation.isPending,
    mutate: ({ apiId, deploymentId }, options) =>
      mutation.mutate({ graphqlApiId: apiId, deploymentId }, options),
  };
};

const useRestoreForCard: UseDeploymentMutationHook = () => {
  const mutation = useRestoreDeployment();
  return {
    isPending: mutation.isPending,
    mutate: ({ apiId, deploymentId }, options) =>
      mutation.mutate({ graphqlApiId: apiId, deploymentId }, options),
  };
};

const useDeleteForCard: UseDeploymentMutationHook = () => {
  const mutation = useDeleteDeployment();
  return {
    isPending: mutation.isPending,
    mutate: ({ apiId, deploymentId }, options) =>
      mutation.mutate({ graphqlApiId: apiId, deploymentId }, options),
  };
};

const GRAPHQL_DEPLOY_MUTATIONS: DeployMutationHooks = {
  useDelete: useDeleteForCard,
  useDeploy: useDeployForCard,
  useRestore: useRestoreForCard,
  useUndeploy: useUndeployForCard,
};

const messages = defineMessages({
  apiNotFound: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.GraphqlDeployPage.apiNotFound',
    defaultMessage: 'GraphQL API not found',
  },
});

/**
 * A GraphQL API's Deploy page: the REST page's `ApiDeployView` over GraphQL's
 * own data and mutations. No `ScopeGate`: this route lives outside
 * `ConsoleScopeProvider`'s REST-only api-scope matching (see
 * `graphqlApiPath`), so it guards on its own route param instead.
 */
export function GraphqlDeployPage() {
  return (
    <AppPage>
      <GraphqlDeployPageContent />
    </AppPage>
  );
}

function GraphqlDeployPageContent() {
  const intl = useIntl();
  const { orgHandle = '', graphqlApiHandler } = useParams();
  const apiQuery = useGraphQLApi(graphqlApiHandler);
  const deploymentsQuery = useDeployments(graphqlApiHandler);
  const apiNotFoundTitle = intl.formatMessage(messages.apiNotFound);

  if (!graphqlApiHandler || apiQuery.error) return <ErrorState title={apiNotFoundTitle} />;

  return (
    <ApiDeployView
      api={apiQuery.data}
      apiId={graphqlApiHandler}
      apiNotFoundTitle={apiNotFoundTitle}
      apiPending={apiQuery.isPending}
      deployments={deploymentsQuery.data?.list ?? []}
      deploymentsFetching={deploymentsQuery.isFetching}
      mutations={GRAPHQL_DEPLOY_MUTATIONS}
      onRefreshDeployments={() => void deploymentsQuery.refetch()}
      orgHandle={orgHandle}
    />
  );
}
