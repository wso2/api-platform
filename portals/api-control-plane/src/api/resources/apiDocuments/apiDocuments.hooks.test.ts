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

import { http as mswHttp, HttpResponse } from 'msw';
import { beforeEach, describe, expect, it } from 'vitest';
import { waitFor } from '@testing-library/react';

import {
  apiUrl,
  listEnvelope,
  noContent,
  recorder,
  resource,
  type Recorder,
} from '../../../test/msw';
import { renderApiHook, settle } from '../../../test/renderApiHook';
import { server } from '../../../test/server';
import { resetHttpClient } from '../../core/http';
import type { ApiDocument, ApiDocumentListResponse } from './apiDocuments.endpoints';
import {
  useApiDocument,
  useApiDocumentContent,
  useApiDocumentPages,
  useApiDocuments,
  useCreateApiDocument,
  useDeleteApiDocument,
  useUpdateApiDocument,
} from './apiDocuments.hooks';
import { apiDocumentQueries, nextDocumentsOffset } from './apiDocuments.queries';

/**
 * Hook-layer tests for API documents.
 *
 * The React Query behaviours a component test cannot see from the outside live
 * here:
 *
 *   1. Queries wait until the parent API is known (no fetch while `apiId` is
 *      `undefined`), so switching routes does not fire an abandoned request.
 *   2. Writes invalidate every page and filter under the owning API — one
 *      invalidation covers numbered lists, infinite pages, and open documents.
 *   3. Delete removes the deleted doc from every cached list page synchronously
 *      so the auto-select-first-doc effect can't pick a stale row before the
 *      refetch returns.
 */

const API_TYPE = 'rest-api';
const API_ID = 'orders-api';
const COLLECTION = `/apis/${API_TYPE}/${API_ID}/docs`;

const aDocument = (overrides: Partial<ApiDocument> = {}): ApiDocument => ({
  contentType: 'text/markdown; charset=utf-8',
  displayName: 'Getting started',
  id: 'getting-started',
  type: 'HowTo',
  ...overrides,
});

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('useApiDocuments — paged list', () => {
  it('does not fetch while the apiId is unknown', async () => {
    server.use(resource(COLLECTION, listEnvelope([]), { record: requests }));

    renderApiHook(() => useApiDocuments(API_TYPE, undefined));
    await settle();

    expect(requests.count()).toBe(0);
  });

  it('GETs the API’s collection with the paging and filter query', async () => {
    server.use(resource(COLLECTION, listEnvelope([aDocument()]), { record: requests }));

    const { result } = renderApiHook(() =>
      useApiDocuments(API_TYPE, API_ID, { limit: 5, offset: 10, type: 'HowTo' })
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const request = requests.last();
    expect(request?.params.get('limit')).toBe('5');
    expect(request?.params.get('offset')).toBe('10');
    expect(request?.params.get('type')).toBe('HowTo');
    expect(result.current.data?.list[0].id).toBe('getting-started');
  });
});

describe('useApiDocumentPages — infinite list', () => {
  it('appends pages using the server-reported total, not list length', async () => {
    // Two pages of five, total=10. If the hook guessed from "short page" it
    // would stop after page 1 because `list.length === limit`; it must read
    // `pagination.total` instead. This is exactly what `nextDocumentsOffset`
    // encodes, and the hook has to wire it to `getNextPageParam`.
    const pageOne = listEnvelope(
      Array.from({ length: 5 }, (_, i) => aDocument({ id: `page1-${i}` })),
      { offset: 0, limit: 5, total: 10 }
    );
    const pageTwo = listEnvelope(
      Array.from({ length: 5 }, (_, i) => aDocument({ id: `page2-${i}` })),
      { offset: 5, limit: 5, total: 10 }
    );
    server.use(
      mswHttp.get(apiUrl(COLLECTION), async ({ request }) => {
        await requests.capture(request);
        const offset = new URL(request.url).searchParams.get('offset') ?? '0';
        return HttpResponse.json(offset === '0' ? pageOne : pageTwo);
      })
    );

    const { result } = renderApiHook(() => useApiDocumentPages(API_TYPE, API_ID, { limit: 5 }));
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.hasNextPage).toBe(true);

    await result.current.fetchNextPage();
    await waitFor(() => expect(result.current.data?.pages.length).toBe(2));
    expect(result.current.data?.pages[1].list[0].id).toBe('page2-0');
    // Both pages exhausted — total is reached.
    expect(result.current.hasNextPage).toBe(false);
  });

  it('nextDocumentsOffset returns undefined once total is reached', () => {
    // The utility `getNextPageParam` is wired to is deterministic — unit-test
    // its boundary cases here so the hook integration above does not have to
    // enumerate them.
    expect(
      nextDocumentsOffset({
        count: 5,
        list: Array.from({ length: 5 }, (_, i) => aDocument({ id: `d${i}` })),
        pagination: { offset: 0, limit: 5, total: 5 },
      })
    ).toBeUndefined();
    expect(
      nextDocumentsOffset({
        count: 0,
        list: [],
        pagination: { offset: 0, limit: 5, total: 0 },
      })
    ).toBeUndefined();
    expect(
      nextDocumentsOffset({
        count: 3,
        list: Array.from({ length: 3 }, (_, i) => aDocument({ id: `d${i}` })),
        pagination: { offset: 0, limit: 5, total: 10 },
      })
    ).toBe(3);
  });
});

