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

/** Loads the config module fresh, as the browser does once per page, with the BFF's runtime values in place. */
async function loadConfig(runtime: Record<string, string>) {
  window.__RUNTIME_CONFIG__ = runtime;
  vi.resetModules();
  return import('./config.env');
}

afterEach(() => {
  delete window.__RUNTIME_CONFIG__;
});

describe('MULTI_HUB_PUBLISHING', () => {
  it.each([
    // [api_portal_enabled, api_portal_integration_enabled, hub list and per-hub pages]
    ['true', 'true', true],
    ['false', 'true', true],
    ['true', 'false', false],
    ['false', 'false', false],
  ])('with api_portal_enabled=%s and api_portal_integration_enabled=%s the hub pages are %s', async (portal, hubs, expected) => {
    const config = await loadConfig({
      APIP_AIW_API_PORTAL_ENABLED: portal,
      APIP_AIW_FEATURE_FLAGS_API_PORTAL_INTEGRATION_ENABLED: hubs,
    });

    expect(config.MULTI_HUB_PUBLISHING).toBe(expected);
  });

  it('is off for a deployment that sets neither, leaving the single-hub dialog as it is', async () => {
    const config = await loadConfig({});

    expect(config.API_PORTAL_ENABLED).toBe(false);
    expect(config.isFeatureEnabled('api-portal-integration')).toBe(false);
    expect(config.MULTI_HUB_PUBLISHING).toBe(false);
  });

  it('is off where the single-hub dialog is on but the flag is absent', async () => {
    const config = await loadConfig({ APIP_AIW_API_PORTAL_ENABLED: 'true' });

    expect(config.API_PORTAL_ENABLED).toBe(true);
    expect(config.MULTI_HUB_PUBLISHING).toBe(false);
  });
});
