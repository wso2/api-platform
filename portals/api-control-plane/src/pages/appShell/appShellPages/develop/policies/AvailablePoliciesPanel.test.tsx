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

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { runtimeConfig } from '@/config/runtime';
import { makeAuthState } from '@/test/mockAuthState';
import { aCustomPolicy, collection, failure, recorder } from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor, within } from '@/test/utils';
import { AvailablePoliciesPanel } from './AvailablePoliciesPanel';

// The hub is a different origin from the page, so jsdom enforces CORS on the reply.
const cors = { 'Access-Control-Allow-Origin': '*' };
const hub = runtimeConfig.policyHubBaseUrl;

const TOTAL = 25;
const entry = (index: number) => ({
  displayName: `Policy ${String(index).padStart(2, '0')}`,
  isLatest: true,
  name: `policy-${index}`,
  version: '1.0',
});

/** Serves the catalog in `limit`-sized pages and records every query string asked for. */
const serveCatalog = (requests: URLSearchParams[]) => {
  server.use(
    http.get(`${hub}/policies/categories`, () =>
      HttpResponse.json({ data: ['Security', 'Traffic'] }, { headers: cors }),
    ),
    http.get(`${hub}/policies`, ({ request }) => {
      const query = new URL(request.url).searchParams;
      requests.push(query);
      const offset = Number(query.get('offset') ?? 0);
      const limit = Number(query.get('limit') ?? 20);
      const data = Array.from({ length: TOTAL }, (_unused, index) => entry(index + 1)).slice(
        offset,
        offset + limit,
      );
      return HttpResponse.json(
        { data, pagination: { limit, offset, total: TOTAL } },
        { headers: cors },
      );
    }),
  );
};

type ObserverCallback = (entries: Array<{ isIntersecting: boolean }>) => void;
let observerCallbacks: ObserverCallback[] = [];

