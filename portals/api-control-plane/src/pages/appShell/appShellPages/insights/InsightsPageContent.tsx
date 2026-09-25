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
import { defineMessages, FormattedMessage } from 'react-intl';

import { ComingSoon } from '@/components/ComingSoon';
import { ExternalToolPanel } from '@/components/common/ExternalToolPanel';
import { runtimeConfig } from '@/config/runtime';

const messages = defineMessages({
  action: {
    id: 'apiControlPlane.pages.appShell.appShellPages.insights.InsightsPage.action',
    defaultMessage: 'Open Moesif Insights',
    description:
      'Button that opens the Moesif analytics console in a new tab. Moesif is a product name — leave it untranslated.',
  },
  cloudFeature: {
    id: 'appShell.insightsPage.feature',
    defaultMessage: 'API insights',
    description:
      'Feature name shown on the Coming Soon placeholder when API-scoped Insights is not available yet in cloud.',
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
 * Body of the Insights page — identical for REST and GraphQL APIs (a generic
 * Moesif redirect panel that never reads the API's own data), so it is shared
 * directly rather than forked. Only the outer scope guard differs per kind:
 * `InsightsPage` wraps this in `ScopeGate`, `GraphqlInsightsPage` in
 * `GraphqlApiPageGuard`.
 */
export function InsightsPageContent() {
  // Cloud ships org/project Moesif embeds via the insights plugin; API-scoped
  // analytics is not ready yet, so show Coming Soon when the cloud proxy is on
  // (same signal that gates those sidebar extensions).
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
