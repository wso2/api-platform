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
  latestRegularVersion,
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

  it('registers the default regular self-hosted gateway marked as default', async () => {
    const { client, calls } = recording(ok);
    await client.createDefaultGateway('dev', '1.2');
    expect(calls[0]).toEqual({
      method: 'POST',
      path: '/gateways',
      body: {
        id: 'dev-default-self-hosted-gateway',
        displayName: DEFAULT_SELF_HOSTED_NAME,
        description: 'Default API Platform Self-Hosted Gateway',
        endpoints: [PENDING_ENDPOINT],
        functionalityType: 'regular',
        isCritical: false,
        version: '1.2',
        properties: { environment: 'dev', isDefault: 'true' },
      },
    });
  });

  it('omits the version when the host has none', async () => {
    const { client, calls } = recording(ok);
    await client.createDefaultGateway('dev');
    expect(calls[0].body).not.toHaveProperty('version');
  });

  it('keeps the handle within the 40-character column without a trailing hyphen', async () => {
    const { client, calls } = recording(ok);
    await client.createDefaultGateway('a-very-long-environment-name-indeed');
    const id = (calls[0].body as { id: string }).id;
    expect(id.length).toBeLessThanOrEqual(40);
    expect(id.startsWith('a-very-long-environment-name-indeed-')).toBe(true);
    expect(id.endsWith('-')).toBe(false);
  });

  it('returns the id and the first token', async () => {
    const { client, calls } = recording(ok);
    expect(await client.createDefaultGateway('dev')).toEqual({
      id: 'dev-default-self-hosted-gateway',
      token: 'tok-1',
    });
    expect(calls[1]).toEqual({
      method: 'POST',
      path: '/gateways/dev-default-self-hosted-gateway/tokens',
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
