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

import { Box } from '@wso2/oxygen-ui';
import { defineMessages, useIntl } from 'react-intl';

import { useRestApi } from '@/api/resources/restApis';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { routes } from '@/routes/paths';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { ScopeGate } from '@/scope/ScopeGate';
import { PolicyPanel } from './PolicyPanel';

const messages = defineMessages({
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PoliciesPage.loading',
    defaultMessage: 'Loading API',
  },
  notFound: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PoliciesPage.notFound',
    defaultMessage: 'API not found',
  },
  loadError: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PoliciesPage.loadError',
    defaultMessage: 'Unable to load the API.',
  },
  scopePrompt: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PoliciesPage.scopePrompt',
    defaultMessage: 'Policies are attached to a single API.',
    description: 'Explains why an API must be picked before this page can render.',
  },
});

export function PoliciesPage() {
  const intl = useIntl();
  const { params } = useConsoleScope();
  const apiQuery = useRestApi(params.apiHandler);

  const content = apiQuery.isPending ? (
    <LoadingState label={intl.formatMessage(messages.loading)} />
  ) : apiQuery.error ? (
    <ErrorState title={intl.formatMessage(messages.loadError)} />
  ) : !apiQuery.data ? (
    <ErrorState title={intl.formatMessage(messages.notFound)} />
  ) : (
    <Box sx={{ mt: 2 }}>
      <PolicyPanel api={apiQuery.data} />
    </Box>
  );

  return (
    <ScopeGate
      prompt={intl.formatMessage(messages.scopePrompt)}
      requires="api"
      to={routes.apiDevelopPolicies}
    >
      {content}
    </ScopeGate>
  );
}
