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

import { AppRoutes } from './AppRoutes';
import { anOrganization, aProject, collection, resource } from '../test/msw';
import { authStatePresets } from '../test/mockAuthState';
import { server } from '../test/server';
import { renderWithProviders, screen } from '../test/utils';

const disabled = vi.hoisted(() => ({ names: [] as string[] }));

vi.mock('../config/features', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../config/features')>()),
  isFeatureDisabled: (name: string) => disabled.names.includes(name),
}));

/**
 * Allows time for lazy route imports and rendering under a loaded test suite.
 */
const LAZY_ROUTE_TIMEOUT = { timeout: 5000 };

// Covers the org-level Subscription Plans and Meters pages and the sidebar's
// Manage menu, which replaced the Settings link.
describe('Org-level Manage pages', () => {
  // The scope hooks always go to the real transport — `VITE_USE_MOCK_API` only
  // governs the legacy client — so the endpoints `ConsoleScopeProvider` resolves
  // the org and project from are stubbed at the network layer.
  const org = anOrganization({
    id: 'api-platform-demo',
    displayName: 'API Platform Demo',
  });
  const project = aProject({ id: 'retail-apis', displayName: 'Retail APIs' });

  beforeEach(() => {
    vi.stubEnv('VITE_USE_MOCK_API', 'true');
    server.use(
      collection('/organizations', [org]),
      resource('/organizations/:organizationId', org),
      collection('/projects', [project]),
      resource('/projects/:projectId', project),
      collection('/rest-apis', []),
      collection('/graphql-apis', []),
      collection('/subscription-plans', []),
      collection('/meters', []),
    );
  });
  afterEach(() => {
    vi.unstubAllEnvs();
    disabled.names = [];
  });

  // `disableFeatures` hides a page from the sidebar and answers its URL as not found.
  it.each([
    ['subscription-plans', 'Subscription Plans'],
    ['meters', 'Meters'],
  ])('answers not found for %s when the feature is disabled', async (feature, heading) => {
    disabled.names = [feature];
    renderWithProviders(<AppRoutes />, {
      route: `/organizations/api-platform-demo/${feature}`,
      authState: authStatePresets.authenticated(),
    });

    expect(await screen.findByText('Page not found', {}, LAZY_ROUTE_TIMEOUT)).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: heading })).not.toBeInTheDocument();
  });

  it.each([
    ['subscription-plans', 'Subscription Plans'],
    ['meters', 'Meters'],
  ])('renders the org-level %s page with no Settings chrome', async (path, heading) => {
    renderWithProviders(<AppRoutes />, {
      route: `/organizations/api-platform-demo/${path}`,
      authState: authStatePresets.authenticated(),
    });

    expect(
      await screen.findByRole('heading', { name: heading }, LAZY_ROUTE_TIMEOUT),
    ).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Settings' })).not.toBeInTheDocument();
  });

  it('replaces the Settings link with a Manage menu while browsing the org', async () => {
    renderWithProviders(<AppRoutes />, {
      route: '/organizations/api-platform-demo/home',
      authState: authStatePresets.authenticated(),
    });

    expect(await screen.findByText('Manage')).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /^settings$/i })).not.toBeInTheDocument();
  });

  it('keeps the Manage menu, not a project Settings link, inside a project', async () => {
    renderWithProviders(<AppRoutes />, {
      route: '/organizations/api-platform-demo/projects/retail-apis/home',
      authState: authStatePresets.authenticated(),
    });

    expect(await screen.findByText('Manage')).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /^settings$/i })).not.toBeInTheDocument();
  });
});
