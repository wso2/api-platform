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

import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { useParams } from 'react-router-dom';

import { useGraphQLApi } from '@/api/resources/graphqlApis';
import { ComingSoon } from '@/components/ComingSoon';
import { ErrorState, LoadingState } from '@/components/StateViews';

const messages = defineMessages({
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.develop.GraphqlDevelopDocumentsPage.loading',
    defaultMessage: 'Loading API',
  },
  apiNotFound: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.develop.GraphqlDevelopDocumentsPage.apiNotFound',
    defaultMessage: 'GraphQL API not found',
  },
  feature: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.develop.GraphqlDevelopDocumentsPage.feature',
    defaultMessage: 'Documents for this API',
  },
  detail: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.develop.GraphqlDevelopDocumentsPage.detail',
    defaultMessage:
      'You will be able to publish guides, references, and release notes alongside the API so consumers can read them in the Developer Portal.',
  },
});

/**
 * Fork of `develop/documents/DocumentsPage.tsx` for a GraphQL API. `Documents`
 * has no backend concept for either API kind today — REST's own tab is the
 * same `ComingSoon` placeholder this mirrors verbatim. No `ScopeGate`: this
 * route lives outside `ConsoleScopeProvider`'s REST-only api-scope matching
 * (see `graphqlApiPath`), so it guards on its own route param instead.
 */
export function GraphqlDevelopDocumentsPage() {
  const intl = useIntl();
  const { graphqlApiHandler } = useParams();
  const apiQuery = useGraphQLApi(graphqlApiHandler);

  if (!graphqlApiHandler || apiQuery.error) {
    return <ErrorState title={intl.formatMessage(messages.apiNotFound)} />;
  }
  if (apiQuery.isPending) return <LoadingState label={intl.formatMessage(messages.loading)} />;
  if (!apiQuery.data) return <ErrorState title={intl.formatMessage(messages.apiNotFound)} />;

  return (
    <ComingSoon
      detail={<FormattedMessage {...messages.detail} />}
      feature={<FormattedMessage {...messages.feature} />}
    />
  );
}
