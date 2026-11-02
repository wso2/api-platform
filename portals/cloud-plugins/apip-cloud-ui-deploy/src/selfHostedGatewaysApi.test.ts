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

import { describe, expect, it } from 'vitest';

import type { ApiFetch, GatewayVersionEntry } from './hostPort';
import {
  DEFAULT_SELF_HOSTED_NAME,
  PENDING_ENDPOINT,
  createSelfHostedGatewaysClient,
  deriveHandleName,
  latestRegularVersion,
  latestVersionFor,
} from './selfHostedGatewaysApi';

describe('latestRegularVersion', () => {
  const versions: GatewayVersionEntry[] = [
    { type: 'event', version: '1.0', channel: 'LTS' },
    { type: 'regular', version: '2026.05.13', channel: 'STS' },
    { type: 'regular', version: '1.1', channel: 'LTS' },
  ];

  it('is the first regular entry, whatever precedes it', () => {
    expect(latestRegularVersion(versions)?.version).toBe('2026.05.13');
  });

  it('is undefined when there is no regular entry or no catalog', () => {
    expect(latestRegularVersion([{ type: 'event', version: '1.0', channel: 'LTS' }])).toBeUndefined();
    expect(latestRegularVersion([])).toBeUndefined();
    expect(latestRegularVersion(undefined)).toBeUndefined();
  });
});

describe('latestVersionFor', () => {
  it('is the first entry of the requested type', () => {
    const versions: GatewayVersionEntry[] = [
      { type: 'regular', version: '1.2', channel: 'LTS' },
      { type: 'ai', version: '1.0', channel: 'LTS' },
    ];
    expect(latestVersionFor(versions, 'ai')?.version).toBe('1.0');
    expect(latestVersionFor(versions, 'regular')?.version).toBe('1.2');
  });
});

describe('createDefaultGateway', () => {
  const recording = (respond: (path: string) => unknown) => {
    const calls: { method: string; path: string; body?: unknown }[] = [];
    const apiFetch = (async (method: string, path: string, body?: unknown) => {
      calls.push({ method, path, body });
      return respond(path);
    }) as ApiFetch;
    return { client: createSelfHostedGatewaysClient(apiFetch), calls };
  };
  const ok = (path: string) => (path.endsWith('/tokens') ? { token: 'tok-1' } : { id: 'dev-default-self-hosted-gateway' });

  it('registers the default regular self-hosted gateway in its environment', async () => {
    const { client, calls } = recording(ok);
    await client.createDefaultGateway('dev', '1.2');
    expect(calls[0]).toEqual({
      method: 'POST',
      path: '/self-hosted-gateways',
      body: {
        displayName: DEFAULT_SELF_HOSTED_NAME,
        handle: 'default-self-hosted-gateway',
        description: 'Default API Platform Self-Hosted Gateway',
        environment: 'dev',
        endpoints: [PENDING_ENDPOINT],
        functionalityType: 'regular',
        version: '1.2',
      },
    });
  });

  it('omits the version when the host has none', async () => {
    const { client, calls } = recording(ok);
    await client.createDefaultGateway('dev');
    expect(calls[0].body).not.toHaveProperty('version');
  });

  it('shortens the name so <environment>-<name> fits the 40-character column', async () => {
    const { client, calls } = recording(ok);
    const environment = 'a-very-long-environment';
    await client.createDefaultGateway(environment, undefined, 'ai');
    const handle = (calls[0].body as { handle: string }).handle;
    expect(`${environment}-${handle}`.length).toBeLessThanOrEqual(40);
    expect(handle.endsWith('-')).toBe(false);
    expect(handle.startsWith('default-')).toBe(true);
  });

  it('keeps shortened names of different gateways distinct', () => {
    const environment = 'production-eu-west';
    expect(deriveHandleName('Default Self-Hosted Gateway', environment)).not.toBe(
      deriveHandleName('Default AI Self-Hosted Gateway', environment)
    );
  });

  it('leaves a name that fits as its plain slug', () => {
    expect(deriveHandleName('Default Self-Hosted Gateway', 'dev')).toBe('default-self-hosted-gateway');
  });

  it('registers an AI gateway, named for its type, when asked for one', async () => {
    const { client, calls } = recording(ok);
    await client.createDefaultGateway('dev', '1.0', 'ai');
    expect(calls[0].body).toMatchObject({
      displayName: 'Default AI Self-Hosted Gateway',
      functionalityType: 'ai',
      version: '1.0',
    });
  });

  it('returns the id and the first token', async () => {
    const { client, calls } = recording(ok);
    expect(await client.createDefaultGateway('dev')).toEqual({
      id: 'dev-default-self-hosted-gateway',
      token: 'tok-1',
    });
    expect(calls[1]).toEqual({
      method: 'POST',
      path: '/self-hosted-gateways/dev-default-self-hosted-gateway/tokens',
      body: {},
    });
  });

  it('returns a null token when minting fails after the create', async () => {
    const { client } = recording((path) => {
      if (path.endsWith('/tokens')) throw new Error('boom');
      return { id: 'g1' };
    });
    expect(await client.createDefaultGateway('dev')).toEqual({ id: 'g1', token: null });
  });

  it('throws when the create response has no id', async () => {
    const { client } = recording(() => ({}));
    await expect(client.createDefaultGateway('dev')).rejects.toThrow(/incomplete response/);
  });
});

describe('generateToken', () => {
  it('throws when the response carries no token', async () => {
    const client = createSelfHostedGatewaysClient((async () => ({})) as ApiFetch);
    await expect(client.generateToken('g')).rejects.toThrow(/did not return a token/);
  });

  it('returns the token the server minted', async () => {
    const client = createSelfHostedGatewaysClient((async () => ({ token: 't' })) as ApiFetch);
    expect(await client.generateToken('g')).toBe('t');
  });
});

describe('reconfigureToken', () => {
  it('revokes every active token before minting a new one', async () => {
    const calls: string[] = [];
    const client = createSelfHostedGatewaysClient((async (method: string, path: string) => {
      calls.push(`${method} ${path}`);
      if (method === 'GET') {
        return { list: [{ id: 't1', status: 'active' }, { id: 't2', status: 'active' }, { id: 't0', status: 'revoked' }] };
      }
      return method === 'POST' ? { token: 'new' } : undefined;
    }) as ApiFetch);
    expect(await client.reconfigureToken('g')).toBe('new');
    expect(calls).toEqual([
      'GET /self-hosted-gateways/g/tokens',
      'DELETE /self-hosted-gateways/g/tokens/t1',
      'DELETE /self-hosted-gateways/g/tokens/t2',
      'POST /self-hosted-gateways/g/tokens',
    ]);
  });
});

describe('hasActiveToken', () => {
  it('is false when every token is revoked', async () => {
    const client = createSelfHostedGatewaysClient((async () => ({
      list: [{ id: 't0', status: 'revoked' }],
    })) as ApiFetch);
    expect(await client.hasActiveToken('g')).toBe(false);
  });
});
