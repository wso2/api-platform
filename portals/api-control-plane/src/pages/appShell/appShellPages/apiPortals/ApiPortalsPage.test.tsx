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
import { beforeEach, describe, expect, it } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { anApiPortal, collection } from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen } from '@/test/utils';
import { ApiPortalsPage } from './ApiPortalsPage';

const ORG = 'api-platform-demo';

function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <Routes>
        <Route element={<ApiPortalsPage />} path="/organizations/:orgHandle/api-portals" />
        <Route element={<div>register page</div>} path="/organizations/:orgHandle/api-portals/new" />
        <Route
          element={<div>portal detail</div>}
          path="/organizations/:orgHandle/api-portals/:apiPortalId"
        />
      </Routes>
    </ApiScopeProvider>,
    { route: `/organizations/${ORG}/api-portals`, scope: makeConsoleScope() },
  );
}

beforeEach(() => {
  resetHttpClient();
});

describe('ApiPortalsPage', () => {
  it('offers registration to an organization with no portals', async () => {
    server.use(collection('/api-portals', []));

    const { user } = renderPage();

    expect(await screen.findByText('Register your first API Portal')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Register portal' }));
    expect(await screen.findByText('register page')).toBeInTheDocument();
  });

  it('lists portals, filters by search and opens one', async () => {
    server.use(
      collection('/api-portals', [
        anApiPortal(),
        anApiPortal({ id: 'internal', name: 'Internal Portal', url: 'https://internal.test' }),
      ]),
    );

    const { user } = renderPage();

    expect(await screen.findByText('Partner Portal')).toBeInTheDocument();
    expect(screen.getByText('2 portals')).toBeInTheDocument();

    await user.type(screen.getByPlaceholderText('Search portals'), 'internal');
    expect(screen.queryByText('Partner Portal')).not.toBeInTheDocument();
    expect(screen.getByText('1 portal')).toBeInTheDocument();

    await user.click(screen.getByText('Internal Portal'));
    expect(await screen.findByText('portal detail')).toBeInTheDocument();
  });
});
