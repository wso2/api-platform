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

import { fetchConsent, recordConsent } from './consentApi';

const respond = (status: number, body?: unknown) =>
  vi.fn().mockResolvedValue(new Response(body === undefined ? null : JSON.stringify(body), { status }));

afterEach(() => vi.unstubAllGlobals());

describe('fetchConsent', () => {
  it('reads the record through the proxy, uncached and same-origin', async () => {
    const fetchMock = respond(200, { optoutOfEmails: false, consentStatus: 'Confirmed' });
    vi.stubGlobal('fetch', fetchMock);
    expect(await fetchConsent()).toEqual({ optoutOfEmails: false, consentStatus: 'Confirmed' });
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe('/proxy/marketing-consent/marketing-consent');
    expect(init.cache).toBe('no-store');
    expect(init.credentials).toBe('same-origin');
    expect(init.signal).toBeInstanceOf(AbortSignal);
  });

  it.each(['No Opt-in', 'Unconfirmed', 'Confirmed', 'Unsubscribed'])('accepts %s', async (status) => {
    vi.stubGlobal('fetch', respond(200, { optoutOfEmails: false, consentStatus: status }));
    expect((await fetchConsent())?.consentStatus).toBe(status);
  });

  it('keeps a true blanket opt-out', async () => {
    vi.stubGlobal('fetch', respond(200, { optoutOfEmails: true, consentStatus: 'Unsubscribed' }));
    expect(await fetchConsent()).toEqual({ optoutOfEmails: true, consentStatus: 'Unsubscribed' });
  });

  it('treats a missing or null optoutOfEmails as false', async () => {
    vi.stubGlobal('fetch', respond(200, { consentStatus: 'No Opt-in' }));
    expect((await fetchConsent())?.optoutOfEmails).toBe(false);
    vi.stubGlobal('fetch', respond(200, { optoutOfEmails: null, consentStatus: 'No Opt-in' }));
    expect((await fetchConsent())?.optoutOfEmails).toBe(false);
  });

  it.each([400, 403, 404, 500, 502])('answers null for %i', async (status) => {
    vi.stubGlobal('fetch', respond(status, { error: 'x' }));
    expect(await fetchConsent()).toBeNull();
  });

  it.each([
    ['an unrecognised status', { optoutOfEmails: false, consentStatus: 'Maybe' }],
    ['a missing status', { optoutOfEmails: false }],
    ['a null body', null],
  ])('answers null for %s', async (_label, body) => {
    vi.stubGlobal('fetch', respond(200, body));
    expect(await fetchConsent()).toBeNull();
  });

  it('answers null for a body that is not JSON', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('<html>', { status: 200 })));
    expect(await fetchConsent()).toBeNull();
  });

  it('answers null when the request fails or times out', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('network')));
    expect(await fetchConsent()).toBeNull();
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new DOMException('timeout', 'TimeoutError')));
    expect(await fetchConsent()).toBeNull();
  });
});

describe('recordConsent', () => {
  it.each(['Confirmed', 'Unsubscribed'] as const)(
    'posts only consentstatus=%s, with the CSRF header, and accepts 202',
    async (status) => {
      const fetchMock = respond(202);
      vi.stubGlobal('fetch', fetchMock);
      expect(await recordConsent(status)).toBe(true);
      const [url, init] = fetchMock.mock.calls[0];
      expect(url).toBe('/proxy/marketing-consent/marketing-consent');
      expect(init.method).toBe('POST');
      expect(JSON.parse(init.body)).toEqual({ consentstatus: status });
      expect(init.headers['X-Requested-By']).toBe('api-control-plane');
      expect(init.headers['Content-Type']).toBe('application/json');
    }
  );

  it.each([200, 400, 403, 404, 502])('does not count %i as accepted', async (status) => {
    vi.stubGlobal('fetch', respond(status, { error: 'x' }));
    expect(await recordConsent('Unsubscribed')).toBe(false);
  });

  it('reports false when the request fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('network')));
    expect(await recordConsent('Confirmed')).toBe(false);
  });
});
