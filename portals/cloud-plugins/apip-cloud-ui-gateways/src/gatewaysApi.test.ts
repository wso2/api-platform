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

import { createGatewaysClient } from './gatewaysApi';
import type { ApiFetch } from './hostPort';

/** An apiFetch that answers every list call with the given records. */
const servingGateways = (list: unknown[]): ApiFetch =>
  (async () => ({ list })) as ApiFetch;

const first = async (list: unknown[]) =>
  (await createGatewaysClient(servingGateways(list)).listGateways())[0];

describe('gateway status', () => {
  it('reads each status the server reports', async () => {
    const cases: [string, string][] = [
      ['PROVISIONING', 'provisioning'],
      ['FAILED', 'failed'],
      ['ACTIVE', 'active'],
      ['DISCONNECTED', 'inactive'],
    ];
    for (const [reported, want] of cases) {
      // isActive deliberately contradicts the status on two of these: the
      // status is the whole answer, and a gateway being built has nothing
      // connected yet.
      const gateway = await first([{ id: 'dev-default', status: reported, isActive: false }]);
      expect(gateway.status).toBe(want);
    }
  });

  it('carries the reason for a failure, and nothing otherwise', async () => {
    const failed = await first([
      { id: 'dev-default', status: 'FAILED', statusReason: 'The gateway could not be released to its environment.' },
    ]);
    expect(failed.statusReason).toBe('The gateway could not be released to its environment.');

    const active = await first([{ id: 'dev-default', status: 'ACTIVE' }]);
    expect(active.statusReason).toBeUndefined();
  });

  it('falls back to connectivity when the server reports no status', async () => {
    // A server that predates the status field still answers the connectivity
    // half, which is what this read before the field existed.
    expect((await first([{ id: 'dev-default', isActive: true }])).status).toBe('active');
    expect((await first([{ id: 'dev-default', isActive: false }])).status).toBe('inactive');
    expect((await first([{ id: 'dev-default', status: 'SOMETHING_NEW', isActive: true }])).status).toBe('active');
  });
});
