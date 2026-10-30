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
import {
  accepts,
  anApiPortal,
  noContent,
  recorder,
  resource,
  type Recorder,
} from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor, within } from '@/test/utils';
import { ApiPortalDetailPage } from './ApiPortalDetailPage';
import { HASH_PLACEHOLDER, sharedKeyHash } from './utils/apiPortalSetup';

const ORG = 'api-platform-demo';
const PORTAL_ID = 'partner-portal';
const PORTAL_PATH = `/api-portals/${PORTAL_ID}`;

let updates: Recorder;
let deletes: Recorder;

function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <Routes>
        <Route
          element={<ApiPortalDetailPage />}
          path="/organizations/:orgHandle/api-portals/:apiPortalId"
        />
        <Route element={<div>portal list</div>} path="/organizations/:orgHandle/api-portals" />
      </Routes>
    </ApiScopeProvider>,
    { route: `/organizations/${ORG}/api-portals/${PORTAL_ID}`, scope: makeConsoleScope() },
  );
}

beforeEach(() => {
  updates = recorder();
  deletes = recorder();
  resetHttpClient();
  server.use(resource(PORTAL_PATH, anApiPortal({ description: 'For partners' })));
});

describe('ApiPortalDetailPage', () => {
  it('shows the portal and a placeholder until a key is set', async () => {
    renderPage();

    expect(await screen.findByRole('heading', { name: 'Partner Portal' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'https://portal.example.com' })).toBeInTheDocument();
    expect(screen.getByText('For partners')).toBeInTheDocument();
    expect(screen.getByText(new RegExp(HASH_PLACEHOLDER))).toBeInTheDocument();
  });

  it('rotates the shared key after confirmation and shows the new hash', async () => {
    server.use(accepts('put', PORTAL_PATH, anApiPortal(), { record: updates }));

    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Generate new key' }));
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Generate new key' }));

    await waitFor(() => expect(updates.count()).toBe(1));
    const { sharedKey } = JSON.parse(updates.last()!.body);
    expect(sharedKey).toMatch(/^[0-9a-f]{64}$/);

    const hash = await sharedKeyHash(sharedKey);
    expect(await screen.findByText(new RegExp(hash))).toBeInTheDocument();
  });

  it('saves edits to the name and URL', async () => {
    server.use(
      accepts('put', PORTAL_PATH, anApiPortal({ name: 'Partners' }), { record: updates }),
    );

    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Edit' }));
    const dialog = await screen.findByRole('dialog');
    const name = within(dialog).getByLabelText(/Name/);
    await user.clear(name);
    await user.type(name, 'Partners');
    await user.click(within(dialog).getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(updates.count()).toBe(1));
    expect(JSON.parse(updates.last()!.body)).toEqual({
      description: 'For partners',
      name: 'Partners',
      url: 'https://portal.example.com',
    });
  });

  it('deletes the portal and returns to the list', async () => {
    server.use(noContent('delete', PORTAL_PATH, { record: deletes }));

    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Delete' }));
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Delete' }));

    await waitFor(() => expect(deletes.count()).toBe(1));
    expect(await screen.findByText('portal list')).toBeInTheDocument();
  });
});