beforeEach(() => {
  resetHttpClient();
  observerCallbacks = [];
  // jsdom's AbortSignal is not the one undici's fetch accepts, so the signal React
  // Query hands the hub request is dropped here; MSW still answers the request.
  const mswFetch = globalThis.fetch;
  vi.spyOn(globalThis, 'fetch').mockImplementation((input, init) =>
    mswFetch(input, { ...init, signal: undefined }),
  );
  // jsdom has no layout, so the end-of-list sentinel never "scrolls into view" on its
  // own; the tests trigger the observer by hand.
  vi.stubGlobal(
    'IntersectionObserver',
    class {
      constructor(callback: ObserverCallback) {
        observerCallbacks.push(callback);
      }
      observe() {}
      disconnect() {}
      unobserve() {}
    },
  );
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe('AvailablePoliciesPanel', () => {
  it('shows placeholder cards while loading, then the policies, with no paging buttons', async () => {
    serveCatalog([]);
    const { container } = renderWithProviders(
      <AvailablePoliciesPanel artifactKind="rest-api" onSelect={vi.fn()} />,
    );

    expect(container.querySelector('[aria-busy="true"]')).toBeInTheDocument();
    expect(await screen.findByText('Policy 01')).toBeInTheDocument();
    expect(container.querySelector('[aria-busy="true"]')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Next' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Previous' })).not.toBeInTheDocument();
  });

  it('loads the next page when the end of the list scrolls into view', async () => {
    const requests: URLSearchParams[] = [];
    serveCatalog(requests);
    renderWithProviders(<AvailablePoliciesPanel artifactKind="rest-api" onSelect={vi.fn()} />);

    await screen.findByText('Policy 20');
    expect(screen.queryByText('Policy 21')).not.toBeInTheDocument();

    observerCallbacks.at(-1)?.([{ isIntersecting: true }]);

    expect(await screen.findByText('Policy 25')).toBeInTheDocument();
    expect(requests.map((query) => query.get('offset'))).toEqual(['0', '20']);
  });

  it('asks the hub to search the whole catalog once typing settles', async () => {
    const requests: URLSearchParams[] = [];
    serveCatalog(requests);
    const { user } = renderWithProviders(
      <AvailablePoliciesPanel artifactKind="rest-api" onSelect={vi.fn()} />,
    );
    await screen.findByText('Policy 01');

    await user.type(screen.getByPlaceholderText('Search policies'), 'jwt');

    await waitFor(() => expect(requests.at(-1)?.get('search')).toBe('jwt'), { timeout: 3000 });
    // Typing was debounced into one request, not one per keystroke.
    expect(requests.filter((query) => query.get('search')).length).toBe(1);
  });

  it('keeps the loaded policies and offers a retry when a later page fails', async () => {
    let secondPageCalls = 0;
    serveCatalog([]);
    server.use(
      http.get(`${hub}/policies`, ({ request }) => {
        const query = new URL(request.url).searchParams;
        const offset = Number(query.get('offset') ?? 0);
        const limit = Number(query.get('limit') ?? 20);
        if (offset > 0) {
          secondPageCalls += 1;
          if (secondPageCalls === 1) return new HttpResponse(null, { headers: cors, status: 500 });
        }
        const data = Array.from({ length: TOTAL }, (_unused, index) => entry(index + 1)).slice(
          offset,
          offset + limit,
        );
        return HttpResponse.json(
          { data, pagination: { limit, offset, total: TOTAL } },
          { headers: cors },
        );
      }),
    );
    const { user } = renderWithProviders(
      <AvailablePoliciesPanel artifactKind="rest-api" onSelect={vi.fn()} />,
    );
    await screen.findByText('Policy 20');

    observerCallbacks.at(-1)?.([{ isIntersecting: true }]);
    expect(
      await screen.findByText('Unable to load policies from the Policy Hub.'),
    ).toBeInTheDocument();
    // The first twenty are still there to drag and pick.
    expect(screen.getByText('Policy 01')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Retry' }));
    expect(await screen.findByText('Policy 25')).toBeInTheDocument();
  });

  it('offers a retry when the first page fails', async () => {
    let calls = 0;
    server.use(
      http.get(`${hub}/policies/categories`, () =>
        HttpResponse.json({ data: [] }, { headers: cors }),
      ),
      http.get(`${hub}/policies`, () => {
        calls += 1;
        return calls === 1
          ? new HttpResponse(null, { headers: cors, status: 500 })
          : HttpResponse.json(
              { data: [entry(1)], pagination: { limit: 20, offset: 0, total: 1 } },
              { headers: cors },
            );
      }),
    );
    const { user } = renderWithProviders(
      <AvailablePoliciesPanel artifactKind="rest-api" onSelect={vi.fn()} />,
    );

    await user.click(await screen.findByRole('button', { name: 'Retry' }));

    expect(await screen.findByText('Policy 01')).toBeInTheDocument();
  });
});

describe('AvailablePoliciesPanel — custom policies', () => {
  const hmacSigner = aCustomPolicy({
    description: 'Signs requests with an HMAC.',
    displayName: 'HMAC Signer',
    name: 'hmac-signer',
    uuid: 'hmac-signer-uuid',
    version: 'v1.2.0',
  });
  const requestTagger = aCustomPolicy({
    name: 'request-tagger',
    uuid: 'request-tagger-uuid',
    version: 'v1.0.0',
  });

  const renderInOrg = (
    onSelect = vi.fn(),
    options: Parameters<typeof renderWithProviders>[1] = {},
  ) =>
    renderWithProviders(
      <ApiScopeProvider orgId="acme-org">
        <AvailablePoliciesPanel artifactKind="rest-api" onSelect={onSelect} />
      </ApiScopeProvider>,
      options,
    );

  it('lists the organization’s custom policies first, marked Custom, and attaches them', async () => {
    serveCatalog([]);
    server.use(collection('/gateway-custom-policies', [hmacSigner, requestTagger]));
    const onSelect = vi.fn();
    const { user } = renderInOrg(onSelect);

    const card = (await screen.findByText('HMAC Signer')).closest('.MuiCard-root') as HTMLElement;
    expect(within(card).getByText('Custom')).toBeInTheDocument();
    expect(within(card).getByText('v1.2.0')).toBeInTheDocument();
    // A policy without a display name is listed by its name.
    expect(screen.getByText('request-tagger')).toBeInTheDocument();
    const names = (await screen.findAllByText(/^(HMAC Signer|request-tagger|Policy \d+)$/)).map(
      (node) => node.textContent,
    );
    expect(names.slice(0, 2)).toEqual(['HMAC Signer', 'request-tagger']);

    await user.click(card);
    expect(onSelect).toHaveBeenCalledWith(
      expect.objectContaining({ name: 'hmac-signer', version: 'v1.2.0' }),
    );
  });

  it('filters custom policies by the search, on name, display name and description', async () => {
    serveCatalog([]);
    server.use(collection('/gateway-custom-policies', [hmacSigner, requestTagger]));
    const { user } = renderInOrg();
    await screen.findByText('HMAC Signer');

    await user.type(screen.getByPlaceholderText('Search policies'), 'HMAC Signer');

    await waitFor(() => expect(screen.queryByText('request-tagger')).not.toBeInTheDocument(), {
      timeout: 3000,
    });
    expect(screen.getByText('HMAC Signer')).toBeInTheDocument();
  });

  it('still lists the hub’s policies when custom policies cannot be loaded', async () => {
    serveCatalog([]);
    server.use(failure('get', '/gateway-custom-policies', 500, 'INTERNAL_ERROR'));
    renderInOrg();

    expect(
      await screen.findByText('Unable to load the organization’s custom policies.'),
    ).toBeInTheDocument();
    expect(await screen.findByText('Policy 01')).toBeInTheDocument();
  });

  it('does not read custom policies without permission to', async () => {
    const reads = recorder();
    serveCatalog([]);
    server.use(collection('/gateway-custom-policies', [hmacSigner], { record: reads }));
    renderInOrg(vi.fn(), {
      authState: makeAuthState({
        user: { email: 'publisher@example.com', name: 'Publisher', scopes: ['ap:rest_api:update'] },
      }),
      permissionMode: 'enforce',
    });

    await screen.findByText('Policy 01');
    expect(screen.queryByText('HMAC Signer')).not.toBeInTheDocument();
    expect(reads.count()).toBe(0);
  });
});
