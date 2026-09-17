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
import { beforeEach, describe, expect, it } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { apiUrl } from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor } from '@/test/utils';
import { resetTestApiKeys, useTestApiKey } from './useTestApiKey';

/**
 * Minting the console's key through `apiKeys`, remembered for the session.
 *
 * The property under test is *how often a real key is created*. A test key is
 * an ordinary, persisted API key, so an extra mint is not a wasted request — it
 * is another entry in the user's key list. Every assertion here counts calls
 * against a recording handler rather than trusting the hook's own state.
 */

const API_ID = 'payments-api';
const ORG = 'acme-org';

let mintCalls = 0;
let nextKey = 'key-1';

const mintHandler = () =>
  http.post(apiUrl(`/rest-apis/${API_ID}/api-keys`), () => {
    mintCalls += 1;
    return HttpResponse.json({
      apiKey: nextKey,
      keyId: `id-${mintCalls}`,
      message: 'ok',
      status: 'success',
    });
  });

/**
 * Surfaces the hook's result so assertions read the rendered values.
 *
 * Both props are required, with no defaults: a default parameter is used when
 * `undefined` is passed, so a defaulted `restApiId` would silently substitute
 * the real id in exactly the test that means to omit it.
 */
function Probe({ enabled, restApiId }: { enabled: boolean; restApiId: string | undefined }) {
  const { error, isPending, isRegenerating, key, regenerate } = useTestApiKey(restApiId, enabled);

  return (
    <>
      <span data-testid="value">{key?.value ?? ''}</span>
      <span data-testid="pending">{String(isPending)}</span>
      <span data-testid="regenerating">{String(isRegenerating)}</span>
      <span data-testid="error">{error ? 'error' : ''}</span>
      <button onClick={regenerate} type="button">
        New key
      </button>
    </>
  );
}

const renderProbe = (props: Partial<Parameters<typeof Probe>[0]> = {}) =>
  renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <Probe
        enabled={props.enabled ?? true}
        restApiId={'restApiId' in props ? props.restApiId : API_ID}
      />
    </ApiScopeProvider>,
  );

const value = () => screen.getByTestId('value').textContent;

beforeEach(() => {
  mintCalls = 0;
  nextKey = 'key-1';
  resetTestApiKeys();
  resetHttpClient();
});

