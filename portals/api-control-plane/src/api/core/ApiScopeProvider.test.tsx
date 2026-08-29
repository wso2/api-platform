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

import { type ReactNode } from 'react';
import { QueryClientProvider } from '@tanstack/react-query';
import { render, waitFor } from '@testing-library/react';
import { http as mswHttp, HttpResponse } from 'msw';
import { beforeEach, describe, expect, it } from 'vitest';

import { server } from '../../test/server';
import { useRestApis } from '../resources/restApis/restApis.hooks';
import { ApiScopeProvider } from './ApiScopeProvider';
import { resetHttpClient } from './http';
import { createQueryClient } from './queryClient';
import { orgScope, scopeKey } from './queryKeys';

/**
 * Integration cover for the seam that makes the whole layer run.
 *
 * Every scoped query is gated on `enabled: Boolean(org)`, and `org` comes from
 * this provider. Without it mounted, the context default leaves `org`
 * undefined and *no request is ever issued* — a failure mode that is silent by
 * construction, since a disabled query looks exactly like one that has not
 * loaded yet. These tests are what stop that from regressing unnoticed.
 */

const BASE = `${window.location.origin}/api/v0.9`;

/** Records the org header of every `/rest-apis` request that reaches the wire. */
let requestedOrgs: (string | null)[] = [];

const recordingHandler = mswHttp.get(`${BASE}/rest-apis`, ({ request }) => {
  requestedOrgs.push(request.headers.get('X-Org-Id'));
  return HttpResponse.json({ count: 0, list: [], pagination: { total: 0, offset: 0, limit: 20 } });
});

/** A component whose only job is to call a scoped hook. */
function ApiListProbe() {
  useRestApis();
  return null;
}

const renderWithScope = (
  scope: { orgId?: string; projectId?: string },
  children: ReactNode = <ApiListProbe />
) => {
  const queryClient = createQueryClient();
  const result = render(
    <QueryClientProvider client={queryClient}>
      <ApiScopeProvider {...scope}>{children}</ApiScopeProvider>
    </QueryClientProvider>
  );
  return { ...result, queryClient };
};

beforeEach(() => {
  requestedOrgs = [];
  resetHttpClient();
  server.use(recordingHandler);
});

describe('providing scope', () => {
  it('lets a scoped query run once an organization and project are known', async () => {
    renderWithScope({ orgId: 'acme-org', projectId: 'retail' });

    await waitFor(() => expect(requestedOrgs).toHaveLength(1));
  });

  it('sends the active organization on the wire', async () => {
    renderWithScope({ orgId: 'acme-org', projectId: 'retail' });

    await waitFor(() => expect(requestedOrgs).toEqual(['acme-org']));
  });

  it('issues no request at all while the organization is unknown', async () => {
    // The `enabled` gate, seen from the outside: a query with no scope must not
    // reach the network, rather than firing an unscoped request the server
    // would reject.
    renderWithScope({ projectId: 'retail' });

    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(requestedOrgs).toEqual([]);
  });

  it('issues no request while the project is unknown', async () => {
    renderWithScope({ orgId: 'acme-org' });

    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(requestedOrgs).toEqual([]);
  });
});

describe('switching organization', () => {
  const seedCacheFor = (queryClient: ReturnType<typeof createQueryClient>, org: string) => {
    const scope = orgScope(org)!;
    queryClient.setQueryData([...scopeKey(scope), 'restApis', 'detail', 'x'], {
      id: 'x',
    });
  };

  const cachedKeysFor = (
    queryClient: ReturnType<typeof createQueryClient>,
    org: string
  ) =>
    queryClient
      .getQueryCache()
      .findAll({ queryKey: scopeKey(orgScope(org)!) })
      .map((query) => query.queryKey);

  it('evicts the previous organization’s cache so its data cannot be shown under another', async () => {
    const { queryClient, rerender } = renderWithScope(
      { orgId: 'acme-org', projectId: 'retail' },
      <div />
    );
    seedCacheFor(queryClient, 'acme-org');
    expect(cachedKeysFor(queryClient, 'acme-org')).toHaveLength(1);

    rerender(
      <QueryClientProvider client={queryClient}>
        <ApiScopeProvider orgId="globex-org" projectId="retail">
          <div />
        </ApiScopeProvider>
      </QueryClientProvider>
    );

    await waitFor(() =>
      expect(cachedKeysFor(queryClient, 'acme-org')).toHaveLength(0)
    );
  });

  it('keeps the cache when the organization has not actually changed', async () => {
    // A re-render for any other reason — a project change, a parent update —
    // must not throw away data the user is still looking at.
    const { queryClient, rerender } = renderWithScope(
      { orgId: 'acme-org', projectId: 'retail' },
      <div />
    );
    seedCacheFor(queryClient, 'acme-org');

    rerender(
      <QueryClientProvider client={queryClient}>
        <ApiScopeProvider orgId="acme-org" projectId="wholesale">
          <div />
        </ApiScopeProvider>
      </QueryClientProvider>
    );

    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(cachedKeysFor(queryClient, 'acme-org')).toHaveLength(1);
  });

  it('keeps the cache when the organization becomes momentarily unknown', async () => {
    // Navigating between org-scoped routes can briefly yield no organization.
    // Treating that as a switch would evict a cache the user is about to
    // return to.
    const { queryClient, rerender } = renderWithScope(
      { orgId: 'acme-org', projectId: 'retail' },
      <div />
    );
    seedCacheFor(queryClient, 'acme-org');

    rerender(
      <QueryClientProvider client={queryClient}>
        <ApiScopeProvider>
          <div />
        </ApiScopeProvider>
      </QueryClientProvider>
    );

    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(cachedKeysFor(queryClient, 'acme-org')).toHaveLength(1);
  });
});
