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

const messages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PoliciesPageTitle.title',
    defaultMessage: 'Policies',
    description: 'Heading of the Policies page of an API. Noun.',
  },
  subtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PoliciesPageTitle.subtitle',
    defaultMessage:
      'Policies the gateway applies to this API’s traffic, such as authentication, rate limiting and header changes.',
    description: 'Description under the Policies page heading.',
  },
});

/**
 * Heading and description of the Policies page, matching the Documents page.
 * Shared by the REST and GraphQL policy panels so both pages read the same.
 */
export function PoliciesPageTitle() {
  return (
    <PageTitle>
      <PageTitle.Header>
        <FormattedMessage {...messages.title} />
      </PageTitle.Header>
      <PageTitle.SubHeader>
        <FormattedMessage {...messages.subtitle} />
      </PageTitle.SubHeader>
    </PageTitle>
  );
}
