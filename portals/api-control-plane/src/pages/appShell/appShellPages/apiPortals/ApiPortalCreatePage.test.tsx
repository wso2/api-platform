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
import { accepts, anApiPortal, recorder, resource, type Recorder } from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor } from '@/test/utils';
import { ApiPortalCreatePage } from './ApiPortalCreatePage';
import { ApiPortalDetailPage } from './ApiPortalDetailPage';
import { sharedKeyHash } from './utils/apiPortalSetup';

const ORG = 'api-platform-demo';

let requests: Recorder;

function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <Routes>
        <Route
          element={<ApiPortalCreatePage />}
          path="/organizations/:orgHandle/api-portals/new"
        />
        <Route
          element={<ApiPortalDetailPage />}
          path="/organizations/:orgHandle/api-portals/:apiPortalId"
        />
      </Routes>
    </ApiScopeProvider>,
    { route: `/organizations/${ORG}/api-portals/new`, scope: makeConsoleScope() },
  );
}

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('ApiPortalCreatePage', () => {
  it('reveals every unmet rule on submit and sends nothing', async () => {
    server.use(accepts('post', '/api-portals', anApiPortal(), { record: requests }));

    const { user } = renderPage();

    await user.click(screen.getByRole('button', { name: 'Register portal' }));

    expect(await screen.findByText('Enter a portal name.')).toBeInTheDocument();
    expect(screen.getByText('Enter a handle.')).toBeInTheDocument();
    expect(screen.getByText('Enter the portal URL.')).toBeInTheDocument();
    expect(requests.count()).toBe(0);
  });

  it('rejects a URL that is not https', async () => {
    server.use(accepts('post', '/api-portals', anApiPortal(), { record: requests }));

    const { user } = renderPage();

    await user.type(screen.getByLabelText(/Name/), 'Partner Portal');
    await user.type(screen.getByLabelText(/URL/), 'http://portal.example.com');
    await user.click(screen.getByRole('button', { name: 'Register portal' }));

    expect(
      await screen.findByText('Enter a full https URL, for example https://portal.example.com.'),
    ).toBeInTheDocument();
    expect(requests.count()).toBe(0);
  });

  it('registers with a generated key and shows only its hash', async () => {
    server.use(
      accepts('post', '/api-portals', anApiPortal(), { record: requests }),
      resource('/api-portals/partner-portal', anApiPortal()),
    );

    const { user } = renderPage();

    await user.type(screen.getByLabelText(/Name/), 'Partner Portal');
    expect(screen.getByLabelText(/Handle/)).toHaveValue('partner-portal');
    await user.type(screen.getByLabelText(/URL/), 'https://portal.example.com');
    await user.click(screen.getByRole('button', { name: 'Register portal' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    const body = JSON.parse(requests.last()!.body);
    expect(body).toEqual({
      handle: 'partner-portal',
      name: 'Partner Portal',
      sharedKey: expect.stringMatching(/^[0-9a-f]{64}$/),
      url: 'https://portal.example.com',
    });

    // The detail page receives the hash, never the key.
    const hash = await sharedKeyHash(body.sharedKey);
    expect(await screen.findByText(/You registered Partner Portal/)).toBeInTheDocument();
    expect(screen.getByText(new RegExp(hash))).toBeInTheDocument();
    expect(screen.queryByText(new RegExp(body.sharedKey))).not.toBeInTheDocument();
  });
});
