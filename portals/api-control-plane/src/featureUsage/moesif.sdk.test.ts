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

import { http, HttpResponse } from 'msw';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { routes } from '@/routes/paths';
import { server } from '@/test/server';

/*
 * Contract test against the real moesif-browser-js (not mocked). The PII filter
 * hooks an internal SDK method, so this checks what actually reaches the
 * network: if an SDK upgrade sends by another path, these assertions fail
 * instead of handles quietly reaching Moesif.
 */

const ORG = 'b3e1c2d4-7f6a-4e5b-9c8d-0a1b2c3d4e5f';
const USER = '0193e7a2-4c5d-7e6f-8a9b-0c1d2e3f4a5b';

type Captured = { path: string; body: unknown };
let captured: Captured[];

const originalCreateObjectURL = URL.createObjectURL;
const originalRevokeObjectURL = URL.revokeObjectURL;

beforeEach(() => {
  captured = [];
  localStorage.clear();
  // jsdom has no blob URLs. The SDK derives each action's transaction_id from
  // one, so stand in for the browser's: blob:<origin>/<uuid>.
  URL.createObjectURL = () => `blob:${window.location.origin}/${crypto.randomUUID()}`;
  URL.revokeObjectURL = () => undefined;
  server.use(
    http.post(/api\.moesif\.net\/v1\/.*/, async ({ request }) => {
      captured.push({ body: await request.json(), path: new URL(request.url).pathname });
      return new HttpResponse(null, { status: 201 });
    }),
  );
  // A page whose URL carries an organization, project, search text and a fragment.
  window.history.pushState(
    {},
    '',
    `${routes.newApi('acme-corp', 'payments')}?search=Payments+API#step-2`,
  );
});

afterEach(() => {
  vi.useRealTimers();
  URL.createObjectURL = originalCreateObjectURL;
  URL.revokeObjectURL = originalRevokeObjectURL;
  delete window.__RUNTIME_CONFIG__;
  window.history.pushState({}, '', '/');
});

const trackCreateWithRealSdk = async () => {
  vi.resetModules();
  window.__RUNTIME_CONFIG__ = {
    featureUsageTrackingEnabled: 'true',
    moesifPublishableKey: 'publishable-key',
  };
  const { trackFeatureUsage } = await import('./moesif');
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
  trackFeatureUsage(
    'apip.manage-apis.rest-api-create-click',
    { creation_method: 'scratch' },
    { companyId: ORG, userId: USER },
  );
  // The SDK is imported lazily: let it load and initialise (identify is sent
  // at once), then run the clock past the batch interval so the action flushes.
  await vi.dynamicImportSettled();
  await vi.advanceTimersByTimeAsync(6000);
  vi.useRealTimers();
  await vi.waitFor(() => expect(captured.map((c) => c.path)).toContain('/v1/actions/batch'));
};

describe('moesif-browser-js behind the PII filter', () => {
  it('sends the route template, not the page URL with its handles', async () => {
    await trackCreateWithRealSdk();

    const batch = captured.find((c) => c.path === '/v1/actions/batch')?.body as Array<{
      action_name: string;
      request: { uri: string };
    }>;
    expect(batch).toHaveLength(1);
    expect(batch[0].action_name).toBe('apip.manage-apis.rest-api-create-click');
    expect(batch[0].request.uri).toBe(`${window.location.origin}${routes.newApi()}`);
  });

  it('lets no handle, search text or fragment out in any request', async () => {
    await trackCreateWithRealSdk();

    const wire = JSON.stringify(captured);
    for (const forbidden of ['acme-corp', 'payments', 'Payments', 'search', 'step-2']) {
      expect(wire).not.toContain(forbidden);
    }
  });

  it('identifies the user and organization by ID only', async () => {
    await trackCreateWithRealSdk();

    expect(captured.find((c) => c.path === '/v1/companies')?.body).toEqual({ company_id: ORG });
    expect(captured.find((c) => c.path === '/v1/users')?.body).toEqual(
      expect.objectContaining({ company_id: ORG, user_id: USER }),
    );
    expect(
      Object.keys(captured.find((c) => c.path === '/v1/users')?.body as object).sort(),
    ).toEqual(expect.not.arrayContaining(['metadata', 'campaign']));
  });
});