describe('useApiDocument and useApiDocumentContent', () => {
  it('does not fetch metadata while the docId is unknown', async () => {
    server.use(resource(`${COLLECTION}/getting-started`, aDocument(), { record: requests }));

    renderApiHook(() => useApiDocument(API_TYPE, API_ID, undefined));
    await settle();

    expect(requests.count()).toBe(0);
  });

  it('fetches metadata for the given docId', async () => {
    server.use(resource(`${COLLECTION}/getting-started`, aDocument(), { record: requests }));

    const { result } = renderApiHook(() => useApiDocument(API_TYPE, API_ID, 'getting-started'));
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data?.id).toBe('getting-started');
    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/apis/rest-api/orders-api/docs/getting-started'
    );
  });

  it('fetches the content sub-resource as text + content type', async () => {
    server.use(
      mswHttp.get(apiUrl(`${COLLECTION}/getting-started/content`), async ({ request }) => {
        await requests.capture(request);
        return new HttpResponse('# Hello', {
          headers: { 'Content-Type': 'text/markdown; charset=utf-8' },
        });
      })
    );

    const { result } = renderApiHook(() =>
      useApiDocumentContent(API_TYPE, API_ID, 'getting-started')
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data?.text).toBe('# Hello');
    expect(result.current.data?.contentType).toBe('text/markdown; charset=utf-8');
  });
});

