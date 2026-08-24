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
import { authStatePresets } from '../test/mockAuthState';
import { renderWithProviders, screen } from '../test/utils';

// Covers the org-level Settings page (mirrors ai-workspace, which mounts the
// same Settings feature at both org and project scope) and the sidebar's
// pinned-to-bottom Settings link, which must show exactly one link at a
// time — organization-level while browsing the org, project-level once a
// project is selected — never both at once.
describe('Org-level Settings', () => {
  beforeEach(() => vi.stubEnv('VITE_USE_MOCK_API', 'true'));
  afterEach(() => vi.unstubAllEnvs());

  it('redirects /settings to /settings/general at the org level', async () => {
    renderWithProviders(<AppRoutes />, {
      route: '/organizations/api-platform-demo/settings',
      authState: authStatePresets.authenticated(),
    });

    expect(
      await screen.findByText(/Minimal settings overview for API Platform Demo/)
    ).toBeInTheDocument();
  });

  it('shows one org-scoped pinned Settings link while browsing the org', async () => {
    renderWithProviders(<AppRoutes />, {
      route: '/organizations/api-platform-demo/home',
      authState: authStatePresets.authenticated(),
    });

    const settingsLinks = await screen.findAllByRole('link', { name: /settings/i });
    expect(settingsLinks).toHaveLength(1);
    expect(settingsLinks[0]).toHaveAttribute(
      'href',
      '/organizations/api-platform-demo/settings'
    );
  });

  it('shows one project-scoped pinned Settings link inside a project, not both', async () => {
    renderWithProviders(<AppRoutes />, {
      route: '/organizations/api-platform-demo/projects/retail-apis/home',
      authState: authStatePresets.authenticated(),
    });

    // The org-level Settings link must be hidden here — never both at once.
    const settingsLinks = await screen.findAllByRole('link', { name: /settings/i });
    expect(settingsLinks).toHaveLength(1);
    expect(settingsLinks[0]).toHaveAttribute(
      'href',
      '/organizations/api-platform-demo/projects/retail-apis/settings'
    );
  });
});
