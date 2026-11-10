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

import { Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { routes } from '@/routes/paths';
import { resetTrackedFeatures, trackedFeatures } from '@/test/featureUsage';
import { makeConsoleScope } from '@/test/mockScope';
import {
  anOrganization,
  aProject,
  collection,
  noContent,
  recorder,
  resource,
  type Recorder,
} from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor, within } from '@/test/utils';
import { OrganizationHomePage } from './OrganizationHomePage';

vi.mock('@/featureUsage/moesif');
// The greeting reads the host port, which a page test doesn't mount; it isn't what is measured here.
vi.mock('@/hooks/useGreetingName', () => ({ useGreetingName: (fallback: string) => fallback }));

const ORG = 'acme-org';

let deletes: Recorder;

/** The page's hooks read `ApiScopeContext`, so the scope provider is mounted. */
function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <Routes>
        <Route path={routes.organizationHome()} element={<OrganizationHomePage />} />
      </Routes>
    </ApiScopeProvider>,
    {
      route: routes.organizationHome(ORG),
      scope: makeConsoleScope({
        params: { orgHandle: ORG, projectHandler: undefined },
        project: undefined,
        projects: [aProject({ id: 'retail', displayName: 'Retail APIs' })],
      }),
    },
  );
}

beforeEach(() => {
  deletes = recorder();
  resetHttpClient();
  resetTrackedFeatures();
  server.use(
    resource('/organizations/:organizationId', anOrganization({ id: ORG })),
    collection('/gateways', []),
    collection('/api-portals', []),
    collection('/rest-apis', []),
    collection('/graphql-apis', []),
    noContent('delete', '/projects/:projectId', { record: deletes }),
  );
});

describe('OrganizationHomePage feature usage', () => {
  it('reports a project deletion when the type-to-confirm dialog is confirmed', async () => {
    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Delete Retail APIs' }));
    const dialog = screen.getByRole('dialog');
    await user.type(within(dialog).getByRole('textbox'), 'Retail APIs');
    expect(trackedFeatures()).toEqual([]);

    await user.click(within(dialog).getByRole('button', { name: 'Delete' }));

    await waitFor(() => expect(deletes.count()).toBe(1));
    expect(trackedFeatures()).toEqual([['apip.admin.project-delete-click', {}]]);
  });

  it('reports nothing when the deletion is cancelled', async () => {
    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Delete Retail APIs' }));
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Cancel' }));

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(deletes.count()).toBe(0);
    expect(trackedFeatures()).toEqual([]);
  });
});
