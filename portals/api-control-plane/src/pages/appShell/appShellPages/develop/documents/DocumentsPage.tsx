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

import { defineMessages } from 'react-intl';

import { routes } from '@/routes/paths';
import { ScopeGate } from '@/scope/ScopeGate';
import { DevelopPageShell } from '../DevelopPageShell';
import { DocumentsPanel } from './DocumentsPanel';

const messages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.develop.DocumentsPage.title',
    defaultMessage: 'Documents',
  },
  subtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.develop.DocumentsPage.subtitle',
    defaultMessage: 'Documentation for {apiName}',
    description: 'Sub-header under the section name; {apiName} is the API display name.',
  },
});

export function DocumentsPage() {
  return (
    <ScopeGate
      prompt="Documents belong to a single API."
      requires="api"
      to={routes.apiDevelopDocuments}
    >
      {/* `DocumentsTab` takes no detail of its own; the shell is here for the
          heading, which still names the API being documented. */}
      <DevelopPageShell subtitle={messages.subtitle} title={messages.title}>
        {() => <DocumentsPanel />}
      </DevelopPageShell>
    </ScopeGate>
  );
}
