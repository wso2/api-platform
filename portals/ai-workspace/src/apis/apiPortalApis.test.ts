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

import { describe, it, expect, vi, beforeEach } from 'vitest';

import { get } from '../clients/choreoApiClient';

vi.mock('../clients/choreoApiClient', () => ({ get: vi.fn() }));

vi.mock('../utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

// API_PORTAL_URL is read at module scope, so each case re-imports the module
// under test with the configured value it wants.
const configuredUrl = vi.hoisted(() => ({ value: '' }));
vi.mock('../config.env', () => ({
  get API_PORTAL_URL() {
    return configuredUrl.value;
  },
}));

const BASE = 'https://control-plane.test';
const PORTAL_ID = 'default';

/** An error shaped the way `getHttpStatus` reads it. */
const httpError = (status: number) => Object.assign(new Error('request failed'), { status });

async function loadModule(configured: string) {
  configuredUrl.value = configured;
  vi.resetModules();
  return import('./apiPortalApis');
}

beforeEach(() => {
  vi.mocked(get).mockReset();
});

describe('getApiPortal', () => {
  it('reads the registration by handle, URL-encoding the id', async () => {
    const portal = { id: 'a/b', handle: 'a/b', name: 'Portal', status: 'ACTIVE' };
    vi.mocked(get).mockResolvedValue(portal);

    const { getApiPortal } = await loadModule('');
    const result = await getApiPortal('a/b', BASE);

    expect(get).toHaveBeenCalledWith('/api-portals/a%2Fb', undefined, BASE);
    expect(result).toBe(portal);
  });

  it('propagates a failed read rather than swallowing it', async () => {
    vi.mocked(get).mockRejectedValue(httpError(500));

    const { getApiPortal } = await loadModule('');
    await expect(getApiPortal(PORTAL_ID, BASE)).rejects.toThrow('request failed');
  });
});

describe('resolveApiPortalUrl', () => {
  it('prefers the organization registration URL over the configured one', async () => {
    vi.mocked(get).mockResolvedValue({ url: 'https://registered.example.com' });

    const { resolveApiPortalUrl } = await loadModule('https://configured.example.com');

    await expect(resolveApiPortalUrl(PORTAL_ID, BASE)).resolves.toBe(
      'https://registered.example.com'
    );
  });

  it('trims surrounding whitespace off the registered URL', async () => {
    vi.mocked(get).mockResolvedValue({ url: '  https://registered.example.com  ' });

    const { resolveApiPortalUrl } = await loadModule('');

    await expect(resolveApiPortalUrl(PORTAL_ID, BASE)).resolves.toBe(
      'https://registered.example.com'
    );
  });

  it('falls back to the configured URL when the registration carries no url', async () => {
    vi.mocked(get).mockResolvedValue({ id: PORTAL_ID, handle: PORTAL_ID, status: 'ACTIVE' });

    const { resolveApiPortalUrl } = await loadModule('https://configured.example.com');

    await expect(resolveApiPortalUrl(PORTAL_ID, BASE)).resolves.toBe(
      'https://configured.example.com'
    );
  });

  it('falls back to the configured URL when the registration url is blank', async () => {
    vi.mocked(get).mockResolvedValue({ url: '   ' });

    const { resolveApiPortalUrl } = await loadModule('https://configured.example.com');

    await expect(resolveApiPortalUrl(PORTAL_ID, BASE)).resolves.toBe(
      'https://configured.example.com'
    );
  });

  // A viewer holding `ap:mcp_proxy:read` but not `ap:api_portal:read` is refused
  // this lookup. That is the documented reason `api_portal_url` exists: the
  // configured address is what such a viewer gets, not an error and not nothing.
  it.each([401, 403, 404])(
    'falls back to the configured URL on %i without throwing',
    async (status) => {
      vi.mocked(get).mockRejectedValue(httpError(status));

      const { resolveApiPortalUrl } = await loadModule('https://configured.example.com');

      await expect(resolveApiPortalUrl(PORTAL_ID, BASE)).resolves.toBe(
        'https://configured.example.com'
      );
    }
  );

  it('falls back to the configured URL on an unexpected server error', async () => {
    vi.mocked(get).mockRejectedValue(httpError(500));

    const { resolveApiPortalUrl } = await loadModule('https://configured.example.com');

    await expect(resolveApiPortalUrl(PORTAL_ID, BASE)).resolves.toBe(
      'https://configured.example.com'
    );
  });

  it('returns undefined when neither source supplies a URL, so no link is shown', async () => {
    vi.mocked(get).mockRejectedValue(httpError(403));

    const { resolveApiPortalUrl } = await loadModule('   ');

    await expect(resolveApiPortalUrl(PORTAL_ID, BASE)).resolves.toBeUndefined();
  });
});
