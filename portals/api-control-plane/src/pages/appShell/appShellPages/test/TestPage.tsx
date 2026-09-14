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

import { Card, CardContent, CodeBlock, PageTitle } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useRestApi } from '@/api/resources/restApis';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { routes } from '@/routes/paths';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { ScopeGate } from '@/scope/ScopeGate';

const messages = defineMessages({
  apiNotFound: {
    id: 'apiControlPlane.pages.appShell.appShellPages.test.TestPage.apiNotFound',
    defaultMessage: 'API not found',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.test.TestPage.loading',
    defaultMessage: 'Loading test console',
    description: 'Shown while the API the curl command is built from is being fetched.',
  },
  scopePrompt: {
    id: 'apiControlPlane.pages.appShell.appShellPages.test.TestPage.scopePrompt',
    defaultMessage: 'The curl console runs against a single API.',
    description: 'Explains why an API must be picked before this page can render.',
  },
  subtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.test.TestPage.subtitle',
    defaultMessage: 'Use the following curl command to test the API.',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.test.TestPage.title',
    defaultMessage: 'Test {apiName}',
    description:
      'Page heading. {apiName} is the API display name, user-supplied; do not translate it.',
  },
});

/** Builds a curl command for testing an API. Uses $API_BASE_URL as unresolved shell variable. */
const curlCommand = (context: string) =>
  `curl -X GET "$API_BASE_URL${context}" \\\n  -H "Authorization: Bearer <token>"`;

export function TestPage() {
  const intl = useIntl();

  return (
    <ScopeGate
      prompt={intl.formatMessage(messages.scopePrompt)}
      requires="api"
      to={routes.apiTestCurl}
    >
      <Test />
    </ScopeGate>
  );
}

function Test() {
  const intl = useIntl();
  const { params } = useConsoleScope();
  const apiQuery = useRestApi(params.apiHandler);

  if (apiQuery.isPending) return <LoadingState label={intl.formatMessage(messages.loading)} />;
  if (apiQuery.error || !apiQuery.data) {
    return <ErrorState title={intl.formatMessage(messages.apiNotFound)} />;
  }

  const api = apiQuery.data;

  return (
    <>
      <PageTitle>
        <PageTitle.Header>
          <FormattedMessage {...messages.title} values={{ apiName: api.displayName }} />
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <FormattedMessage {...messages.subtitle} />
        </PageTitle.SubHeader>
      </PageTitle>
      <Card variant="outlined">
        <CardContent>
          <CodeBlock code={curlCommand(api.context || '/')} language="bash" />
        </CardContent>
      </Card>
    </>
  );
}
