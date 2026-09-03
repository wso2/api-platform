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
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  API_CONTROL_PLANE_GATEWAYS_SLOT,
  ExtensionsProvider,
  type ApiControlPlanePageOverride,
} from '../extensions';
import { HiddenRegionsProvider } from '../slots';
import { AppRoutes } from './AppRoutes';
import { aGateway, anOrganization, aProject, collection, resource } from '../test/msw';
import { authStatePresets } from '../test/mockAuthState';
import { server } from '../test/server';
import { renderWithProviders, screen } from '../test/utils';

/*
 * Covers the `page.gateways` override slot — the console's only page-override
 * position, and the two primitives behind it at a page (rather than a tab)
 * position, which nothing else here exercises.
 *
 * The built-ins moved one level down to make room for it, so half of these
 * tests are the regression guard for that: `routes.gateways()`,
 * `routes.newGateway()` and `routes.gateway()` still answer exactly as before
 * whenever no override is registered.
 */
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
      collection('/rest-apis', []),
      collection('/gateways', []),
    );
  });
  afterEach(() => vi.unstubAllEnvs());

  const GATEWAYS = '/organizations/api-platform-demo/gateways';

  /** Only the built-in list page carries this sentence. */
  const BUILT_IN_LIST = /Provision and manage self-hosted or WSO2-managed/;

  const override: ApiControlPlanePageOverride = {
    id: 'gateways',
    order: 0,
    render: () => <div>Cloud gateways page</div>,
    slot: API_CONTROL_PLANE_GATEWAYS_SLOT,
  };

  const renderAt = (route: string, entries: readonly ApiControlPlanePageOverride[] = []) =>
    renderWithProviders(
      <ExtensionsProvider extensions={entries}>
        <AppRoutes extensions={entries} />
      </ExtensionsProvider>,
      { authState: authStatePresets.authenticated(), route },
    );

  it('renders the built-in list page when nothing is registered', async () => {
    renderAt(GATEWAYS);

    expect(await screen.findByText(BUILT_IN_LIST)).toBeInTheDocument();
  });

  it('keeps the built-in create and detail URLs answering after the re-nesting', async () => {
    renderAt(`${GATEWAYS}/new`);

    expect(await screen.findByText('Provision a gateway')).toBeInTheDocument();
  });

  it('passes ancestor route params down into the re-nested detail page', async () => {
    // The detail page reads BOTH `orgHandle` (matched by the outer `gateways/*`
    // route) and `gatewayId` (matched by the inner one). A page that renders
    // the fetched gateway at all is the proof the two accumulated across the
    // nested `<Routes>`: match the inner route alone and `gatewayId` is
    // undefined, which leaves the query disabled and the page on its loading
    // state forever.
    server.use(
      resource(
        '/gateways/:gatewayId',
        aGateway({ displayName: 'Shared Gateway', id: 'shared-gateway' })
      )
    );

    renderAt(`${GATEWAYS}/shared-gateway`);

    expect(await screen.findByText('Shared Gateway')).toBeInTheDocument();
  });

  it('renders a registered override instead of the built-in list page', async () => {
    renderAt(GATEWAYS, [override]);

    expect(await screen.findByText('Cloud gateways page')).toBeInTheDocument();
    expect(screen.queryByText(BUILT_IN_LIST)).not.toBeInTheDocument();
  });

  it('mounts the override with a trailing wildcard so its own nested routes resolve', async () => {
    // The override renders its own `<Routes>`; `gateways/create` is a path only
    // it knows about. Drop the `/*` on the host route and this is a 404 while
    // the index route above still passes, which is the failure that looks like
    // a routing bug inside the plugin.
    const nested: ApiControlPlanePageOverride = {
      ...override,
      render: () => (
        <Routes>
          <Route index element={<div>Cloud gateways page</div>} />
          <Route path="create" element={<div>Cloud provision form</div>} />
        </Routes>
      ),
    };

    renderAt(`${GATEWAYS}/create`, [nested]);

    expect(await screen.findByText('Cloud provision form')).toBeInTheDocument();
  });

  it('hands the override a Port carrying the scope it rendered in', async () => {
    const portAware: ApiControlPlanePageOverride = {
      ...override,
      render: (port) => <div>Port org: {port.orgHandle}</div>,
    };

    renderAt(GATEWAYS, [portAware]);

    expect(await screen.findByText('Port org: api-platform-demo')).toBeInTheDocument();
  });

  it('suppresses the built-in page when the region is hidden and nothing replaces it', async () => {
    // Slot adds, Hideable suppresses: hiding the region without registering an
    // override has to leave the position empty rather than fall back to the
    // built-in.
    renderWithProviders(
      <HiddenRegionsProvider hidden={[API_CONTROL_PLANE_GATEWAYS_SLOT]}>
        <AppRoutes />
      </HiddenRegionsProvider>,
      { authState: authStatePresets.authenticated(), route: GATEWAYS },
    );

    expect(await screen.findByText('API Platform Demo')).toBeInTheDocument();
    expect(screen.queryByText(BUILT_IN_LIST)).not.toBeInTheDocument();
  });
});