describe('useTestApiKey', () => {
  it('mints a key through the api-keys endpoint', async () => {
    server.use(mintHandler());

    renderProbe();

    await waitFor(() => expect(value()).toBe('key-1'));
    expect(mintCalls).toBe(1);
  });

  it('mints nothing at all while disabled', async () => {
    server.use(mintHandler());

    renderProbe({ enabled: false });

    // An API whose api-key-auth policy demands no key must never have one
    // created against it — inert, not merely hidden.
    await waitFor(() => expect(screen.getByTestId('pending').textContent).toBe('false'));
    expect(mintCalls).toBe(0);
    expect(value()).toBe('');
  });

  it('mints nothing without an API id', async () => {
    server.use(mintHandler());

    renderProbe({ restApiId: undefined });

    await waitFor(() => expect(screen.getByTestId('pending').textContent).toBe('false'));
    expect(mintCalls).toBe(0);
  });

  it('reuses the session’s key across remounts rather than minting again', async () => {
    server.use(mintHandler());

    const first = renderProbe();
    await waitFor(() => expect(value()).toBe('key-1'));
    first.unmount();

    nextKey = 'key-2';
    renderProbe();

    // The whole reason this hook keeps a session map: without it, navigating
    // away and back would leave another real key in the user's list.
    await waitFor(() => expect(value()).toBe('key-1'));
    expect(mintCalls).toBe(1);
  });

  it('mints once when two components ask at the same time', async () => {
    server.use(mintHandler());

    renderWithProviders(
      <ApiScopeProvider orgId={ORG}>
        <>
          <Probe enabled restApiId={API_ID} />
          <Probe enabled restApiId={API_ID} />
        </>
      </ApiScopeProvider>,
    );

    await waitFor(() => expect(screen.getAllByTestId('value')[0].textContent).toBe('key-1'));
    // Both share the in-flight promise instead of each issuing a request.
    await waitFor(() => expect(mintCalls).toBe(1));
  });

  it('issues a replacement on regenerate', async () => {
    server.use(mintHandler());

    const { user } = renderProbe();
    await waitFor(() => expect(value()).toBe('key-1'));

    nextKey = 'key-2';
    await user.click(screen.getByRole('button', { name: /New key/i }));

    await waitFor(() => expect(value()).toBe('key-2'));
    expect(mintCalls).toBe(2);
  });

  it('remembers the replacement, so a remount does not mint a third', async () => {
    server.use(mintHandler());

    const { user, unmount } = renderProbe();
    await waitFor(() => expect(value()).toBe('key-1'));

    nextKey = 'key-2';
    await user.click(screen.getByRole('button', { name: /New key/i }));
    await waitFor(() => expect(value()).toBe('key-2'));
    unmount();

    renderProbe();

    await waitFor(() => expect(value()).toBe('key-2'));
    expect(mintCalls).toBe(2);
  });

  it('surfaces a failure without leaving a key on screen', async () => {
    server.use(
      http.post(apiUrl(`/rest-apis/${API_ID}/api-keys`), () => {
        mintCalls += 1;
        return HttpResponse.json({ code: '500', message: 'nope' }, { status: 500 });
      }),
    );

    renderProbe();

    await waitFor(() => expect(screen.getByTestId('error').textContent).toBe('error'));
    expect(value()).toBe('');
  });

  it('hands back no key while the API it was minted for is no longer the one asked about', async () => {
    const OTHER = 'billing-api';
    server.use(
      mintHandler(),
      http.post(apiUrl(`/rest-apis/${OTHER}/api-keys`), () =>
        HttpResponse.json({
          apiKey: 'other-key',
          keyId: 'id-other',
          message: 'ok',
          status: 'success',
        }),
      ),
    );

    const { rerender } = renderProbe();
    await waitFor(() => expect(value()).toBe('key-1'));

    // Only the route's API changes, so React re-renders this component rather
    // than remounting it, and the previous key survives in state. Returning it
    // would inject one API's credential into a request aimed at another.
    rerender(
      <ApiScopeProvider orgId={ORG}>
        <Probe enabled restApiId={OTHER} />
      </ApiScopeProvider>,
    );

    // Asserted with no waiting on purpose: the ownership check happens during
    // render, so the previous API's key is gone on the very first frame after
    // the switch, before the replacement has been asked for.
    expect(value()).toBe('');

    await waitFor(() => expect(value()).toBe('other-key'));
  });

  it('takes the key off screen when a replacement fails', async () => {
    let calls = 0;
    server.use(
      http.post(apiUrl(`/rest-apis/${API_ID}/api-keys`), () => {
        calls += 1;
        mintCalls += 1;
        if (calls === 1) {
          return HttpResponse.json({
            apiKey: 'key-1',
            keyId: 'id-1',
            message: 'ok',
            status: 'success',
          });
        }
        return HttpResponse.json({ code: '500', message: 'nope' }, { status: 500 });
      }),
    );

    const { user } = renderProbe();
    await waitFor(() => expect(value()).toBe('key-1'));

    await user.click(screen.getByRole('button', { name: /New key/i }));

    // The old key was forgotten the moment the replacement was asked for, so
    // showing it afterwards would present a credential the console has already
    // abandoned as though it were current.
    await waitFor(() => expect(screen.getByTestId('error').textContent).toBe('error'));
    expect(value()).toBe('');
  });

  it('does not remember a failed mint, so a later attempt retries', async () => {
    let shouldFail = true;
    server.use(
      http.post(apiUrl(`/rest-apis/${API_ID}/api-keys`), () => {
        mintCalls += 1;
        if (shouldFail) return HttpResponse.json({ code: '500', message: 'nope' }, { status: 500 });
        return HttpResponse.json({
          apiKey: 'key-2',
          keyId: 'id',
          message: 'ok',
          status: 'success',
        });
      }),
    );

    const { user } = renderProbe();
    await waitFor(() => expect(screen.getByTestId('error').textContent).toBe('error'));

    shouldFail = false;
    await user.click(screen.getByRole('button', { name: /New key/i }));

    await waitFor(() => expect(value()).toBe('key-2'));
  });

  it('treats a 200 carrying an error status as a failure', async () => {
    server.use(
      http.post(apiUrl(`/rest-apis/${API_ID}/api-keys`), () => {
        mintCalls += 1;
        // The endpoint can answer 200 with `status: 'error'` and no key.
        return HttpResponse.json({ message: 'refused', status: 'error' });
      }),
    );

    renderProbe();

    await waitFor(() => expect(screen.getByTestId('error').textContent).toBe('error'));
    expect(value()).toBe('');
  });

  it('asks for a recognisable, short-lived key', async () => {
    let body: { displayName?: string; expiresIn?: unknown } | undefined;
    server.use(
      http.post(apiUrl(`/rest-apis/${API_ID}/api-keys`), async ({ request }) => {
        body = (await request.json()) as typeof body;
        return HttpResponse.json({ apiKey: 'k', keyId: 'id', message: 'ok', status: 'success' });
      }),
    );

    renderProbe();

    await waitFor(() => expect(value()).toBe('k'));
    expect(body?.displayName).toBe('Test console key');
    expect(body?.expiresIn).toEqual({ duration: 1, unit: 'hours' });
  });
});
