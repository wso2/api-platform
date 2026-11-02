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

const stubFetch = () => {
  const fetchMock = vi.fn(async () => new Response('{}', { status: 200 }));
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
};

describe('fetchEntitlementSummary', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('reads through the proxy at the root when no base path is configured', async () => {
    vi.stubGlobal('window', { __RUNTIME_CONFIG__: {} });
    const fetchMock = stubFetch();
    await fetchEntitlementSummary();
    expect(fetchMock.mock.calls[0]?.[0]).toMatch(/^\/proxy\/billing-entitlement\/summary\?/);
  });

  it('reads through the proxy under the console base path', async () => {
    vi.stubGlobal('window', { __RUNTIME_CONFIG__: { appBasePath: '/api-management' } });
    const fetchMock = stubFetch();
    await fetchEntitlementSummary();
    expect(fetchMock.mock.calls[0]?.[0]).toMatch(
      /^\/api-management\/proxy\/billing-entitlement\/summary\?/,
    );
  });
});
