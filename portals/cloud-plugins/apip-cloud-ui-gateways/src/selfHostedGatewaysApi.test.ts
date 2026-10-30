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

import { describe, expect, it, vi } from 'vitest';

import type { ApiFetch } from './hostPort';
import { PENDING_ENDPOINT, createSelfHostedGatewaysClient } from './selfHostedGatewaysApi';
import type { SelfHostedGatewayInput } from './types';

type Call = { method: string; path: string; body?: unknown };

/** An apiFetch that records every call and answers through `respond`. */
const recording = (respond: (call: Call) => unknown) => {
  const calls: Call[] = [];
  const apiFetch = (async (method: string, path: string, body?: unknown) => {
    const call = { method, path, body };
    calls.push(call);
    return respond(call);
  }) as ApiFetch;
  return { apiFetch, calls };
};

const input = (overrides: Partial<SelfHostedGatewayInput> = {}): SelfHostedGatewayInput => ({
  name: 'Edge One',
  type: 'regular',
  environmentId: 'dev',
  ...overrides,
});

describe('listGateways', () => {
  it('lists every kind when none is asked for', async () => {
    const { apiFetch, calls } = recording(() => ({ list: [] }));
    await createSelfHostedGatewaysClient(apiFetch).listGateways();
    expect(calls[0]).toMatchObject({ method: 'GET', path: '/gateways' });
  });

  it('filters by the requested kinds', async () => {
    const { apiFetch, calls } = recording(() => ({ list: [] }));
    await createSelfHostedGatewaysClient(apiFetch).listGateways(['regular', 'event']);
    expect(calls[0].path).toBe('/gateways?functionalityType=regular,event');
  });

  it('maps a native record to a self-hosted gateway', async () => {
    const { apiFetch } = recording(() => ({
      list: [
        {
          id: 'dev-edge-one',
          displayName: 'Edge One',
          functionalityType: 'event',
          endpoints: ['https://edge.example.com'],
          isActive: true,
          version: '1.2',
          properties: { environment: 'dev', isDefault: 'true' },
        },
      ],
    }));
    const [gateway] = await createSelfHostedGatewaysClient(apiFetch).listGateways();
    expect(gateway).toMatchObject({
      id: 'dev-edge-one',
      name: 'Edge One',
      type: 'event',
      hosting: 'self-hosted',
      environmentId: 'dev',
      url: 'https://edge.example.com',
      status: 'active',
      isDefault: true,
      version: '1.2',
    });
  });

  it('never surfaces the pending placeholder as the gateway url', async () => {
    const { apiFetch } = recording(() => ({
      list: [{ id: 'g1', endpoints: [PENDING_ENDPOINT] }],
    }));
    const [gateway] = await createSelfHostedGatewaysClient(apiFetch).listGateways();
    expect(gateway.url).toBe('');
  });

  it('reads connectivity as the only status, never provisioning or failed', async () => {
    const { apiFetch } = recording(() => ({
      list: [
        { id: 'a', isActive: true },
        { id: 'b', isActive: false },
        { id: 'c' },
      ],
    }));
    const gateways = await createSelfHostedGatewaysClient(apiFetch).listGateways();
    expect(gateways.map((g) => g.status)).toEqual(['active', 'inactive', 'inactive']);
  });

  it('defaults a missing or unknown kind to regular, and a missing name to the id', async () => {
    const { apiFetch } = recording(() => ({
      list: [
        { id: 'a' },
        { id: 'b', functionalityType: 'mystery' },
        { id: 'c', functionalityType: 'ai' },
      ],
    }));
    const gateways = await createSelfHostedGatewaysClient(apiFetch).listGateways();
    expect(gateways.map((g) => g.type)).toEqual(['regular', 'regular', 'ai']);
    expect(gateways[0].name).toBe('a');
  });

  it('only marks a gateway default when the property is exactly "true"', async () => {
    const { apiFetch } = recording(() => ({
      list: [
        { id: 'a', properties: { isDefault: 'true' } },
        { id: 'b', properties: { isDefault: 'false' } },
        { id: 'c', properties: { isDefault: true } },
        { id: 'd' },
      ],
    }));
    const gateways = await createSelfHostedGatewaysClient(apiFetch).listGateways();
    expect(gateways.map((g) => g.isDefault)).toEqual([true, false, false, false]);
  });

  it('is empty when the server returns nothing', async () => {
    const { apiFetch } = recording(() => undefined);
    expect(await createSelfHostedGatewaysClient(apiFetch).listGateways()).toEqual([]);
  });
});

