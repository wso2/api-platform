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

import { AppRoutes } from './routes/AppRoutes';
import { authStatePresets } from './test/mockAuthState';
import { renderWithProviders, screen } from './test/utils';

// Smoke test: render the REAL route tree (ProtectedRoute → ConsoleScopeProvider
// → AppLayout/AppShell → lazy pages) through the provider stack, in mock-API
// mode so the scope queries resolve from fixtures with no network. This is a
// thin guardrail that catches provider-wiring regressions, not deep assertions.
describe('App smoke (mock mode, authenticated)', () => {
  beforeEach(() => vi.stubEnv('VITE_USE_MOCK_API', 'true'));
  afterEach(() => vi.unstubAllEnvs());

  it('renders the org home through the full app shell', async () => {
    renderWithProviders(<AppRoutes />, {
      route: '/organizations/api-platform-demo/home',
      authState: authStatePresets.authenticated(),
    });

    // The AppLayout shell (footer) renders…
    expect(await screen.findByText(/WSO2 LLC/)).toBeInTheDocument();
    // …and the org's data has flowed through the real ConsoleScopeProvider.
    expect(
      (await screen.findAllByText(/API Platform Demo/)).length
    ).toBeGreaterThan(0);
  });

  it('navigates to the projects list and renders project cards', async () => {
    renderWithProviders(<AppRoutes />, {
      route: '/organizations/api-platform-demo/projects',
      authState: authStatePresets.authenticated(),
    });

    expect(await screen.findByText('Projects')).toBeInTheDocument();
    expect(await screen.findByText('Retail APIs')).toBeInTheDocument();
  });
});
