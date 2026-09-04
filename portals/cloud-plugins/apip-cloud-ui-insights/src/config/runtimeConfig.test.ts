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

import { afterEach, describe, expect, it, vi } from 'vitest';

const loadRuntimeConfig = async () => {
  vi.resetModules();
  return (await import('./runtimeConfig')).insightsRuntimeConfig;
};

afterEach(() => {
  delete window.config;
  delete window.__RUNTIME_CONFIG__;
  vi.resetModules();
  vi.unstubAllEnvs();
});

describe('insightsRuntimeConfig', () => {
  it('leaves moesifAppUrl undefined when nothing is configured', async () => {
    const config = await loadRuntimeConfig();

    expect(config.moesifAppUrl).toBeUndefined();
  });

  it('accepts an allowlisted moesifAppUrl from window runtime config', async () => {
    window.__RUNTIME_CONFIG__ = {
      moesifAppUrl: 'https://www.moesif.com/wrap',
    };

    const config = await loadRuntimeConfig();

    expect(config.moesifAppUrl).toBe('https://www.moesif.com');
  });

  it('rejects a non-allowlisted moesifAppUrl without falling back to web-dev', async () => {
    window.__RUNTIME_CONFIG__ = {
      moesifAppUrl: 'https://evil.example.com',
    };

    const config = await loadRuntimeConfig();

    expect(config.moesifAppUrl).toBeUndefined();
  });

  it('reports configured when an allowlisted Moesif origin is present', async () => {
    window.__RUNTIME_CONFIG__ = {
      APIP_AIW_MOESIF_WEB_URL: 'https://web-dev.moesif.com',
    };
    vi.resetModules();
    const { isInsightsMoesifConfigured } = await import('./runtimeConfig');
    expect(isInsightsMoesifConfigured()).toBe(true);
  });

  it('reports unconfigured when Moesif origin is missing', async () => {
    vi.resetModules();
    const { isInsightsMoesifConfigured } = await import('./runtimeConfig');
    expect(isInsightsMoesifConfigured()).toBe(false);
  });
});
