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

import { routes } from '@/routes/paths';
import type { MoesifAction } from './actions';

const ORG = 'b3e1c2d4-7f6a-4e5b-9c8d-0a1b2c3d4e5f';
const USER = '0193e7a2-4c5d-7e6f-8a9b-0c1d2e3f4a5b';
const COLLECTOR = '/_api/api-management/feature-usage';
const ENABLED = { featureUsageTrackingEnabled: 'true', featureUsageCollectorPath: COLLECTOR };

/** Stands in for the network: nothing in this file leaves the process. */
const fetchMock = vi.fn<typeof fetch>(() => Promise.resolve(new Response(null, { status: 202 })));

/** A fresh page load: the module and its runtime config are re-evaluated. */
const load = async (config: Record<string, string> = ENABLED) => {
  vi.resetModules();
  window.__RUNTIME_CONFIG__ = config;
  return import('./collector');
};

const trackCreate = (
  trackFeatureUsage: Awaited<ReturnType<typeof load>>['trackFeatureUsage'],
  identity = { companyId: ORG, userId: USER },
) =>
  trackFeatureUsage(
    'apip.manage-apis.rest-api-create-click',
    { creation_method: 'scratch' },
    identity,
  );

const sentBatch = (call = 0) =>
  JSON.parse(fetchMock.mock.calls[call][1]?.body as string) as MoesifAction[];

beforeEach(() => {
  vi.useFakeTimers();
  vi.stubGlobal('fetch', fetchMock);
  sessionStorage.clear();
  // A page whose URL carries an organization, project, search text and a fragment.
  window.history.pushState(
    {},
    '',
    `${routes.newApi('acme-corp', 'payments')}?search=Payments+API#step-2`,
  );
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  fetchMock.mockClear();
  delete window.__RUNTIME_CONFIG__;
  window.history.pushState({}, '', '/');
});

describe('feature-usage tracking switch', () => {
  it.each([
    ['nothing configured (on-prem)', {}],
    ['enabled without a collector path', { featureUsageTrackingEnabled: 'true' }],
    ['a collector path without the switch', { featureUsageCollectorPath: COLLECTOR }],
    [
      'a collector on another origin',
      { featureUsageTrackingEnabled: 'true', featureUsageCollectorPath: 'https://api.moesif.net' },
    ],
    [
      'a protocol-relative collector',
      { featureUsageTrackingEnabled: 'true', featureUsageCollectorPath: '//api.moesif.net' },
    ],
    [
      'explicitly off',
      { featureUsageTrackingEnabled: 'false', featureUsageCollectorPath: COLLECTOR },
    ],
  ])('sends nothing with %s', async (_name, config) => {
    const { flushFeatureUsage, isFeatureUsageTrackingEnabled, trackFeatureUsage } =
      await load(config);
    trackCreate(trackFeatureUsage);
    flushFeatureUsage();
    await vi.runAllTimersAsync();

    expect(isFeatureUsageTrackingEnabled()).toBe(false);
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe('trackFeatureUsage', () => {
  it('posts the batch to the same-origin collector route, with the session cookie and no Moesif ID', async () => {
    const { BATCH_MAX_WAIT_MS, trackFeatureUsage } = await load();
    trackCreate(trackFeatureUsage);
    await vi.advanceTimersByTimeAsync(BATCH_MAX_WAIT_MS);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe(`${COLLECTOR}/v1/actions/batch`);
    expect(init).toMatchObject({ credentials: 'same-origin', keepalive: true, method: 'POST' });
    expect(init?.headers).toEqual({ 'Content-Type': 'application/json' });
  });

  it('sends the taxonomy code with the masked page and the IDs only', async () => {
    const { BATCH_MAX_WAIT_MS, trackFeatureUsage } = await load();
    trackCreate(trackFeatureUsage);
    await vi.advanceTimersByTimeAsync(BATCH_MAX_WAIT_MS);

    const [action] = sentBatch();
    expect(action).toMatchObject({
      action_name: 'apip.manage-apis.rest-api-create-click',
      company_id: ORG,
      metadata: { creation_method: 'scratch', initiating_product: 'apip' },
      request: { uri: `${window.location.origin}${routes.newApi()}` },
      user_id: USER,
    });
    expect(JSON.stringify(sentBatch())).not.toMatch(/acme-corp|payments|Payments|search|step-2/);
  });

  it('batches actions: one request per wait, not one per action', async () => {
    const { BATCH_MAX_WAIT_MS, trackFeatureUsage } = await load();
    trackCreate(trackFeatureUsage);
    trackCreate(trackFeatureUsage);
    await vi.advanceTimersByTimeAsync(BATCH_MAX_WAIT_MS - 1);
    expect(fetchMock).not.toHaveBeenCalled();

    await vi.advanceTimersByTimeAsync(1);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(sentBatch()).toHaveLength(2);
  });

  it('sends at once when a batch is full', async () => {
    const { BATCH_SIZE, trackFeatureUsage } = await load();
    for (let i = 0; i < BATCH_SIZE; i += 1) trackCreate(trackFeatureUsage);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(sentBatch()).toHaveLength(BATCH_SIZE);
  });

  it.each([
    ['the page is closed', () => window.dispatchEvent(new Event('pagehide'))],
    [
      'the page is hidden',
      () => {
        vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden');
        document.dispatchEvent(new Event('visibilitychange'));
      },
    ],
  ])('sends what is queued when %s', async (_name, leave) => {
    const { trackFeatureUsage } = await load();
    trackCreate(trackFeatureUsage);
    leave();

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(sentBatch()).toHaveLength(1);
  });

  it('gives every action of a tab the same session token', async () => {
    const { BATCH_MAX_WAIT_MS, trackFeatureUsage } = await load();
    trackCreate(trackFeatureUsage);
    trackCreate(trackFeatureUsage);
    await vi.advanceTimersByTimeAsync(BATCH_MAX_WAIT_MS);

    const [first, second] = sentBatch();
    expect(first.session_token).toMatch(/^[0-9a-f-]{36}$/);
    expect(second.session_token).toBe(first.session_token);
  });

  it.each([
    [
      'throws',
      () => {
        throw new Error('network down');
      },
    ],
    ['rejects', () => Promise.reject(new Error('network down'))],
  ])('never throws into the caller when fetch %s', async (_name, failure) => {
    fetchMock.mockImplementationOnce(failure as never);
    const { flushFeatureUsage, trackFeatureUsage } = await load();

    expect(() => {
      trackCreate(trackFeatureUsage);
      flushFeatureUsage();
    }).not.toThrow();
    await vi.runAllTimersAsync();
  });
});
