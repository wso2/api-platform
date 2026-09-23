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

import { useGraphQLApi } from '@/api/resources/graphqlApis';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { GraphqlPolicyPanel } from './GraphqlPolicyPanel';

const messages = defineMessages({
  loading: {
    id: 'gateways.detail.Policies.loading',
    defaultMessage: 'Loading policies',
  },
  apiNotFound: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.develop.GraphqlDevelopPoliciesPage.apiNotFound',
    defaultMessage: 'GraphQL API not found',
  },
});

/**
 * Fork of `develop/policies/PoliciesPage.tsx` for a GraphQL API. No
 * `ScopeGate`: this route lives outside `ConsoleScopeProvider`'s REST-only
 * api-scope matching (see `graphqlApiPath`), so it guards on its own route
 * param instead.
 */
export function GraphqlDevelopPoliciesPage() {
  const intl = useIntl();
  const { graphqlApiHandler } = useParams();
  const apiQuery = useGraphQLApi(graphqlApiHandler);

  if (!graphqlApiHandler || apiQuery.error) {
    return <ErrorState title={intl.formatMessage(messages.apiNotFound)} />;
  }
  if (apiQuery.isPending) return <LoadingState label={intl.formatMessage(messages.loading)} />;
  if (!apiQuery.data) return <ErrorState title={intl.formatMessage(messages.apiNotFound)} />;

  const api = apiQuery.data;

  return <GraphqlPolicyPanel api={api} />;
}
