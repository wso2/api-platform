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
import { GRAPHQL_API_TYPE } from '@/api/resources/apiPublications';
import { useGraphQLApi } from '@/api/resources/graphqlApis';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { DocumentsPanel } from '../../develop/documents/DocumentsPanel';

const messages = defineMessages({
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.ApiEditPage.loading',
    defaultMessage: 'Loading API',
    description: 'Shown while the API being edited is fetched.',
  },
  apiNotFound: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.develop.GraphqlDevelopDocumentsPage.apiNotFound',
    defaultMessage: 'GraphQL API not found',
  },
});

/**
 * Develop › Documents for a GraphQL API. Reuses REST's `DocumentsPanel`, which
 * is keyed by `{apiType}`, passing `graphql-api` and this route's own handle —
 * this route sits outside `ConsoleScopeProvider`'s REST-only api-scope
 * matching (see `graphqlApiPath`), so there is no API in scope to fall back
 * on. No `ScopeGate` for the same reason: it guards on its own route param.
 */
export function GraphqlDevelopDocumentsPage() {
  return (
    <AppPage>
      <GraphqlDevelopDocumentsPageContent />
    </AppPage>
  );
}

function GraphqlDevelopDocumentsPageContent() {
  const intl = useIntl();
  const { graphqlApiHandler } = useParams();
  const apiQuery = useGraphQLApi(graphqlApiHandler);

  if (!graphqlApiHandler || apiQuery.error) {
    return <ErrorState title={intl.formatMessage(messages.apiNotFound)} />;
  }
  if (apiQuery.isPending) return <LoadingState label={intl.formatMessage(messages.loading)} />;
  if (!apiQuery.data) return <ErrorState title={intl.formatMessage(messages.apiNotFound)} />;

  return <DocumentsPanel apiHandle={graphqlApiHandler} apiType={GRAPHQL_API_TYPE} />;
}
