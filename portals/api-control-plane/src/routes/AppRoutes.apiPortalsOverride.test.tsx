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

import { useLocation } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  ExtensionsProvider,
  PAGE_API_PORTALS_SLOT,
  type ApiControlPlaneExtension,
} from '../extensions';
import { AppRoutes } from './AppRoutes';
import { routes } from './paths';
import { anOrganization, aProject, collection, resource } from '../test/msw';
import { authStatePresets } from '../test/mockAuthState';
import { server } from '../test/server';
import { renderWithProviders, screen } from '../test/utils';

// Covers the `page.apiPortals` slot: the cloud build replaces the built-in API
// Portals registry with its managed portals, at the same URL. Like the gateways
// override, it is one self-contained flow, so the built-in `new`/`:apiPortalId`
// URLs land on the index.
describe('AppRoutes API Portals page override', () => {
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

  const portalsPath = routes.orgApiPortals('api-platform-demo');

  const override: ApiControlPlaneExtension = {
    id: 'apiPortals',
    label: 'Portals',
    level: 'organization',
    order: 120,
    render: () => <div>Mock managed portals</div>,
    routePath: 'api-portals',
    slot: PAGE_API_PORTALS_SLOT,
  };

  /** Reports the live URL so a redirect is asserted, not just what rendered. */
  function LocationProbe() {
    return <div data-testid="pathname">{useLocation().pathname}</div>;
  }

  const renderAt = (route: string) =>
    renderWithProviders(
      <ExtensionsProvider extensions={[override]}>
        <AppRoutes extensions={[override]} />
        <LocationProbe />
      </ExtensionsProvider>,
      { authState: authStatePresets.authenticated(), route }
    );

  it('renders the override at the API Portals index', async () => {
    renderAt(portalsPath);

    expect(await screen.findByText('Mock managed portals')).toBeInTheDocument();
    expect(screen.getByTestId('pathname').textContent).toBe(portalsPath);
  });

  it('redirects the built-in create URL to the index', async () => {
    renderAt(routes.newOrgApiPortal('api-platform-demo'));

    expect(await screen.findByText('Mock managed portals')).toBeInTheDocument();
    expect(screen.getByTestId('pathname').textContent).toBe(portalsPath);
  });

  it('redirects the old managed portals URL to the registry', async () => {
    renderAt(routes.legacyManagedApiPortals('api-platform-demo'));

    expect(await screen.findByText('Mock managed portals')).toBeInTheDocument();
    expect(screen.getByTestId('pathname').textContent).toBe(portalsPath);
  });

  it('redirects a portal detail URL to the index', async () => {
    renderAt(routes.orgApiPortal('api-platform-demo', 'portal-1'));

    expect(await screen.findByText('Mock managed portals')).toBeInTheDocument();
    expect(screen.getByTestId('pathname').textContent).toBe(portalsPath);
  });
});
