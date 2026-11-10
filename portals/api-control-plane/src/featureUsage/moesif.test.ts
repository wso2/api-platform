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

import type { FeatureUsageIdentity } from './moesif';

/**
 * Stands in for moesif-browser-js: the boundary this module talks to.
 * `_executeRequest` is the SDK's network write, which the PII filter wraps.
 */
const transport = vi.hoisted(() => vi.fn());
const sdk = vi.hoisted(() => {
  const client: Record<string, unknown> & {
    identifyCompany: ReturnType<typeof vi.fn>;
    identifyUser: ReturnType<typeof vi.fn>;
    init: ReturnType<typeof vi.fn>;
    reset: ReturnType<typeof vi.fn>;
    track: ReturnType<typeof vi.fn>;
  } = {
    _executeRequest: transport,
    identifyCompany: vi.fn(),
    identifyUser: vi.fn(),
    init: vi.fn(),
    reset: vi.fn(),
    track: vi.fn(),
  };
  client.init.mockImplementation(() => client);
  return client;
});
vi.mock('moesif-browser-js', () => ({ default: sdk }));

const ORG = 'b3e1c2d4-7f6a-4e5b-9c8d-0a1b2c3d4e5f';
const USER = '0193e7a2-4c5d-7e6f-8a9b-0c1d2e3f4a5b';
const ENABLED = { featureUsageTrackingEnabled: 'true', moesifPublishableKey: 'publishable-key' };

/** A fresh page load: the module and its runtime config are re-evaluated. */
const load = async (config: Record<string, string> = ENABLED) => {
  vi.resetModules();
  window.__RUNTIME_CONFIG__ = config;
  return import('./moesif');
};

/** Lets the lazy SDK import and the queued calls run. */
const settle = () => new Promise((resolve) => setTimeout(resolve, 0));

const trackCreate = async (identity: FeatureUsageIdentity = { companyId: ORG, userId: USER }) => {
  const { trackFeatureUsage } = await load();
  trackFeatureUsage('apip.manage-apis.rest-api-create-click', { creation_method: 'scratch' }, identity);
  await settle();
};

afterEach(() => {
  vi.clearAllMocks();
  sdk.init.mockImplementation(() => sdk);
  // Each page load wraps the transport again; start every test from the SDK's own.
  sdk._executeRequest = transport;
  delete window.__RUNTIME_CONFIG__;
  localStorage.clear();
});

describe('feature-usage tracking switch', () => {
  it.each([
    ['nothing configured (on-prem)', {}],
    ['enabled without a publishable key', { featureUsageTrackingEnabled: 'true' }],
    ['a key without the switch', { moesifPublishableKey: 'publishable-key' }],
    [
      'explicitly off',
      { featureUsageTrackingEnabled: 'false', moesifPublishableKey: 'publishable-key' },
    ],
  ])('sends nothing and never initialises Moesif with %s', async (_name, config) => {
    const { isFeatureUsageTrackingEnabled, trackFeatureUsage } = await load(config);
    trackFeatureUsage(
      'apip.manage-apis.rest-api-create-click',
      { creation_method: 'scratch' },
      { companyId: ORG },
    );
    await settle();

    expect(isFeatureUsageTrackingEnabled()).toBe(false);
    expect(sdk.init).not.toHaveBeenCalled();
    expect(sdk.track).not.toHaveBeenCalled();
  });
});

describe('trackFeatureUsage', () => {
  it('initialises Moesif once with the publishable key, batching, and no API capture', async () => {
    const { trackFeatureUsage } = await load();
    trackFeatureUsage('apip.manage-apis.rest-api-create-click', { creation_method: 'scratch' }, {});
    trackFeatureUsage('apip.manage-apis.rest-api-create-click', { creation_method: 'contract' }, {});
    await settle();

    expect(sdk.init).toHaveBeenCalledTimes(1);
    const options = sdk.init.mock.calls[0][0];
    expect(options).toMatchObject({
      applicationId: 'publishable-key',
      batchEnabled: true,
      disableGclid: true,
      disableReferrer: true,
      disableUtm: true,
    });
    expect(options.skip()).toBe(true);
  });

  it('sends the taxonomy code as the action, with the taxonomy fields as metadata', async () => {
    await trackCreate();

    expect(sdk.track).toHaveBeenCalledWith('apip.manage-apis.rest-api-create-click', {
      action: 'REST API Creation Click',
      category: 'manage-apis',
      creation_method: 'scratch',
      event_category: 'engagement',
      initiating_product: 'apip',
      platform: 'web',
      tier: 'core',
    });
  });

  it('sends only the dimensions the feature declares', async () => {
    const { trackFeatureUsage } = await load();
    trackFeatureUsage(
      'apip.manage-apis.rest-api-create-click',
      { apiName: 'Payments API', creation_method: 'scratch' } as never,
      {},
    );
    await settle();

    expect(sdk.track.mock.calls[0][1]).not.toHaveProperty('apiName');
  });

  it('never throws into the caller', async () => {
    sdk.track.mockImplementation(() => {
      throw new Error('sdk failure');
    });
    const { trackFeatureUsage } = await load();

    expect(() =>
      trackFeatureUsage('apip.manage-apis.rest-api-create-click', { creation_method: 'scratch' }, {}),
    ).not.toThrow();
    await settle();
  });

  it('drops actions quietly when Moesif fails to initialise', async () => {
    sdk.init.mockImplementation(() => {
      throw new Error('init failure');
    });
    await trackCreate();

    expect(sdk.track).not.toHaveBeenCalled();
  });
});

