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
import {
  aGateway,
  accepts,
  recorder,
  resource,
  type GatewayFixture,
  type Recorder,
} from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor } from '@/test/utils';
import { GatewayDetailPage } from './GatewayDetailPage';

const ORG = 'api-platform-demo';
const GATEWAY_ID = 'default-gw';

const gateway = (overrides: Partial<GatewayFixture> = {}): GatewayFixture =>
  aGateway({
    createdAt: '2026-07-01T10:00:00Z',
    description: 'Default GW',
    displayName: 'Default GW',
    endpoints: ['https://localhost:8443'],
    id: GATEWAY_ID,
    isActive: false,
    properties: { environment: 'development', gatewayMode: 'self-hosted' },
    version: '1.0',
    ...overrides,
  });

/** One recorder per handler: the shared one has no method filter of its own. */
let updates: Recorder;
let tokenRotations: Recorder;
let manifestReads: Recorder;

/**
 * The page's hooks read `ApiScopeContext` rather than the console scope, so the
 * provider is mounted here — without it every query stays `enabled: false` and
 * the page renders its loading state forever.
 */
function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <Routes>
        <Route
          element={<GatewayDetailPage />}
          path="/organizations/:orgHandle/gateways/:gatewayId"
        />
      </Routes>
    </ApiScopeProvider>,
    {
      route: `/organizations/${ORG}/gateways/${GATEWAY_ID}`,
      scope: makeConsoleScope(),
    },
  );
}

beforeEach(() => {
  updates = recorder();
  tokenRotations = recorder();
  manifestReads = recorder();
  resetHttpClient();
  // The banner's dismissal outlives a reload by design, so a test that closes
  // it would otherwise leak into the next one.
  window.localStorage.clear();
});

