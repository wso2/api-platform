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
    expect(calls[0]).toMatchObject({ method: 'GET', path: '/self-hosted-gateways' });
  });

  it('filters by the requested kinds', async () => {
    const { apiFetch, calls } = recording(() => ({ list: [] }));
    await createSelfHostedGatewaysClient(apiFetch).listGateways(['regular', 'event']);
    expect(calls[0].path).toBe('/self-hosted-gateways?functionalityType=regular,event');
  });

  it('maps a record to a self-hosted gateway, in its bound environment and never the default', async () => {
    const { apiFetch } = recording(() => ({
      list: [
        {
          id: 'dev-edge-one',
          displayName: 'Edge One',
          functionalityType: 'event',
          endpoints: ['https://edge.example.com'],
          isActive: true,
          version: '1.2',
          environment: 'dev',
          isDefault: true,
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
      isDefault: false,
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
    expect(calls[0]).toEqual({ method: 'POST', path: '/self-hosted-gateways/dev%20edge/tokens', body: {} });
  });

  it('throws when the response carries no token', async () => {
    const { apiFetch } = recording(() => ({}));
    await expect(createSelfHostedGatewaysClient(apiFetch).generateToken('g')).rejects.toThrow(
      /did not return a token/
    );
  });
});

describe('hasActiveToken', () => {
  it('is true when an active token exists', async () => {
    const { apiFetch, calls } = recording(() => ({ list: [{ id: 't1', status: 'active' }] }));
    expect(await createSelfHostedGatewaysClient(apiFetch).hasActiveToken('g')).toBe(true);
    expect(calls[0]).toMatchObject({ method: 'GET', path: '/self-hosted-gateways/g/tokens' });
  });

  it('is false when there are none, or only revoked ones', async () => {
    const none = recording(() => ({ list: [] }));
    expect(await createSelfHostedGatewaysClient(none.apiFetch).hasActiveToken('g')).toBe(false);
    const revoked = recording(() => ({ list: [{ id: 't0', status: 'revoked' }] }));
    expect(await createSelfHostedGatewaysClient(revoked.apiFetch).hasActiveToken('g')).toBe(false);
  });
});

describe('reconfigureToken', () => {
  it('revokes every active token first, then mints a new one', async () => {
    const { apiFetch, calls } = recording((call) => {
      if (call.method === 'GET') {
        return {
          list: [
            { id: 't1', status: 'active' },
            { id: 't2', status: 'active' },
            { id: 't0', status: 'revoked' },
          ],
        };
      }
      return call.method === 'POST' ? { token: 'new' } : undefined;
    });
    expect(await createSelfHostedGatewaysClient(apiFetch).reconfigureToken('g')).toBe('new');
    expect(calls.map((c) => `${c.method} ${c.path}`)).toEqual([
      'GET /self-hosted-gateways/g/tokens',
      'DELETE /self-hosted-gateways/g/tokens/t1',
      'DELETE /self-hosted-gateways/g/tokens/t2',
      'POST /self-hosted-gateways/g/tokens',
    ]);
  });

  it('mints a token when there is nothing to revoke', async () => {
    const { apiFetch, calls } = recording((call) => (call.method === 'GET' ? { list: [] } : { token: 'new' }));
    expect(await createSelfHostedGatewaysClient(apiFetch).reconfigureToken('g')).toBe('new');
    expect(calls.map((c) => c.method)).toEqual(['GET', 'POST']);
  });

  it('mints nothing when a revoke fails', async () => {
    const { apiFetch, calls } = recording((call) => {
      if (call.method === 'GET') return { list: [{ id: 't1', status: 'active' }] };
      if (call.method === 'DELETE') throw new Error('revoke failed');
      return { token: 'new' };
    });
    await expect(createSelfHostedGatewaysClient(apiFetch).reconfigureToken('g')).rejects.toThrow(/revoke failed/);
    expect(calls.some((c) => c.method === 'POST')).toBe(false);
  });
});

describe('createGateway', () => {
  const answerCreateAndToken = (call: Call) =>
    call.path === '/self-hosted-gateways' && call.method === 'POST'
      ? { id: 'dev-edge-one', displayName: 'Edge One', endpoints: [PENDING_ENDPOINT], environment: 'dev' }
      : { token: 'tok-1' };

  it('sends the environment as its own field, with the pending endpoint', async () => {
    const { apiFetch, calls } = recording(answerCreateAndToken);
    await createSelfHostedGatewaysClient(apiFetch).createGateway(input());
    expect(calls[0]).toEqual({
      method: 'POST',
      path: '/self-hosted-gateways',
      body: {
        displayName: 'Edge One',
        description: undefined,
        environment: 'dev',
        functionalityType: 'regular',
        endpoints: [PENDING_ENDPOINT],
      },
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

  it('never asks for the default', async () => {
    const { apiFetch, calls } = recording(answerCreateAndToken);
    await createSelfHostedGatewaysClient(apiFetch).createGateway(input({ isDefault: true }));
    expect(calls[0].body).not.toHaveProperty('isDefault');
  });

  it('mints the first token and returns it with the gateway', async () => {
    const { apiFetch, calls } = recording(answerCreateAndToken);
    const result = await createSelfHostedGatewaysClient(apiFetch).createGateway(input());
    expect(result.token).toBe('tok-1');
    expect(result.gateway).toMatchObject({ id: 'dev-edge-one', hosting: 'self-hosted', url: '', environmentId: 'dev' });
    expect(calls[1]).toMatchObject({ method: 'POST', path: '/self-hosted-gateways/dev-edge-one/tokens' });
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
});

describe('updateGateway', () => {
  it('sends only the editable fields', async () => {
    const { apiFetch, calls } = recording(() => undefined);
    await createSelfHostedGatewaysClient(apiFetch).updateGateway(
      'dev-edge',
      input({ description: 'd', isDefault: true })
    );
    expect(calls).toEqual([
      {
        method: 'PUT',
        path: '/self-hosted-gateways/dev-edge',
        body: { displayName: 'Edge One', description: 'd' },
      },
    ]);
  });
});

describe('deleteGateway', () => {
  it('deletes the encoded gateway route', async () => {
    const apiFetch = vi.fn(async () => undefined) as unknown as ApiFetch;
    await createSelfHostedGatewaysClient(apiFetch).deleteGateway('dev edge');
    expect(apiFetch).toHaveBeenCalledWith('DELETE', '/self-hosted-gateways/dev%20edge');
  });
});
