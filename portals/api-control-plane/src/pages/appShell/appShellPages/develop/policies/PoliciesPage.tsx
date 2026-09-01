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

import { routes } from '@/routes/paths';
import { ScopeGate } from '@/scope/ScopeGate';
import { DevelopPageShell } from '../DevelopPageShell';
import { PolicyPanel } from './PolicyPanel';

const messages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.develop.PoliciesPage.title',
    defaultMessage: 'Policies',
  },
  subtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.develop.PoliciesPage.subtitle',
    defaultMessage: 'Policies attached to {apiName}',
    description: 'Sub-header under the section name; {apiName} is the API display name.',
  },
  scopePrompt: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PoliciesPage.scopePrompt',
    defaultMessage: 'Policies are attached to a single API.',
    description: 'Explains why an API must be picked before this page can render.',
  },
});

export function PoliciesPage() {
  const intl = useIntl();

  return (
    <ScopeGate
      prompt={intl.formatMessage(messages.scopePrompt)}
      requires="api"
      to={routes.apiDevelopPolicies}
    >
      <DevelopPageShell subtitle={messages.subtitle} title={messages.title}>
        {(api) => <PolicyPanel api={api} />}
      </DevelopPageShell>
    </ScopeGate>
  );
}