describe('generateToken', () => {
  it('posts to the gateway tokens route and returns the token', async () => {
    const { apiFetch, calls } = recording(() => ({ token: 'tok-1' }));
    const token = await createSelfHostedGatewaysClient(apiFetch).generateToken('dev edge');
    expect(token).toBe('tok-1');
    expect(calls[0]).toEqual({ method: 'POST', path: '/gateways/dev%20edge/tokens', body: {} });
  });

  it('throws when the response carries no token', async () => {
    const { apiFetch } = recording(() => ({}));
    await expect(createSelfHostedGatewaysClient(apiFetch).generateToken('g')).rejects.toThrow(
      /did not return a token/
    );
  });
});

describe('createGateway', () => {
  const answerCreateAndToken = (call: Call) =>
    call.path === '/gateways' && call.method === 'POST'
      ? { id: 'dev-edge-one', displayName: 'Edge One', endpoints: [PENDING_ENDPOINT] }
      : { token: 'tok-1' };

  it('sends the derived handle, the pending endpoint, and the environment in properties', async () => {
    const { apiFetch, calls } = recording(answerCreateAndToken);
    await createSelfHostedGatewaysClient(apiFetch).createGateway(input());
    expect(calls[0].body).toEqual({
      id: 'dev-edge-one',
      displayName: 'Edge One',
      description: undefined,
      endpoints: [PENDING_ENDPOINT],
      functionalityType: 'regular',
      isCritical: false,
      properties: { environment: 'dev' },
    });
  });

  it('uses the given url as the endpoint, trimmed', async () => {
    const { apiFetch, calls } = recording(answerCreateAndToken);
    await createSelfHostedGatewaysClient(apiFetch).createGateway(input({ url: '  https://gw.example.com ' }));
    expect((calls[0].body as { endpoints: string[] }).endpoints).toEqual(['https://gw.example.com']);
  });

  it('sends the version only when one is set', async () => {
    const withVersion = recording(answerCreateAndToken);
    await createSelfHostedGatewaysClient(withVersion.apiFetch).createGateway(input({ version: '1.2' }));
    expect(withVersion.calls[0].body).toMatchObject({ version: '1.2' });

    const without = recording(answerCreateAndToken);
    await createSelfHostedGatewaysClient(without.apiFetch).createGateway(input());
    expect(without.calls[0].body).not.toHaveProperty('version');
  });

  it('marks the default in properties only when asked', async () => {
    const asked = recording(answerCreateAndToken);
    await createSelfHostedGatewaysClient(asked.apiFetch).createGateway(input({ isDefault: true }));
    expect((asked.calls[0].body as { properties: unknown }).properties).toEqual({
      environment: 'dev',
      isDefault: 'true',
    });

    const notAsked = recording(answerCreateAndToken);
    await createSelfHostedGatewaysClient(notAsked.apiFetch).createGateway(input({ isDefault: false }));
    expect((notAsked.calls[0].body as { properties: unknown }).properties).toEqual({ environment: 'dev' });
  });

  it('mints the first token and returns it with the gateway', async () => {
    const { apiFetch, calls } = recording(answerCreateAndToken);
    const result = await createSelfHostedGatewaysClient(apiFetch).createGateway(input());
    expect(result.token).toBe('tok-1');
    expect(result.gateway).toMatchObject({ id: 'dev-edge-one', hosting: 'self-hosted', url: '' });
    expect(calls[1]).toMatchObject({ method: 'POST', path: '/gateways/dev-edge-one/tokens' });
  });

  it('keeps the gateway and returns a null token when minting fails', async () => {
    const { apiFetch } = recording((call) => {
      if (call.path.endsWith('/tokens')) throw new Error('boom');
      return { id: 'dev-edge-one' };
    });
    const result = await createSelfHostedGatewaysClient(apiFetch).createGateway(input());
    expect(result.token).toBeNull();
    expect(result.gateway.id).toBe('dev-edge-one');
  });

  it('throws when the create response has no id', async () => {
    const { apiFetch } = recording(() => ({}));
    await expect(createSelfHostedGatewaysClient(apiFetch).createGateway(input())).rejects.toThrow(
      /incomplete response/
    );
  });

  it('clears the default marker on sibling gateways of the same environment and type', async () => {
    const { apiFetch, calls } = recording((call) => {
      if (call.method === 'POST' && call.path === '/gateways') return { id: 'dev-new' };
      if (call.method === 'GET') {
        return {
          list: [
            { id: 'dev-new', displayName: 'New', properties: { environment: 'dev', isDefault: 'true' } },
            { id: 'dev-old', displayName: 'Old', properties: { environment: 'dev', isDefault: 'true' } },
            { id: 'prod-old', displayName: 'Prod', properties: { environment: 'prod', isDefault: 'true' } },
            { id: 'dev-plain', displayName: 'Plain', properties: { environment: 'dev' } },
          ],
        };
      }
      return { token: 't' };
    });
    await createSelfHostedGatewaysClient(apiFetch).createGateway(input({ name: 'New', isDefault: true }));
    const puts = calls.filter((c) => c.method === 'PUT');
    expect(puts).toEqual([
      {
        method: 'PUT',
        path: '/gateways/dev-old',
        body: { displayName: 'Old', properties: { environment: 'dev' } },
      },
    ]);
  });

  it('does not fail the create when clearing other defaults fails', async () => {
    const { apiFetch } = recording((call) => {
      if (call.method === 'POST' && call.path === '/gateways') return { id: 'dev-new' };
      if (call.method === 'GET') throw new Error('list failed');
      return { token: 't' };
    });
    const result = await createSelfHostedGatewaysClient(apiFetch).createGateway(input({ isDefault: true }));
    expect(result.gateway.id).toBe('dev-new');
    expect(result.token).toBe('t');
  });

  it('does not look at other gateways when not marking a default', async () => {
    const { apiFetch, calls } = recording(answerCreateAndToken);
    await createSelfHostedGatewaysClient(apiFetch).createGateway(input());
    expect(calls.some((c) => c.method === 'GET')).toBe(false);
  });
});

