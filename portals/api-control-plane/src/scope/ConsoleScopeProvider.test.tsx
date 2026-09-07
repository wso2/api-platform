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

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { AppRoutes } from '../routes/AppRoutes';
import {
  anOrganization,
  aProject,
  collection,
  recorder,
  resource,
  type Recorder,
} from '../test/msw';
import { makeAuthState } from '../test/mockAuthState';
import { server } from '../test/server';
import { renderWithProviders, screen } from '../test/utils';

// This console's session is scoped to exactly one organization for its whole
// lifetime (the BFF forwards one bearer token per session — see
// `ConsoleScopeProvider`'s module comment). These tests guard the boundary
// that keeps a route naming a *different* organization from ever reaching the
// app shell or platform-api with that org's handle.
describe('ConsoleScopeProvider — organization access boundary', () => {
  const org = anOrganization({ id: 'lasanthas', displayName: 'lasanthas' });
  let projectRequests: Recorder;

  beforeEach(() => {
    vi.stubEnv('VITE_USE_MOCK_API', 'true');
    projectRequests = recorder();
    server.use(
      // Organizations are never scope-gated (see `useOrganizations`), so this
      // request is expected to fire regardless of the org-access outcome —
      // only `projectRequests` below is the one that must stay silent.
      collection('/organizations', [org]),
      resource('/organizations/:organizationId', org),
      collection('/projects', [aProject({ id: 'default' })], {
        record: projectRequests,
      }),
      // Org-home also renders gateway count and per-project API counts.
      collection('/gateways', []),
      collection('/rest-apis', [])
    );
  });
  afterEach(() => vi.unstubAllEnvs());

  const authStateForOwnOrg = () =>
    makeAuthState({
      user: {
        name: 'Test User',
        email: 'test.user@example.com',
        org: { id: 'lasanthas', name: 'lasanthas', handle: 'lasanthas' },
      },
    });

  it('renders the app shell when the route names the session\'s own organization', async () => {
    renderWithProviders(<AppRoutes />, {
      route: '/organizations/lasanthas/home',
      authState: authStateForOwnOrg(),
    });

    expect(await screen.findByText(/WSO2 LLC/)).toBeInTheDocument();
  });

  it('denies access instead of showing the shell when the route names a different organization', async () => {
    renderWithProviders(<AppRoutes />, {
      route: '/organizations/apip-anusha/home',
      authState: authStateForOwnOrg(),
    });

    expect(
      await screen.findByText(/You do not have access to this organization/)
    ).toBeInTheDocument();

    // The app shell (sidebar/header) must not have mounted at all — not even
    // to render "apip-anusha" as a label.
    expect(screen.queryByText('apip-anusha')).not.toBeInTheDocument();

    // No project request may have gone out labelled for the mismatched org.
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(projectRequests.count()).toBe(0);
  });

  it('offers a way back to the signed-in user\'s own organization', async () => {
    renderWithProviders(<AppRoutes />, {
      route: '/organizations/apip-anusha/home',
      authState: authStateForOwnOrg(),
    });

    const link = await screen.findByRole('button', { name: /go to my organization/i });
    expect(link).toBeInTheDocument();
  });

  it('does not gate a session with no organization claim (basic/file-based auth)', async () => {
    renderWithProviders(<AppRoutes />, {
      route: '/organizations/lasanthas/home',
      authState: makeAuthState(),
    });

    expect(await screen.findByText(/WSO2 LLC/)).toBeInTheDocument();
    expect(
      screen.queryByText(/You do not have access to this organization/)
    ).not.toBeInTheDocument();
  });
});
