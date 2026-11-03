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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { afterEach, describe, expect, it, vi } from 'vitest';

import { fetchEntitlementSummary } from './entitlementsApi';

/**
 * The summary is read through the console's proxy, beside the host's billing
 * read, so it has to follow the proxy wherever the deployment puts it: `/proxy`
 * behind the BFF, `/api-management/proxy` behind a gateway that serves the
 * console under a path. A Platform API URL that is not a same-origin path has
 * no billing route, so the read stays on the BFF's default.
 */
const stubRuntimeConfig = (config: Record<string, string>) =>
  vi.stubGlobal('window', { __RUNTIME_CONFIG__: config });

const okResponse = (body: unknown) => ({ ok: true, json: async () => body }) as Response;

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('fetchEntitlementSummary', () => {
  it.each([
    [{ platformApiBaseUrl: '/proxy' }, '/proxy/billing-entitlement/summary?product=api-platform'],
    [
      { platformApiBaseUrl: '/api-management/proxy/' },
      '/api-management/proxy/billing-entitlement/summary?product=api-platform',
    ],
    [
      { PLATFORM_API_BASE_URL: '/api-management/proxy' },
      '/api-management/proxy/billing-entitlement/summary?product=api-platform',
    ],
    [{}, '/proxy/billing-entitlement/summary?product=api-platform'],
    [
      { platformApiBaseUrl: 'https://localhost:9243' },
      '/proxy/billing-entitlement/summary?product=api-platform',
    ],
    [
      { platformApiBaseUrl: '//platform.example.com' },
      '/proxy/billing-entitlement/summary?product=api-platform',
    ],
  ])('reads the summary under the proxy base for %j', async (config, expected) => {
    stubRuntimeConfig(config);
    const fetchMock = vi.fn().mockResolvedValue(okResponse({ entitlements: [] }));
    vi.stubGlobal('fetch', fetchMock);

    await fetchEntitlementSummary();

    expect(fetchMock).toHaveBeenCalledWith(expected, expect.anything());
  });

  it('resolves null when the read fails, so a failed read never locks anyone out', async () => {
    stubRuntimeConfig({ platformApiBaseUrl: '/api-management/proxy' });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 404 } as Response));

    await expect(fetchEntitlementSummary()).resolves.toBeNull();
  });
});
