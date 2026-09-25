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

import type { ReactNode } from 'react';
import { defineMessages, useIntl } from 'react-intl';
import { useParams } from 'react-router-dom';

import { ErrorState } from '@/components/StateViews';

const messages = defineMessages({
  apiNotFound: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.components.GraphqlApiPageGuard.apiNotFound',
    defaultMessage: 'GraphQL API not found',
  },
});

/**
 * Minimal replacement for `ScopeGate` on GraphQL API pages that need only the
 * route's `graphqlApiHandler` param, not the API record itself — these routes
 * live outside `ConsoleScopeProvider`'s REST-only api-scope matching (see
 * `graphqlApiPath`), so each page guards on its own route param instead.
 *
 * Pages that also need the API record (e.g. to show its display name) keep
 * their own `useGraphQLApi` call plus loading/error handling; this only
 * covers the bare param-presence check that was duplicated across the
 * simpler pages (Insights, Observability, Compliance).
 */
export function GraphqlApiPageGuard({
  children,
}: {
  children: (graphqlApiHandler: string) => ReactNode;
}) {
  const intl = useIntl();
  const { graphqlApiHandler } = useParams();

  if (!graphqlApiHandler) {
    return <ErrorState title={intl.formatMessage(messages.apiNotFound)} />;
  }

  return <>{children(graphqlApiHandler)}</>;
}
