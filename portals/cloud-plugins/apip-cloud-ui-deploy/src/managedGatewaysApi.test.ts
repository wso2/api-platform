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

import type { ApiFetch } from './hostPort';
import { DEFAULT_GATEWAY_NAME, createManagedGatewaysClient } from './managedGatewaysApi';

describe('listGateways', () => {
  const listing = (list?: unknown[]) => {
    const calls: unknown[][] = [];
    const apiFetch = (async (...args: unknown[]) => {
      calls.push(args);
      return list === undefined ? undefined : { list };
    }) as ApiFetch;
    return { client: createManagedGatewaysClient(apiFetch), calls };
  };

  it('asks only for regular gateways', async () => {
    const { client, calls } = listing([]);
    await client.listGateways();
    expect(calls[0]).toEqual(['GET', '/managed-gateways?functionalityType=regular']);
  });

  it('reads each status the server reports', async () => {
    const { client } = listing([
      { id: 'a', status: 'PROVISIONING' },
      { id: 'b', status: 'FAILED', statusReason: 'no capacity' },
      { id: 'c', status: 'ACTIVE' },
      { id: 'd', status: 'DELETING' },
    ]);
    const states = await client.listGateways();
    expect(states.map((s) => s.status)).toEqual(['provisioning', 'failed', 'active', 'other']);
    expect(states[1].statusReason).toBe('no capacity');
  });

  it('falls back to isActive only when the server reports no status', async () => {
    const { client } = listing([
      { id: 'a', isActive: true },
      { id: 'b', isActive: false },
      { id: 'c', status: 'FAILED', isActive: true },
    ]);
    const states = await client.listGateways();
    expect(states.map((s) => s.status)).toEqual(['active', 'other', 'failed']);
  });

  it('defaults the display name to the id and the environment to empty', async () => {
    const { client } = listing([{ id: 'dev-default' }]);
    expect(await client.listGateways()).toEqual([
      { id: 'dev-default', environment: '', displayName: 'dev-default', status: 'other', statusReason: undefined },
    ]);
  });

  it('is empty when the server returns nothing', async () => {
    const { client } = listing(undefined);
    expect(await client.listGateways()).toEqual([]);
  });
});

describe('createDefaultGateway', () => {
  it('creates the default regular managed gateway in the environment', async () => {
    const calls: unknown[][] = [];
    const apiFetch = (async (...args: unknown[]) => {
      calls.push(args);
      return { id: 'dev-default-cloud-gateway' };
    }) as ApiFetch;
    const result = await createManagedGatewaysClient(apiFetch).createDefaultGateway('dev');
    expect(result).toEqual({ id: 'dev-default-cloud-gateway' });
    expect(calls[0]).toEqual([
      'POST',
      '/managed-gateways',
      {
        displayName: DEFAULT_GATEWAY_NAME,
        description: 'Default API Platform Cloud Gateway',
        environment: 'dev',
        functionalityType: 'regular',
        isCritical: false,
        isDefault: true,
      },
    ]);
  });

  it('returns no id when the server answers with no body', async () => {
    const apiFetch = (async () => undefined) as ApiFetch;
    expect(await createManagedGatewaysClient(apiFetch).createDefaultGateway('dev')).toEqual({ id: undefined });
  });
});
