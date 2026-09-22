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

import { ComingSoon } from '@/components/ComingSoon';
import { ErrorState } from '@/components/StateViews';

const messages = defineMessages({
  apiNotFound: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.insights.GraphqlCompliancePage.apiNotFound',
    defaultMessage: 'GraphQL API not found',
  },
  feature: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.insights.GraphqlCompliancePage.feature',
    defaultMessage: 'Compliance reporting for this API',
  },
});

/**
 * Fork of `insights/CompliancePage.tsx` for a GraphQL API. No `ScopeGate`:
 * this route lives outside `ConsoleScopeProvider`'s REST-only api-scope
 * matching (see `graphqlApiPath`), so it guards on its own route param
 * instead, matching every other GraphQL page.
 */
export function GraphqlCompliancePage() {
  const intl = useIntl();
  const { graphqlApiHandler } = useParams();

  if (!graphqlApiHandler) {
    return <ErrorState title={intl.formatMessage(messages.apiNotFound)} />;
  }

  return <ComingSoon feature={<FormattedMessage {...messages.feature} />} />;
}
