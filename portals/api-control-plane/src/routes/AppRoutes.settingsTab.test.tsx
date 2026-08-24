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

import { ExtensionsProvider, type ApiControlPlaneExtension } from '../extensions';
import { AppRoutes } from './AppRoutes';
import { authStatePresets } from '../test/mockAuthState';
import { renderWithProviders, screen } from '../test/utils';

// Covers the `settings.<scope>.tabs` slot: a host-injected extension
// registered against that slot should render inside the Settings page's own
// sub-nav (nested under /settings) rather than as a top-level sidebar/route
// entry.
describe('AppRoutes settingsTab extensions', () => {
  beforeEach(() => vi.stubEnv('VITE_USE_MOCK_API', 'true'));
  afterEach(() => vi.unstubAllEnvs());

  const mockExtension: ApiControlPlaneExtension = {
    id: 'environments',
    routePath: 'settings/environments',
    render: () => <div>Mock Environments page</div>,
    label: 'Environments',
    scope: 'project',
    slot: 'settings.project.tabs',
    order: 10,
  };

  it('lists the extension as a Settings sub-nav tab and does not add a top-level sidebar entry', async () => {
    renderWithProviders(
      <ExtensionsProvider extensions={[mockExtension]}>
        <AppRoutes extensions={[mockExtension]} />
      </ExtensionsProvider>,
      {
        route: '/organizations/api-platform-demo/projects/retail-apis/settings',
        authState: authStatePresets.authenticated(),
      }
    );

    // Settings sub-nav shows both the built-in "General" tab and the
    // extension-contributed "Environments" tab.
    expect(await screen.findByText('General')).toBeInTheDocument();
    expect(await screen.findByText('Environments')).toBeInTheDocument();

    // It must not also appear as its own top-level sidebar entry (there is
    // only ever one "Environments" text node on the page: the settings tab).
    expect(screen.getAllByText('Environments')).toHaveLength(1);
  });

  it('renders the extension element when its settings tab route is visited', async () => {
    renderWithProviders(
      <ExtensionsProvider extensions={[mockExtension]}>
        <AppRoutes extensions={[mockExtension]} />
      </ExtensionsProvider>,
      {
        route: '/organizations/api-platform-demo/projects/retail-apis/settings/environments',
        authState: authStatePresets.authenticated(),
      }
    );

    expect(await screen.findByText('Mock Environments page')).toBeInTheDocument();
  });

  it('does not register a conflicting descriptor whose slot and scope disagree', async () => {
    // Type-valid but internally inconsistent: the slot says "project" while
    // `scope` says "organization". Neither `useSettingsTabs` nor
    // `settingsTabRoutesFor` may accept this — it must not render in EITHER
    // settings page, rather than rendering in the wrong one with a mismatched
    // Port (e.g. missing `projectHandle`).
    const conflictingExtension: ApiControlPlaneExtension = {
      ...mockExtension,
      id: 'conflicting',
      label: 'Conflicting',
      slot: 'settings.project.tabs',
      scope: 'organization',
    };

    renderWithProviders(
      <ExtensionsProvider extensions={[conflictingExtension]}>
        <AppRoutes extensions={[conflictingExtension]} />
      </ExtensionsProvider>,
      {
        route: '/organizations/api-platform-demo/projects/retail-apis/settings',
        authState: authStatePresets.authenticated(),
      }
    );

    expect(await screen.findByText('General')).toBeInTheDocument();
    expect(screen.queryByText('Conflicting')).not.toBeInTheDocument();
  });
});