describe('identity', () => {
  it('identifies the organization, then the user, by ID only', async () => {
    await trackCreate();

    expect(sdk.identifyCompany).toHaveBeenCalledWith(ORG);
    expect(sdk.identifyUser).toHaveBeenCalledWith(USER);
    expect(sdk.identifyCompany.mock.invocationCallOrder[0]).toBeLessThan(
      sdk.identifyUser.mock.invocationCallOrder[0],
    );
    expect(sdk.identifyCompany.mock.invocationCallOrder[0]).toBeLessThan(
      sdk.track.mock.invocationCallOrder[0],
    );
  });

  it('identifies once, not on every action or page load', async () => {
    const { trackFeatureUsage } = await load();
    trackFeatureUsage(
      'apip.manage-apis.rest-api-create-click',
      { creation_method: 'scratch' },
      { companyId: ORG, userId: USER },
    );
    trackFeatureUsage(
      'apip.manage-apis.rest-api-create-click',
      { creation_method: 'scratch' },
      { companyId: ORG, userId: USER },
    );
    await settle();
    await trackCreate(); // the next page load, same user and organization

    expect(sdk.identifyUser).toHaveBeenCalledTimes(1);
    expect(sdk.identifyCompany).toHaveBeenCalledTimes(1);
    expect(sdk.reset).not.toHaveBeenCalled();
    expect(sdk.track).toHaveBeenCalledTimes(3);
  });

  it('resets before identifying a different user or organization on the same browser', async () => {
    await trackCreate();
    await trackCreate({
      companyId: '9a3f4b2c-1d2e-4f5a-8b6c-7d8e9f0a1b2c',
      userId: '5c6d7e8f-9a0b-4c1d-8e2f-3a4b5c6d7e8f',
    });

    expect(sdk.reset).toHaveBeenCalledTimes(1);
    expect(sdk.reset.mock.invocationCallOrder[0]).toBeLessThan(
      sdk.identifyCompany.mock.invocationCallOrder[1],
    );
    expect(sdk.identifyUser).toHaveBeenLastCalledWith('5c6d7e8f-9a0b-4c1d-8e2f-3a4b5c6d7e8f');
  });

  it('never sends an ID that is not a UUID, but still sends the action', async () => {
    await trackCreate({ companyId: 'acme-corp', userId: 'alice@example.com' });

    expect(sdk.identifyCompany).not.toHaveBeenCalled();
    expect(sdk.identifyUser).not.toHaveBeenCalled();
    expect(sdk.track).toHaveBeenCalledTimes(1);
  });
});

describe('PII filter', () => {
  const unsafeAction = {
    action_name: 'apip.manage-apis.rest-api-create-click',
    company_id: ORG,
    metadata: { apiName: 'Payments API', creation_method: 'scratch' },
    request: { uri: 'https://console.example.com/organizations/acme-corp/home?q=secret' },
  };

  it('is installed before Moesif is initialised, so it sees every request', async () => {
    let transportAtInit: unknown;
    sdk.init.mockImplementation(() => {
      transportAtInit = sdk._executeRequest;
      return sdk;
    });
    await trackCreate();

    expect(transportAtInit).toBeTypeOf('function');
    expect(transportAtInit).not.toBe(transport);
  });

  it('hands the SDK’s network write only the filtered payload', async () => {
    await trackCreate();
    const send = sdk._executeRequest as (...args: unknown[]) => unknown;
    send('https://api.moesif.net/v1/actions', unsafeAction, {}, vi.fn());

    expect(transport).toHaveBeenCalledTimes(1);
    const sent = JSON.stringify(transport.mock.calls[0][1]);
    expect(sent).toContain('/organizations/:orgHandle/home');
    expect(sent).not.toMatch(/acme-corp|secret|Payments API/);
  });

  it('drops what cannot be made safe, and tells the SDK it is done with it', async () => {
    await trackCreate();
    const callback = vi.fn();
    const send = sdk._executeRequest as (...args: unknown[]) => unknown;
    send('https://api.moesif.net/v1/events', { request: { uri: 'https://x/y' } }, {}, callback);

    expect(transport).not.toHaveBeenCalled();
    expect(callback).toHaveBeenCalledTimes(1);
  });

  it('keeps tracking off when the SDK offers no transport to filter', async () => {
    delete sdk._executeRequest;
    await trackCreate();

    expect(sdk.init).not.toHaveBeenCalled();
    expect(sdk.track).not.toHaveBeenCalled();
  });
});
