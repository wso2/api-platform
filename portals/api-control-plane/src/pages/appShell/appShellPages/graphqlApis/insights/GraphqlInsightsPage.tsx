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

import { PageTitle } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { useParams } from 'react-router-dom';

import { ComingSoon } from '@/components/ComingSoon';
import { ExternalToolPanel } from '@/components/common/ExternalToolPanel';
import { ErrorState } from '@/components/StateViews';
import { runtimeConfig } from '@/config/runtime';

const messages = defineMessages({
  action: {
    id: 'apiControlPlane.pages.appShell.appShellPages.insights.InsightsPage.action',
    defaultMessage: 'Open Moesif Insights',
    description: 'Button that opens the Moesif analytics console in a new tab. Moesif is a product name — leave it untranslated.',
  },
  apiNotFound: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.insights.GraphqlInsightsPage.apiNotFound',
    defaultMessage: 'GraphQL API not found',
  },
  cloudFeature: {
    id: 'appShell.insightsPage.feature',
    defaultMessage: 'API insights',
    description: 'Feature name shown on the Coming Soon placeholder when API-scoped Insights is not available yet in cloud.',
  },
  panelDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.insights.InsightsPage.panelDescription',
    defaultMessage:
      'Track usage trends, request activity, latency, and customer behavior from your Moesif analytics workspace.',
  },
  panelTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.insights.InsightsPage.panelTitle',
    defaultMessage: 'Your API insights live in Moesif',
  },
  subHeader: {
    id: 'apiControlPlane.pages.appShell.appShellPages.insights.InsightsPage.subHeader',
    defaultMessage: 'Usage analytics and traffic insights.',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.insights.InsightsPage.title',
    defaultMessage: 'Insights',
  },
});

/**
 * Fork of `insights/InsightsPage.tsx` for a GraphQL API. The gateway's
 * analytics pipeline already tags GraphQL requests distinctly (see
 * `policy-engine/internal/analytics/analytics.go`'s `graphqlAnalytics`
 * enrichment), so the same Moesif workspace already carries this API's data —
 * this page only needed its own route into that same content. No `ScopeGate`:
 * this route lives outside `ConsoleScopeProvider`'s REST-only api-scope
 * matching (see `graphqlApiPath`), so it guards on its own route param
 * instead, matching every other GraphQL page.
 */
export function GraphqlInsightsPage() {
  const intl = useIntl();
  const { graphqlApiHandler } = useParams();

  if (!graphqlApiHandler) {
    return <ErrorState title={intl.formatMessage(messages.apiNotFound)} />;
  }

  // Cloud ships org/project Moesif embeds via the insights plugin; API-scoped
  // analytics is not ready yet there either — mirrors InsightsPage's own gate.
  if (runtimeConfig.cloudProxyEnabled) {
    return <ComingSoon feature={<FormattedMessage {...messages.cloudFeature} />} />;
  }

  return (
    <>
      <PageTitle>
        <PageTitle.Header>
          <FormattedMessage {...messages.title} />
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <FormattedMessage {...messages.subHeader} />
        </PageTitle.SubHeader>
      </PageTitle>
      <ExternalToolPanel
        actionLabel={<FormattedMessage {...messages.action} />}
        description={<FormattedMessage {...messages.panelDescription} />}
        href={runtimeConfig.moesifWebUrl}
        title={<FormattedMessage {...messages.panelTitle} />}
      />
    </>
  );
}
