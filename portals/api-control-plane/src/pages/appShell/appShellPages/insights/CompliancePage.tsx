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

import { ExternalToolPanel } from '@/components/common/ExternalToolPanel';
import { runtimeConfig } from '@/config/runtime';

const messages = defineMessages({
  action: {
    id: 'apiControlPlane.pages.appShell.appShellPages.insights.CompliancePage.action',
    defaultMessage: 'Open Moesif Insights',
    description:
      'Button that opens the Moesif analytics console in a new tab. Moesif is a product name — leave it untranslated.',
  },
  panelDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.insights.CompliancePage.panelDescription',
    defaultMessage:
      'Review governance rules, specification conformance, and policy violations from your Moesif analytics workspace.',
  },
  panelTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.insights.CompliancePage.panelTitle',
    defaultMessage: 'Your API compliance reporting lives in Moesif',
  },
  subHeader: {
    id: 'apiControlPlane.pages.appShell.appShellPages.insights.CompliancePage.subHeader',
    defaultMessage: 'Governance and compliance reporting.',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.insights.CompliancePage.title',
    defaultMessage: 'Compliance',
  },
});

export function CompliancePage() {
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
