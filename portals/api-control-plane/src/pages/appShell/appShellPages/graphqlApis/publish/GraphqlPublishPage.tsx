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
import { useGraphQLApi } from '@/api/resources/graphqlApis';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { ApiPortalPublicationsList } from '../../portals/ApiPortalPublicationsList';
import { graphqlPublishTarget } from '../../portals/publishTarget';

const messages = defineMessages({
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.ApiEditPage.loading',
    defaultMessage: 'Loading API',
    description: 'Shown while the API being edited is fetched.',
  },
  apiNotFound: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.publish.GraphqlPublishPage.apiNotFound',
    defaultMessage: 'GraphQL API not found',
  },
});

/**
 * A GraphQL API's Publish page: REST's Portals list — every API Portal, with
 * this API's publication status on each — over the GraphQL API's own
 * publications. No `ScopeGate`: this route lives outside
 * `ConsoleScopeProvider`'s REST-only api-scope matching (see
 * `graphqlApiPath`), so it guards on its own route param instead.
 */
export function GraphqlPublishPage() {
  return (
    <AppPage>
      <GraphqlPublishPageContent />
    </AppPage>
  );
}

function GraphqlPublishPageContent() {
  const intl = useIntl();
  const { graphqlApiHandler = '', orgHandle = '', projectHandler = '' } = useParams();
  const apiQuery = useGraphQLApi(graphqlApiHandler || undefined);
  const target = useMemo(
    () => graphqlPublishTarget(orgHandle, projectHandler, graphqlApiHandler),
    [graphqlApiHandler, orgHandle, projectHandler],
  );

  if (!graphqlApiHandler || apiQuery.error) {
    return <ErrorState title={intl.formatMessage(messages.apiNotFound)} />;
  }
  if (apiQuery.isPending) return <LoadingState label={intl.formatMessage(messages.loading)} />;
  if (!apiQuery.data) return <ErrorState title={intl.formatMessage(messages.apiNotFound)} />;

  return <ApiPortalPublicationsList apiName={apiQuery.data.displayName} target={target} />;
}