describe('updateGateway', () => {
  it('sends only the editable fields', async () => {
    const { apiFetch, calls } = recording(() => ({ list: [] }));
    await createSelfHostedGatewaysClient(apiFetch).updateGateway(
      'dev-edge',
      input({ description: 'd', isDefault: true })
    );
    expect(calls[0]).toEqual({
      method: 'PUT',
      path: '/gateways/dev-edge',
      body: { displayName: 'Edge One', description: 'd', properties: { environment: 'dev', isDefault: 'true' } },
    });
  });

  it('leaves the other gateways alone when not marking a default', async () => {
    const { apiFetch, calls } = recording(() => undefined);
    await createSelfHostedGatewaysClient(apiFetch).updateGateway('dev-edge', input());
    expect(calls).toHaveLength(1);
  });

  it('swallows a failure to clear other defaults', async () => {
    const { apiFetch } = recording((call) => {
      if (call.method === 'GET') throw new Error('nope');
      return undefined;
    });
    await expect(
      createSelfHostedGatewaysClient(apiFetch).updateGateway('dev-edge', input({ isDefault: true }))
    ).resolves.toBeUndefined();
  });
});

describe('deleteGateway', () => {
  it('deletes the encoded gateway route', async () => {
    const apiFetch = vi.fn(async () => undefined) as unknown as ApiFetch;
    await createSelfHostedGatewaysClient(apiFetch).deleteGateway('dev edge');
    expect(apiFetch).toHaveBeenCalledWith('DELETE', '/gateways/dev%20edge');
  });
});