describe('GatewayDetailPage', () => {
  it('shows setup progress, identity and the Quick Start commands', async () => {
    server.use(resource('/gateways/:gatewayId', gateway()));

    renderPage();

    // Step 1 of 2: registered, not yet connected.
    expect(
      await screen.findByText('You successfully created your Default GW gateway'),
    ).toBeInTheDocument();
    expect(screen.getByText('1/2')).toBeInTheDocument();

    // Identity: name, status, hosting mode, kind and version all on the header.
    expect(screen.getByRole('heading', { name: 'Default GW' })).toBeInTheDocument();
    expect(screen.getByText('Not connected')).toBeInTheDocument();
    expect(screen.getByText('Self-hosted')).toBeInTheDocument();
    expect(screen.getByText('Regular')).toBeInTheDocument();
    expect(screen.getByText('v1.0')).toBeInTheDocument();

    // The panel's commands are built from the gateway's own type and version.
    expect(
      screen.getByText(/wso2apip-api-gateway-1\.0\.zip/, { exact: false }),
    ).toBeInTheDocument();
  });

  it('counts setup complete once the gateway agent has connected', async () => {
    server.use(resource('/gateways/:gatewayId', gateway({ isActive: true })));

    renderPage();

    expect(await screen.findByText('Your Default GW gateway is connected')).toBeInTheDocument();
    expect(screen.getByText('2/2')).toBeInTheDocument();
  });

  it('keeps the setup banner closed after it is dismissed', async () => {
    server.use(resource('/gateways/:gatewayId', gateway()));

    const { user, unmount } = renderPage();

    const banner = await screen.findByText('You successfully created your Default GW gateway');
    expect(banner).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Dismiss setup progress' }));
    await waitFor(() => expect(banner).not.toBeInTheDocument());

    // A second visit — the dismissal is persisted, not component state.
    unmount();
    renderPage();

    expect(await screen.findByRole('heading', { name: 'Default GW' })).toBeInTheDocument();
    expect(
      screen.queryByText('You successfully created your Default GW gateway'),
    ).not.toBeInTheDocument();
  });

  it('saves an in-place edit of the name and description as a whole gateway', async () => {
    server.use(
      resource('/gateways/:gatewayId', gateway()),
      accepts('put', '/gateways/:gatewayId', gateway({ displayName: 'Edge GW' }), {
        record: updates,
      }),
    );

    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Edit name and description' }));

    const name = screen.getByRole('textbox', { name: /Name/ });
    await user.clear(name);
    await user.type(name, 'Edge GW');
    await user.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(updates.count()).toBe(1));

    // The spec's update body is the whole gateway, so the untouched fields have
    // to survive the edit rather than being dropped from a partial payload.
    expect(JSON.parse(updates.last()!.body)).toMatchObject({
      displayName: 'Edge GW',
      endpoints: ['https://localhost:8443'],
      functionalityType: 'regular',
      version: '1.0',
    });
  });

  it('refuses to save an empty gateway name', async () => {
    server.use(
      resource('/gateways/:gatewayId', gateway()),
      accepts('put', '/gateways/:gatewayId', gateway(), { record: updates }),
    );

    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Edit name and description' }));
    await user.clear(screen.getByRole('textbox', { name: /Name/ }));
    await user.click(screen.getByRole('button', { name: 'Save' }));

    expect(await screen.findByText('Enter a gateway name.')).toBeInTheDocument();
    expect(updates.count()).toBe(0);
  });

  it('reveals the registration token only after Reconfigure is confirmed', async () => {
    server.use(
      resource('/gateways/:gatewayId', gateway()),
      accepts(
        'post',
        '/gateways/:gatewayId/tokens',
        { id: 'token-1', token: 'plaintext-token-value' },
        { record: tokenRotations },
      ),
    );

    const { container, user } = renderPage();

    // Nothing is issued on load: step 2 offers the button and no env file, so
    // a token never reaches the page unless the user asks for one.
    await screen.findByRole('button', { name: 'Reconfigure' });
    expect(container.textContent).not.toContain('GATEWAY_REGISTRATION_TOKEN');
    expect(tokenRotations.count()).toBe(0);

    await user.click(screen.getByRole('button', { name: 'Reconfigure' }));
    await user.click(screen.getByRole('button', { name: 'Generate new token' }));

    // The plaintext token is returned once, so it has to be rendered straight
    // from the response rather than read back from the cache.
    await waitFor(() => expect(container.textContent).toContain('plaintext-token-value'));
    expect(tokenRotations.count()).toBe(1);
  });
  it('lists the manifest policies on the Policies tab, sorted by name', async () => {
    server.use(
      resource('/gateways/:gatewayId', gateway()),
      resource(
        '/gateways/:gatewayId/manifest',
        {
          policies: [
            {
              description: 'Secures APIs by validating API keys in incoming requests.',
              isCustomPolicy: false,
              name: 'API Key Auth',
              version: 'v1',
            },
            {
              isCustomPolicy: true,
              name: 'Analytics Header Filter',
              version: 'v2',
            },
          ],
        },
        { record: manifestReads },
      ),
    );

    const { user } = renderPage();

    // Configurations leads, so nothing is fetched for policies until asked.
    await screen.findByRole('tab', { name: 'Policies' });
    expect(manifestReads.count()).toBe(0);

    await user.click(screen.getByRole('tab', { name: 'Policies' }));

    const rows = await screen.findAllByRole('row');
    // Header plus two policies, ordered by name ascending — "Analytics" first
    // even though the manifest returned it second.
    expect(rows).toHaveLength(3);
    expect(rows[1]).toHaveTextContent('Analytics Header Filter');
    expect(rows[2]).toHaveTextContent('API Key Auth');

    // Where each policy came from is derived from `isCustomPolicy`.
    expect(rows[1]).toHaveTextContent('Custom');
    expect(rows[2]).toHaveTextContent('Policy Hub');

    // A policy with no description shows the absence rather than a blank cell.
    expect(rows[1]).toHaveTextContent('No description');
    expect(manifestReads.count()).toBe(1);
  });

  it('reverses the policy order when the Name column is sorted again', async () => {
    server.use(
      resource('/gateways/:gatewayId', gateway()),
      resource('/gateways/:gatewayId/manifest', {
        policies: [
          { isCustomPolicy: false, name: 'Alpha Policy', version: 'v1' },
          { isCustomPolicy: false, name: 'Zulu Policy', version: 'v1' },
        ],
      }),
    );

    const { user } = renderPage();
    await user.click(await screen.findByRole('tab', { name: 'Policies' }));

    await screen.findByText('Alpha Policy');
    await user.click(screen.getByRole('button', { name: /Name/ }));

    const rows = screen.getAllByRole('row');
    expect(rows[1]).toHaveTextContent('Zulu Policy');
    expect(rows[2]).toHaveTextContent('Alpha Policy');
  });

  it('says no manifest was found when the gateway has reported no policies', async () => {
    server.use(
      resource('/gateways/:gatewayId', gateway()),
      // An unconnected gateway has never synced, so its manifest comes back
      // with no policy list at all rather than an empty one.
      resource('/gateways/:gatewayId/manifest', {}),
    );

    const { user } = renderPage();
    await user.click(await screen.findByRole('tab', { name: 'Policies' }));

    expect(await screen.findByText('No gateway manifest found')).toBeInTheDocument();
    expect(screen.queryByRole('table')).not.toBeInTheDocument();
  });
});
