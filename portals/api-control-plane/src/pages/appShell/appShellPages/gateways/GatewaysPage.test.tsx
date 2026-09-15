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
import { aGateway, collection, recorder, type GatewayFixture, type Recorder } from '@/test/msw';
import { server } from '@/test/server';
import { makeConsoleScope } from '@/test/mockScope';
import { renderWithProviders, screen } from '@/test/utils';
import { GatewaysPage } from './GatewaysPage';

const ORG = 'api-platform-demo';

/**
 * One of each hosting mode, since the mode filter is the page's only stateful
 * control and `gatewayMode` reads it out of the free-form `properties` bag.
 */
const gateways: GatewayFixture[] = [
  aGateway({
    displayName: 'Edge Gateway',
    endpoints: ['https://edge.test'],
    id: 'edge-gateway',
    properties: { gatewayMode: 'self-hosted' },
  }),
  aGateway({
    displayName: 'Cloud Gateway',
    endpoints: ['https://cloud.test'],
    id: 'cloud-gateway',
    isActive: false,
  }),
];

let requests: Recorder;

/**
 * The page's hook reads `ApiScopeContext`, not the console scope, so the
 * provider has to be mounted here — without it the query stays
 * `enabled: false` and the page renders its loading state forever.
 */
function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <Routes>
        <Route element={<GatewaysPage />} path="/organizations/:orgHandle/gateways" />
      </Routes>
    </ApiScopeProvider>,
    {
      route: `/organizations/${ORG}/gateways`,
      scope: makeConsoleScope(),
    },
  );
}

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('GatewaysPage', () => {
  it('lists gateways from the platform API, scoped to the organization', async () => {
    server.use(collection('/gateways', gateways, { record: requests }));

    renderPage();

    expect(await screen.findByText('Edge Gateway')).toBeInTheDocument();
    expect(screen.getByText('Cloud Gateway')).toBeInTheDocument();
    // Both cards carry the connection state, which is the mark the fleet view
    // exists for; anything more detailed lives on the gateway's own page. Two
    // matches each: the summary tile's label, and the one card in that state.
    expect(screen.getAllByText('Connected')).toHaveLength(2);
    expect(screen.getAllByText('Not connected')).toHaveLength(2);
    expect(requests.last()?.headers.get('X-Org-Id')).toBe(ORG);
  });

  it('switches between the card grid and the compact list', async () => {
    server.use(collection('/gateways', gateways, { record: requests }));

    const { user } = renderPage();

    await screen.findByText('Edge Gateway');
    expect(screen.getAllByTestId('gateway-grid-view').length).toBeGreaterThan(0);

    await user.click(screen.getByRole('button', { name: 'List view' }));

    // The environment grouping survives the switch — only the gateways inside
    // each group change shape, so the group count is the same either way.
    expect(screen.queryByTestId('gateway-grid-view')).not.toBeInTheDocument();
    expect(screen.getAllByTestId('gateway-list-view').length).toBeGreaterThan(0);
    expect(screen.getByText('Edge Gateway')).toBeInTheDocument();
    expect(screen.getByText('Cloud Gateway')).toBeInTheDocument();
  });

  it('filters by hosting mode, which comes from the gateway properties', async () => {
    server.use(collection('/gateways', gateways, { record: requests }));

    const { user } = renderPage();

    await screen.findByText('Edge Gateway');
    await user.click(screen.getByRole('button', { name: 'Self-hosted' }));

    expect(screen.getByText('Edge Gateway')).toBeInTheDocument();
    expect(screen.queryByText('Cloud Gateway')).not.toBeInTheDocument();
  });

  it('searches the loaded fleet by name without asking the server again', async () => {
    server.use(collection('/gateways', gateways, { record: requests }));

    const { user } = renderPage();

    await screen.findByText('Edge Gateway');
    const requestsBefore = requests.count();

    await user.type(screen.getByPlaceholderText('Search gateways'), 'cloud');

    expect(screen.getByText('Cloud Gateway')).toBeInTheDocument();
    expect(screen.queryByText('Edge Gateway')).not.toBeInTheDocument();
    expect(requests.count()).toBe(requestsBefore);
  });

  it('offers the create prompt, and nothing else, when the organization has no gateways', async () => {
    server.use(collection('/gateways', [], { record: requests }));

    renderPage();

    expect(await screen.findByText('Provision your first gateway')).toBeInTheDocument();
    // The prompt is the whole page: no search over an empty fleet, no summary
    // tiles reading zero, no filter with nothing to filter.
    expect(screen.queryByPlaceholderText('Search gateways')).not.toBeInTheDocument();
    expect(screen.queryByText('Total gateways')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Self-hosted' })).not.toBeInTheDocument();
    // And one way out, not two: the page-title action steps aside for it.
    expect(screen.getAllByRole('button', { name: /Provision gateway/ })).toHaveLength(1);
  });
});
