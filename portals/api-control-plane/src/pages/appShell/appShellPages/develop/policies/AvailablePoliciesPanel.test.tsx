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

import { resetHttpClient } from '@/api/core/http';
import { runtimeConfig } from '@/config/runtime';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor } from '@/test/utils';
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
    const { container } = renderWithProviders(<AvailablePoliciesPanel onSelect={vi.fn()} />);

    expect(container.querySelector('[aria-busy="true"]')).toBeInTheDocument();
    expect(await screen.findByText('Policy 01')).toBeInTheDocument();
    expect(container.querySelector('[aria-busy="true"]')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Next' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Previous' })).not.toBeInTheDocument();
  });

  it('loads the next page when the end of the list scrolls into view', async () => {
    const requests: URLSearchParams[] = [];
    serveCatalog(requests);
    renderWithProviders(<AvailablePoliciesPanel onSelect={vi.fn()} />);

    await screen.findByText('Policy 20');
    expect(screen.queryByText('Policy 21')).not.toBeInTheDocument();

    observerCallbacks.at(-1)?.([{ isIntersecting: true }]);

    expect(await screen.findByText('Policy 25')).toBeInTheDocument();
    expect(requests.map((query) => query.get('offset'))).toEqual(['0', '20']);
  });

  it('asks the hub to search the whole catalog once typing settles', async () => {
    const requests: URLSearchParams[] = [];
    serveCatalog(requests);
    const { user } = renderWithProviders(<AvailablePoliciesPanel onSelect={vi.fn()} />);
    await screen.findByText('Policy 01');

    await user.type(screen.getByPlaceholderText('Search policies'), 'jwt');

    await waitFor(() => expect(requests.at(-1)?.get('search')).toBe('jwt'), { timeout: 3000 });
    // Typing was debounced into one request, not one per keystroke.
    expect(requests.filter((query) => query.get('search')).length).toBe(1);
  });
});
