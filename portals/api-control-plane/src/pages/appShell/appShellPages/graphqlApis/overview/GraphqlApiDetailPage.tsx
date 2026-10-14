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

import { useMemo } from 'react';
import { defineMessages, useIntl } from 'react-intl';
import { useParams } from 'react-router-dom';

import { AppPage } from '@/components/AppPage';
import { GRAPHQL_API_TYPE } from '@/api/resources/apiPublications';
import type { Gateway } from '@/api/resources/gateways';
import { useGraphQLApi, useGraphQLApiSdl } from '@/api/resources/graphqlApis';
import { useGraphQLApiGateways } from '@/api/resources/graphqlApis/apiGateways';
import { useDeployments } from '@/api/resources/graphqlApis/deployments';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { useCan } from '@/permissions/useCan';
import { routes } from '@/routes/paths';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { ApiOverviewHeader } from '../../apis/overview/ApiOverviewHeader';
import { deployedGateways as deployedGatewaysOf } from '../../test/utils/deployedGateways';
import { GraphqlOverviewTab } from './GraphqlOverviewTab';
import { GraphqlProgressBanner } from './GraphqlProgressBanner';

const messages = defineMessages({
  notFound: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.overview.GraphqlApiDetailPage.notFound',
    defaultMessage: 'GraphQL API not found',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.ApiEditPage.loading',
    defaultMessage: 'Loading API',
    description: 'Shown while the API being edited is fetched.',
  },
});

/**
 * Overview page for a GraphQL API. No `ScopeGate`: this route lives outside
 * `ConsoleScopeProvider`'s REST-only api-scope matching (see `graphqlApiPath`),
 * so it guards on its own route param instead of `isApiScope`. The
 * organization/project tier still resolves generically through
 * `useConsoleScope`/`useApiScope` — only the API tier is read from
 * `useParams` directly.
 */
export function GraphqlApiDetailPage() {
  return (
    <AppPage>
      <GraphqlApiDetailPageContent />
    </AppPage>
  );
}

function GraphqlApiDetailPageContent() {
  const { params } = useConsoleScope();
  const { graphqlApiHandler } = useParams();
  const intl = useIntl();
  const canEdit = useCan('UpdateGraphQLAPI');

  const apiQuery = useGraphQLApi(graphqlApiHandler);
  const sdlQuery = useGraphQLApiSdl(graphqlApiHandler);
  // The API's own gateway associations, as REST's overview reads them —
  // scoped to the API, so this needs no organization-wide gateway access.
  const gatewaysQuery = useGraphQLApiGateways(graphqlApiHandler);
  const deploymentsQuery = useDeployments(graphqlApiHandler);

  // Deployed here, most recently deployed first — the same rule the Test
  // page applies, so the two never disagree on where the API runs.
  const deployedGateways = useMemo(
    (): Gateway[] =>
      deployedGatewaysOf(gatewaysQuery.data?.list ?? [], deploymentsQuery.data?.list ?? []),
    [gatewaysQuery.data, deploymentsQuery.data],
  );

  if (!graphqlApiHandler || apiQuery.error) {
    return <ErrorState title={intl.formatMessage(messages.notFound)} />;
  }
  if (apiQuery.isPending) return <LoadingState label={intl.formatMessage(messages.loading)} />;
  if (!apiQuery.data) return <ErrorState title={intl.formatMessage(messages.notFound)} />;

  const api = apiQuery.data;
  const orgHandle = params.orgHandle ?? '';
  const projectHandler = params.projectHandler ?? '';
  const deployPath = routes.graphqlApiDeploy(orgHandle, projectHandler, graphqlApiHandler);
  const editPath = routes.graphqlApiEdit(orgHandle, projectHandler, graphqlApiHandler);

  return (
    <>
      <ApiOverviewHeader
        api={api}
        apiId={graphqlApiHandler}
        apiType={GRAPHQL_API_TYPE}
        banner={<GraphqlProgressBanner deployed={deployedGateways.length > 0} />}
        canEdit={canEdit}
        deployOperation="DeployGraphQLAPI"
        deployPath={deployPath}
        editPath={editPath}
      />

      <GraphqlOverviewTab
        api={api}
        deployedGateways={deployedGateways}
        deployments={deploymentsQuery.data?.list ?? []}
        sdl={sdlQuery.data}
      />
    </>
  );
}
