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
  PAGE_GATEWAYS_SLOT,
  type ApiControlPlaneExtension,
} from '../extensions';
import { AppRoutes } from './AppRoutes';
import { routes } from './paths';
import { anOrganization, aProject, collection, resource } from '../test/msw';
import { authStatePresets } from '../test/mockAuthState';
import { server } from '../test/server';
import { renderWithProviders, screen } from '../test/utils';

// Covers the `page.gateways` slot: an override replaces the built-in Gateways
// page with one self-contained flow that keeps its view in local state. It
// therefore owns no nested `new`/`:gatewayId` URLs, and those must land on the
// index rather than render the list at a URL claiming to be a create or detail
// page.
describe('AppRoutes gateways page override', () => {
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

  const gatewaysPath = routes.gateways('api-platform-demo');

  const override: ApiControlPlaneExtension = {
    id: 'gateways',
    label: 'Gateways',
    level: 'organization',
    order: 45,
    render: () => <div>Mock managed gateways</div>,
    routePath: 'gateways',
    slot: PAGE_GATEWAYS_SLOT,
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

  it('renders the override at the gateways index', async () => {
    renderAt(gatewaysPath);

    expect(await screen.findByText('Mock managed gateways')).toBeInTheDocument();
    expect(screen.getByTestId('pathname').textContent).toBe(gatewaysPath);
  });

  it('redirects the built-in create URL to the index', async () => {
    renderAt(routes.newGateway('api-platform-demo'));

    expect(await screen.findByText('Mock managed gateways')).toBeInTheDocument();
    // The redirect target must be the gateways index itself. Resolving it
    // relative to the full matched pathname would send `/gateways/new` back to
    // `/gateways/new` and loop forever.
    expect(screen.getByTestId('pathname').textContent).toBe(gatewaysPath);
  });

  it('redirects a gateway detail URL to the index', async () => {
    renderAt(routes.gateway('api-platform-demo', 'gw-1'));

    expect(await screen.findByText('Mock managed gateways')).toBeInTheDocument();
    expect(screen.getByTestId('pathname').textContent).toBe(gatewaysPath);
  });
});
