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

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { runtimeConfig, type GatewayVersionEntry } from '@/config/runtime';
import { resetTrackedFeatures, trackedFeatures } from '@/test/featureUsage';
import { accepts, aGateway, recorder, type Recorder } from '@/test/msw';
import { server } from '@/test/server';
import { makeConsoleScope } from '@/test/mockScope';
import { renderWithProviders, screen, waitFor } from '@/test/utils';
import { GatewayCreatePage } from './GatewayCreatePage';

vi.mock('@/featureUsage/moesif');

const ORG = 'api-platform-demo';

/** A deployment's `[[gateway_versions]]`, newest first per type; nothing for "ai". */
const CATALOG: GatewayVersionEntry[] = [
  { channel: 'LTS', latestVersion: 'v1.2.0', type: 'regular', version: '1.2' },
  { channel: 'STS', latestVersion: 'v1.1.0', type: 'regular', version: '1.1' },
  { channel: 'LTS', latestVersion: 'v1.0.0', type: 'event', version: '1.0' },
];

const configuredCatalog = runtimeConfig.gatewayVersions;

let requests: Recorder;

function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <Routes>
        <Route element={<GatewayCreatePage />} path="/organizations/:orgHandle/gateways/new" />
        <Route
          element={<div>gateway detail</div>}
          path="/organizations/:orgHandle/gateways/:gatewayId"
        />
      </Routes>
    </ApiScopeProvider>,
    {
      route: `/organizations/${ORG}/gateways/new`,
      scope: makeConsoleScope(),
    },
  );
}

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
  resetTrackedFeatures();
  runtimeConfig.gatewayVersions = CATALOG;
});

afterEach(() => {
  runtimeConfig.gatewayVersions = configuredCatalog;
});

/** Fills the two required free-text fields so a submit reaches the network. */
async function fillRequired(user: ReturnType<typeof renderPage>['user']) {
  await user.type(screen.getByLabelText(/Name/), 'Edge Gateway');
  await user.type(screen.getByLabelText(/URL/), 'https://edge.test:8443');
}

describe('GatewayCreatePage', () => {
  it('reveals every unmet rule on submit and sends nothing', async () => {
    server.use(accepts('post', '/gateways', aGateway(), { record: requests }));

    const { user } = renderPage();

    await user.click(screen.getByRole('button', { name: 'Provision gateway' }));

    expect(await screen.findByText('Enter a gateway name.')).toBeInTheDocument();
    expect(screen.getByText('Enter a URL.')).toBeInTheDocument();
    expect(requests.count()).toBe(0);
  });

  it('rejects an endpoint that is not a full URL', async () => {
    server.use(accepts('post', '/gateways', aGateway(), { record: requests }));

    const { user } = renderPage();

    await user.type(screen.getByLabelText(/Name/), 'Edge Gateway');
    await user.type(screen.getByLabelText(/URL/), 'edge.test');
    await user.click(screen.getByRole('button', { name: 'Provision gateway' }));

    expect(
      await screen.findByText('Enter a full URL, for example https://localhost:8443.'),
    ).toBeInTheDocument();
    expect(requests.count()).toBe(0);
  });

  it('derives the handle from the name and posts the spec body', async () => {
    server.use(
      accepts('post', '/gateways', aGateway({ displayName: 'Edge Gateway', id: 'edge-gateway' }), {
        record: requests,
      }),
    );

    const { user } = renderPage();

    // The type row leads the form; picking AI is the one answer that cannot be
    // changed after creation, so the body has to carry it.
    await user.click(screen.getByRole('radio', { name: /AI Gateway/ }));
    await user.type(screen.getByLabelText(/Name/), 'Edge Gateway');
    await user.type(screen.getByLabelText(/URL/), 'https://edge.test:8443');
    await user.click(screen.getByRole('button', { name: 'Provision gateway' }));

    await waitFor(() => expect(requests.count()).toBe(1));

    expect(JSON.parse(requests.last()!.body)).toEqual({
      displayName: 'Edge Gateway',
      endpoints: ['https://edge.test:8443'],
      functionalityType: 'ai',
      id: 'edge-gateway',
      isCritical: false,
      properties: { environment: 'development', gatewayMode: 'self-hosted' },
      version: '1.0',
    });

    // Landing on the gateway's own page is what tells the user provisioning
    // worked — the form itself shows no success state.
    expect(await screen.findByText('gateway detail')).toBeInTheDocument();
  });

  it('offers the configured releases for the type, latest first and selected', async () => {
    server.use(accepts('post', '/gateways', aGateway(), { record: requests }));

    const { user } = renderPage();

    await user.click(screen.getByRole('combobox', { name: /Gateway Version/ }));
    const options = await screen.findAllByRole('option');

    expect(options.map((option) => option.textContent)).toEqual([
      'api-gateway v1.2LatestLTS',
      'api-gateway v1.1STS',
    ]);

    await user.click(options[0]);
    await fillRequired(user);
    await user.click(screen.getByRole('button', { name: 'Provision gateway' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body)).toMatchObject({
      functionalityType: 'regular',
      version: '1.2',
    });
  });

  it('moves the version to the new type’s latest release when the type changes', async () => {
    server.use(accepts('post', '/gateways', aGateway(), { record: requests }));

    const { user } = renderPage();

    await user.click(screen.getByRole('radio', { name: /Event Gateway/ }));

    expect(screen.getByRole('combobox', { name: /Gateway Version/ })).toHaveTextContent(
      'event-gateway v1.0',
    );

    await fillRequired(user);
    await user.click(screen.getByRole('button', { name: 'Provision gateway' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body)).toMatchObject({
      functionalityType: 'event',
      version: '1.0',
    });
  });

  it('explains a type with no configured release instead of offering an empty list', async () => {
    const { user } = renderPage();

    await user.click(screen.getByRole('radio', { name: /AI Gateway/ }));

    expect(
      screen.getByText(
        'No release of this gateway type is configured for this console. It is registered as version 1.0.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: /Gateway Version/ })).toHaveAttribute(
      'aria-disabled',
      'true',
    );
  });

  describe('feature usage', () => {
    it('reports the registration of a self-hosted gateway of the chosen type', async () => {
      server.use(
        accepts('post', '/gateways', aGateway({ id: 'edge-gateway' }), { record: requests }),
      );

      const { user } = renderPage();

      await user.click(screen.getByRole('radio', { name: /AI Gateway/ }));
      await fillRequired(user);
      await user.click(screen.getByRole('button', { name: 'Provision gateway' }));

      await waitFor(() => expect(requests.count()).toBe(1));
      expect(trackedFeatures()).toEqual([
        ['apip.gateway.gateway-register-click', { gateway_hosting: 'self-hosted', gateway_type: 'ai' }],
      ]);
    });

    it('reports nothing when the form is refused before anything is sent', async () => {
      server.use(accepts('post', '/gateways', aGateway(), { record: requests }));

      const { user } = renderPage();

      await user.click(screen.getByRole('button', { name: 'Provision gateway' }));

      expect(await screen.findByText('Enter a gateway name.')).toBeInTheDocument();
      expect(trackedFeatures()).toEqual([]);
    });
  });
});