describe('useCreateApiDocument', () => {
  it('invalidates every page and filter under the owning API on success', async () => {
    // Two seeded cache entries sit under this API — one list page with a
    // filter, one with none. A single invalidation at the API's detail key
    // must mark both as stale, otherwise a user who adds a doc on page 2 would
    // still see the stale page 1 until a full refresh.
    server.use(resource(COLLECTION, aDocument({ id: 'new-doc' }), { method: 'post', status: 201 }));

    const { result, queryClient, org } = renderApiHook(() => useCreateApiDocument());

    const noFilterKey = apiDocumentQueries.list(org, API_TYPE, API_ID, {}).queryKey;
    const withFilterKey = apiDocumentQueries.list(org, API_TYPE, API_ID, { type: 'HowTo' })
      .queryKey;
    const seed: ApiDocumentListResponse = {
      count: 0,
      list: [],
      pagination: { total: 0, offset: 0, limit: 20 },
    };
    queryClient.setQueryData(noFilterKey, seed);
    queryClient.setQueryData(withFilterKey, seed);

    result.current.mutate({
      apiType: API_TYPE,
      apiId: API_ID,
      body: { type: 'HowTo', displayName: 'New Doc', inlineContent: '# body' },
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    // Both pages — different filters, same parent — must be invalidated.
    await waitFor(() =>
      expect(queryClient.getQueryState(noFilterKey)?.isInvalidated).toBe(true)
    );
    await waitFor(() =>
      expect(queryClient.getQueryState(withFilterKey)?.isInvalidated).toBe(true)
    );
  });
});

describe('useUpdateApiDocument', () => {
  it('invalidates both metadata and content caches on success', async () => {
    // The response is metadata-only, so patching the detail entry would leave
    // the separately-cached content stale. A metadata-only PUT changes
    // displayName etc. without touching bytes, so detail invalidation is
    // obviously required; a content-replacing PUT also needs the content key
    // to refetch — the hook invalidates the whole parent, which covers both.
    server.use(
      resource(`${COLLECTION}/getting-started`, aDocument({ displayName: 'Renamed' }), {
        method: 'put',
      })
    );

    const { result, queryClient, org } = renderApiHook(() => useUpdateApiDocument());

    const detailKey = apiDocumentQueries.detail(org, API_TYPE, API_ID, 'getting-started').queryKey;
    const contentKey = apiDocumentQueries.content(org, API_TYPE, API_ID, 'getting-started')
      .queryKey;
    queryClient.setQueryData(detailKey, aDocument());
    queryClient.setQueryData(contentKey, {
      text: '# old',
      contentType: 'text/markdown; charset=utf-8',
    });

    result.current.mutate({
      apiType: API_TYPE,
      apiId: API_ID,
      docId: 'getting-started',
      body: { displayName: 'Renamed', type: 'HowTo' },
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    await waitFor(() => expect(queryClient.getQueryState(detailKey)?.isInvalidated).toBe(true));
    await waitFor(() => expect(queryClient.getQueryState(contentKey)?.isInvalidated).toBe(true));
  });
});

describe('useDeleteApiDocument', () => {
  it('synchronously removes the deleted doc from cached list pages', async () => {
    // The auto-select-first-doc effect in the develop tab reads from the list
    // cache. If delete only invalidated and refetched, there is a frame where
    // the cache still contains the deleted doc and the effect re-selects it,
    // producing a flash of the just-deleted viewer. Patching cached pages in
    // place closes that window.
    server.use(noContent('delete', `${COLLECTION}/getting-started`));

    const { result, queryClient, org } = renderApiHook(() => useDeleteApiDocument());

    const listKey = apiDocumentQueries.list(org, API_TYPE, API_ID, {}).queryKey;
    const seededPage: ApiDocumentListResponse = {
      count: 2,
      list: [aDocument(), aDocument({ id: 'other-doc', displayName: 'Other' })],
      pagination: { total: 2, offset: 0, limit: 20 },
    };
    queryClient.setQueryData(listKey, seededPage);

    result.current.mutate({ apiType: API_TYPE, apiId: API_ID, docId: 'getting-started' });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const after = queryClient.getQueryData<ApiDocumentListResponse>(listKey);
    expect(after?.list.map((d) => d.id)).toEqual(['other-doc']);
    expect(after?.count).toBe(1);
    expect(after?.pagination.total).toBe(1);
  });

  it('also prunes the deleted doc from cached infinite-query pages', async () => {
    // Same guarantee for the overview tab's "View more" list, which uses an
    // infinite query. The shape is {pages, pageParams} rather than a single
    // envelope, so the delete path has to recognise and update both.
    server.use(noContent('delete', `${COLLECTION}/getting-started`));

    const { result, queryClient, org } = renderApiHook(() => useDeleteApiDocument());

    const pagesKey = apiDocumentQueries.pages(org, API_TYPE, API_ID, {}).queryKey;
    const infinite = {
      pages: [
        {
          count: 2,
          list: [aDocument(), aDocument({ id: 'keep', displayName: 'Keep' })],
          pagination: { total: 2, offset: 0, limit: 20 },
        },
      ],
      pageParams: [0],
    };
    queryClient.setQueryData(pagesKey, infinite);

    result.current.mutate({ apiType: API_TYPE, apiId: API_ID, docId: 'getting-started' });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const after = queryClient.getQueryData<typeof infinite>(pagesKey);
    expect(after?.pages[0].list.map((d) => d.id)).toEqual(['keep']);
    expect(after?.pages[0].pagination.total).toBe(1);
  });

  it('removes the deleted doc’s own detail and content cache entries', async () => {
    // The viewer reads from the detail cache, so a stale entry would still
    // render the deleted doc for one tick after the delete completes.
    server.use(noContent('delete', `${COLLECTION}/getting-started`));

    const { result, queryClient, org } = renderApiHook(() => useDeleteApiDocument());

    const detailKey = apiDocumentQueries.detail(org, API_TYPE, API_ID, 'getting-started').queryKey;
    const contentKey = apiDocumentQueries.content(org, API_TYPE, API_ID, 'getting-started')
      .queryKey;
    queryClient.setQueryData(detailKey, aDocument());
    queryClient.setQueryData(contentKey, {
      text: '# body',
      contentType: 'text/markdown; charset=utf-8',
    });

    result.current.mutate({ apiType: API_TYPE, apiId: API_ID, docId: 'getting-started' });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(queryClient.getQueryData(detailKey)).toBeUndefined();
    expect(queryClient.getQueryData(contentKey)).toBeUndefined();
  });
});
