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

import {
  ExtensionsProvider,
  type ApiControlPlaneExtension,
} from '../extensions';
import { AppRoutes } from './AppRoutes';
import { anOrganization, aProject, collection, resource } from '../test/msw';
import { authStatePresets } from '../test/mockAuthState';
import { server } from '../test/server';
import { renderWithProviders, screen } from '../test/utils';

// Covers the `settings.<level>.tabs` slot: a host-injected extension
// registered against that slot renders inside the Settings page's own sub-nav
// (nested under /settings), never as a top-level sidebar/route entry.
describe('AppRoutes settingsTab extensions', () => {
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
      collection('/rest-apis', [])
    );
  });
  afterEach(() => vi.unstubAllEnvs());

  const projectSettingsRoute =
    '/organizations/api-platform-demo/projects/retail-apis/settings';

  const mockExtension: ApiControlPlaneExtension = {
    id: 'environments',
    label: 'Environments',
    level: 'project',
    order: 10,
    render: () => <div>Mock Environments page</div>,
    routePath: 'settings/environments',
    slot: 'settings.project.tabs',
  };

  const renderWithExtension = (
    extension: ApiControlPlaneExtension,
    route: string
  ) =>
    renderWithProviders(
      <ExtensionsProvider extensions={[extension]}>
        <AppRoutes extensions={[extension]} />
      </ExtensionsProvider>,
      { authState: authStatePresets.authenticated(), route }
    );

  it('lists the extension as a Settings sub-nav tab and adds no top-level sidebar entry', async () => {
    renderWithExtension(mockExtension, projectSettingsRoute);

    expect(await screen.findByText('General')).toBeInTheDocument();
    expect(await screen.findByText('Environments')).toBeInTheDocument();

    // One text node only — the settings tab. A second would mean it also
    // registered itself as a top-level sidebar item.
    expect(screen.getAllByText('Environments')).toHaveLength(1);
  });

  it('renders what the extension returns when its settings tab route is visited', async () => {
    renderWithExtension(mockExtension, `${projectSettingsRoute}/environments`);

    expect(
      await screen.findByText('Mock Environments page')
    ).toBeInTheDocument();
  });

  it('hands the extension a Port carrying the scope it was rendered in', async () => {
    const portAware: ApiControlPlaneExtension = {
      ...mockExtension,
      render: (port) => <div>Port project: {port.projectHandle}</div>,
    };

    renderWithExtension(portAware, `${projectSettingsRoute}/environments`);

    expect(
      await screen.findByText('Port project: retail-apis')
    ).toBeInTheDocument();
  });

  it('drops a descriptor whose slot and level disagree', async () => {
    // Type-valid but internally inconsistent: the slot says "project" while
    // `level` says "organization". Neither `useSettingsTabs` nor the route pass
    // may accept it — it must render in NEITHER settings page rather than in
    // the wrong one, against a Port missing the scope it expects.
    const conflicting: ApiControlPlaneExtension = {
      ...mockExtension,
      id: 'conflicting',
      label: 'Conflicting',
      level: 'organization',
      slot: 'settings.project.tabs',
    };

    renderWithExtension(conflicting, projectSettingsRoute);

    expect(await screen.findByText('General')).toBeInTheDocument();
    expect(screen.queryByText('Conflicting')).not.toBeInTheDocument();
  });
});
