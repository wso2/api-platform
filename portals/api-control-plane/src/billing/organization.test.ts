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

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { runtimeConfig } from '../config/runtime';

import { getBillingOrganization, resetBillingOrganization } from './organization';

/**
 * Reading the billing organization activates the subscription as a server-side
 * side effect, and that activation is what provisions the org's gateway. So the
 * property under test is not "it fetches" but "it fetches once": the activation
 * hook and the trial badge both ask for this record, and two unsynchronised
 * reads would race the activation.
 */
const stubBillingEnabled = (enabled: boolean) =>
  vi.spyOn(runtimeConfig, 'billingProxyEnabled', 'get').mockReturnValue(enabled);

const okResponse = (body: unknown) => ({ ok: true, json: async () => body }) as Response;

describe('getBillingOrganization', () => {
  beforeEach(() => {
    resetBillingOrganization();
    stubBillingEnabled(true);
  });

  afterEach(() => {
    vi.restoreAllMocks();
    resetBillingOrganization();
  });

  it('reads the record once however many callers ask for it', async () => {
    const fetchMock = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValue(okResponse({ subscription: { status: 'trial' } }));

    const [first, second] = await Promise.all([getBillingOrganization(), getBillingOrganization()]);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(first).toEqual({ subscription: { status: 'trial' } });
    expect(second).toBe(first);
  });

  it('keeps serving the same record to a caller that asks after it resolved', async () => {
    const fetchMock = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValue(okResponse({ subscription: { status: 'active' } }));

    await getBillingOrganization();
    await getBillingOrganization();

    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it('resolves null without calling out when no billing upstream is configured', async () => {
    stubBillingEnabled(false);
    const fetchMock = vi.spyOn(globalThis, 'fetch');

    await expect(getBillingOrganization()).resolves.toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('lets a later caller retry after a failed read, rather than caching the rejection', async () => {
    const fetchMock = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce({ ok: false, status: 503 } as Response)
      .mockResolvedValueOnce(okResponse({ subscription: { status: 'active' } }));

    await expect(getBillingOrganization()).rejects.toThrow();
    await expect(getBillingOrganization()).resolves.toEqual({
      subscription: { status: 'active' },
    });
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });
});
