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

import { http, HttpResponse } from 'msw';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { server } from '../../test/server';

const BASE = 'http://platform.test';

async function loadClient() {
  vi.stubEnv('VITE_PLATFORM_API_BASE_URL', BASE);
  vi.resetModules();
  return import('./gatewayClient');
}

const ORG = 'acme';

describe('gatewayClient (platform mode)', () => {
  beforeEach(() => vi.clearAllMocks());
  afterEach(() => vi.unstubAllEnvs());

  it('listGateways maps the platform list envelope', async () => {
    const { listGateways } = await loadClient();
    server.use(
      http.get(`${BASE}/api/v0.9/gateways`, () =>
        HttpResponse.json({
          list: [
            {
              id: 'gw1',
              name: 'gw1',
              properties: { gatewayMode: 'self-hosted' },
            },
          ],
        })
      )
    );
    const gateways = await listGateways(ORG);
    expect(gateways).toHaveLength(1);
    expect(gateways[0]).toMatchObject({ id: 'gw1', mode: 'self-hosted' });
  });

  it('getGateway returns undefined on a 404', async () => {
    const { getGateway } = await loadClient();
    server.use(
      http.get(`${BASE}/api/v0.9/gateways/missing`, () =>
        HttpResponse.json({}, { status: 404 })
      )
    );
    await expect(getGateway(ORG, 'missing')).resolves.toBeUndefined();
  });

  it('createGateway tags the gateway self-hosted in properties', async () => {
    const { createGateway } = await loadClient();
    let body: Record<string, unknown> | undefined;
    server.use(
      http.post(`${BASE}/api/v0.9/gateways`, async ({ request }) => {
        body = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json({ ...body, id: 'gw-new' });
      })
    );
    const created = await createGateway(ORG, {
      name: 'edge-gw',
      displayName: 'Edge GW',
      vhost: 'mg.example',
      functionalityType: 'regular',
    });
    expect(body).toMatchObject({
      id: 'edge-gw',
      endpoints: ['mg.example'],
      properties: { gatewayMode: 'self-hosted' },
    });
    expect(created).toMatchObject({ id: 'gw-new', mode: 'self-hosted' });
  });

  it('createGatewayToken maps the token response', async () => {
    const { createGatewayToken } = await loadClient();
    server.use(
      http.post(`${BASE}/api/v0.9/gateways/gw1/tokens`, () =>
        HttpResponse.json({
          id: 'tok1',
          token: 'secret-token',
          createdAt: '2026-06-19T00:00:00Z',
          message: 'one-time',
        })
      )
    );
    const token = await createGatewayToken(ORG, 'gw1');
    expect(token).toEqual({
      id: 'tok1',
      token: 'secret-token',
      createdAt: '2026-06-19T00:00:00Z',
      message: 'one-time',
    });
  });
});
