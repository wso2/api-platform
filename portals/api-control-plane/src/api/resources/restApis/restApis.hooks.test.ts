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

import { waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';

import {
  aRestApi,
  accepts,
  collection,
  failure,
  noContent,
  recorder,
  resource,
  type Recorder,
} from '../../../test/msw';
import { renderApiHook, settle } from '../../../test/renderApiHook';
import { server } from '../../../test/server';
import { resetHttpClient } from '../../core/http';
import {
  useCreateRestApi,
  useDeleteRestApi,
  useRestApi,
  useRestApiCounts,
  useRestApis,
  useUpdateRestApi,
} from './restApis.hooks';
import { restApiKeys } from './restApis.queries';

/**
 * Hook-layer tests for the **full-CRUD-with-optimistic-update** shape.
 *
 * This file stands in for six modules that share it byte-for-byte in structure:
 * `restApis`, `projects`, `gateways`, `applications`, `subscriptions` and
 * `subscriptionPlans`. They differ only in nouns and URLs — URLs being covered
 * per-resource by the `*.endpoints.test.ts` files — so duplicating this file
 * five more times would re-test one template with different names. If you are
 * adding a resource that follows this shape, no new hook test is needed.
 *
 * What is covered here is specifically what a component test *cannot* see:
 * scope gating before a request, the optimistic write and its rollback, cache
 * seeding (visible only as a loading state that does not happen), and the
 * breadth of invalidation (a single-page component cannot tell a root
 * invalidation from a current-page one).
 */

const API_ID = 'pizza-shack';

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('useRestApis — scope gating', () => {
  it('fetches once organization and project are known', async () => {
    server.use(collection('/rest-apis', [aRestApi()], { record: requests }));

    const { result } = renderApiHook(() => useRestApis());

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(requests.count()).toBe(1);
  });

  it('issues no request at all while the organization is unknown', async () => {
    // A disabled query is indistinguishable from a loading one, so this failure
    // mode is silent by construction — hence the explicit absence assertion.
    server.use(collection('/rest-apis', [], { record: requests }));

    renderApiHook(() => useRestApis(), { orgId: undefined });

    await settle();
    expect(requests.count()).toBe(0);
  });

  it('issues no request while the project is unknown', async () => {
    server.use(collection('/rest-apis', [], { record: requests }));

    renderApiHook(() => useRestApis(), { projectId: undefined });

    await settle();
    expect(requests.count()).toBe(0);
  });

  it('takes projectId from scope rather than from the caller', async () => {
    server.use(collection('/rest-apis', [], { record: requests }));

    renderApiHook(() => useRestApis(), { projectId: 'wholesale' });

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(requests.last()?.params.get('projectId')).toBe('wholesale');
  });

  it('lets the caller override scope for a cross-project read', async () => {
    server.use(collection('/rest-apis', [], { record: requests }));

    renderApiHook(() => useRestApis({}, { projectId: 'other-project' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(requests.last()?.params.get('projectId')).toBe('other-project');
  });
});

describe('useRestApiCounts — organization aggregation', () => {
  it('returns each project total and their organization-wide sum', async () => {
    server.use(collection('/rest-apis', [aRestApi(), aRestApi(), aRestApi()]));

    const { result } = renderApiHook(() => useRestApiCounts(['retail', 'wholesale']));

    await waitFor(() => expect(result.current.isPending).toBe(false));
    expect(result.current.counts).toEqual({ retail: 3, wholesale: 3 });
    expect(result.current.total).toBe(6);
  });
});

describe('useRestApi — detail gating', () => {
  it('does not fetch without an id', async () => {
    server.use(resource('/rest-apis/:restApiId', aRestApi(), { record: requests }));

    renderApiHook(() => useRestApi(undefined));

    await settle();
    expect(requests.count()).toBe(0);
  });

  it('fetches once an id is supplied', async () => {
    server.use(resource(`/rest-apis/${API_ID}`, aRestApi(), { record: requests }));

    const { result } = renderApiHook(() => useRestApi(API_ID));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toMatchObject({ id: API_ID });
  });
});

describe('useCreateRestApi — cache seeding', () => {
  it('writes the created resource into the detail cache', async () => {
    server.use(accepts('post', '/rest-apis', aRestApi({ id: 'new-api' })));

    const { result, queryClient, org } = renderApiHook(() => useCreateRestApi());
    result.current.mutate(aRestApi());

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(queryClient.getQueryData(restApiKeys.detail(org, 'new-api'))).toMatchObject({
      id: 'new-api',
    });
  });

  it('means opening the new resource renders without a loading state', async () => {
    // This is the whole point of seeding, and it is invisible to a component
    // test: the page renders correctly either way, just with a spinner and a
    // round trip that need not have happened.
    //
    // Mount detail (not just cache checks) to verify opening behavior from
    // seeding. `id` starts undefined so the query stays disabled until the
    // seed lands, mirroring navigation to the new API page.
    server.use(
      accepts('post', '/rest-apis', aRestApi({ id: 'new-api' })),
      resource('/rest-apis/new-api', aRestApi({ id: 'new-api' }), {
        record: requests,
      }),
    );

    // Mutable holder rather than `let`: the closure below reads it on every
    // render, so it cannot be a `const` the linter would otherwise ask for.
    const opened: { id?: string } = {};
    const { result, rerender, queryClient, org } = renderApiHook(() => ({
      create: useCreateRestApi(),
      detail: useRestApi(opened.id),
    }));

    result.current.create.mutate(aRestApi());
    await waitFor(() => expect(result.current.create.isSuccess).toBe(true));

    opened.id = 'new-api';
    rerender();

    // Data is already present on first render, so assert synchronously.
    expect(result.current.detail.data).toMatchObject({ id: 'new-api' });
    expect(result.current.detail.isLoading).toBe(false);
    expect(queryClient.getQueryData(restApiKeys.detail(org, 'new-api'))).toBeDefined();

    // Keep the background revalidation check: it ensures create still seeds
    // detail data and invalidates the resource root.
    await waitFor(() => expect(requests.count()).toBe(1));
  });

  it('invalidates every list variant, not only the one on screen', async () => {
    // A create shifts pagination on pages the user has not visited. A component
    // showing one page cannot tell a root invalidation from a narrow one.
    server.use(accepts('post', '/rest-apis', aRestApi({ id: 'new-api' })));

    const { result, queryClient, org } = renderApiHook(() => useCreateRestApi());
    const pageOne = restApiKeys.list(org, { projectId: 'retail', offset: 0 });
    const pageTwo = restApiKeys.list(org, { projectId: 'retail', offset: 12 });
    queryClient.setQueryData(pageOne, { count: 0, list: [], pagination: {} });
    queryClient.setQueryData(pageTwo, { count: 0, list: [], pagination: {} });

    result.current.mutate(aRestApi());
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    await waitFor(() => {
      expect(queryClient.getQueryState(pageOne)?.isInvalidated).toBe(true);
      expect(queryClient.getQueryState(pageTwo)?.isInvalidated).toBe(true);
    });
  });
});

describe('useUpdateRestApi — optimistic update', () => {
  const seedDetail = (
    queryClient: ReturnType<typeof renderApiHook>['queryClient'],
    org: ReturnType<typeof renderApiHook>['org'],
  ) =>
    queryClient.setQueryData(
      restApiKeys.detail(org, API_ID),
      aRestApi({ displayName: 'Pizza Shack' }),
    );

  it('shows the edit immediately, before the server has answered', async () => {
    server.use(accepts('put', `/rest-apis/${API_ID}`, aRestApi()));

    const { result, queryClient, org } = renderApiHook(() => useUpdateRestApi());
    seedDetail(queryClient, org);

    result.current.mutate({
      restApiId: API_ID,
      body: aRestApi({ displayName: 'Renamed' }),
    });

    await waitFor(() =>
      expect(
        queryClient.getQueryData<{ displayName: string }>(restApiKeys.detail(org, API_ID))
          ?.displayName,
      ).toBe('Renamed'),
    );
  });

  it('rolls back to the previous value when the server rejects it', async () => {
    // Without the rollback the UI keeps showing an edit that was never saved,
    // which is worse than never having shown it.
    server.use(failure('put', `/rest-apis/${API_ID}`, 500, 'INTERNAL_ERROR'));

    const { result, queryClient, org } = renderApiHook(() => useUpdateRestApi());
    seedDetail(queryClient, org);

    result.current.mutate({
      restApiId: API_ID,
      body: aRestApi({ displayName: 'Renamed' }),
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(
      queryClient.getQueryData<{ displayName: string }>(restApiKeys.detail(org, API_ID))
        ?.displayName,
    ).toBe('Pizza Shack');
  });

  it('rolls back a failed edit even though the mutation defines its own onError', async () => {
    // The rollback lives in the mutation's own onError, which replaces the
    // client default. The global failure handler must still fire — that is why
    // it is registered on the MutationCache rather than in defaultOptions.
    const reported: unknown[] = [];
    server.use(failure('put', `/rest-apis/${API_ID}`, 500, 'INTERNAL_ERROR'));

    const { result, queryClient, org } = renderApiHook(() => useUpdateRestApi());
    queryClient.getMutationCache().config.onError = (error) => reported.push(error);
    seedDetail(queryClient, org);

    result.current.mutate({
      restApiId: API_ID,
      body: aRestApi({ displayName: 'Renamed' }),
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(reported).toHaveLength(1);
  });

  it('reconciles with the server after the edit settles', async () => {
    server.use(accepts('put', `/rest-apis/${API_ID}`, aRestApi()));

    const { result, queryClient, org } = renderApiHook(() => useUpdateRestApi());
    const listKey = restApiKeys.list(org, { projectId: 'retail' });
    queryClient.setQueryData(listKey, { count: 0, list: [], pagination: {} });

    result.current.mutate({ restApiId: API_ID, body: aRestApi() });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    // The response may carry server-managed fields the optimistic merge could
    // not know, so the cache is refreshed regardless of outcome.
    await waitFor(() => expect(queryClient.getQueryState(listKey)?.isInvalidated).toBe(true));
  });

  it('does nothing optimistic when the resource is not in cache', async () => {
    // Editing from a list row, with no detail entry loaded, must not fabricate
    // one — a partial object keyed as the resource would be read as the whole.
    server.use(accepts('put', `/rest-apis/${API_ID}`, aRestApi()));

    const { result, queryClient, org } = renderApiHook(() => useUpdateRestApi());

    result.current.mutate({
      restApiId: API_ID,
      body: aRestApi({ displayName: 'Renamed' }),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(queryClient.getQueryData(restApiKeys.detail(org, API_ID))).toBeUndefined();
  });
});

describe('useDeleteRestApi', () => {
  it('drops the detail entry rather than leaving it to 404 on refetch', async () => {
    server.use(noContent('delete', `/rest-apis/${API_ID}`));

    const { result, queryClient, org } = renderApiHook(() => useDeleteRestApi());
    queryClient.setQueryData(restApiKeys.detail(org, API_ID), aRestApi());

    result.current.mutate({ restApiId: API_ID });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(queryClient.getQueryData(restApiKeys.detail(org, API_ID))).toBeUndefined();
  });

  it('invalidates every list variant', async () => {
    server.use(noContent('delete', `/rest-apis/${API_ID}`));

    const { result, queryClient, org } = renderApiHook(() => useDeleteRestApi());
    const pageTwo = restApiKeys.list(org, { projectId: 'retail', offset: 12 });
    queryClient.setQueryData(pageTwo, { count: 0, list: [], pagination: {} });

    result.current.mutate({ restApiId: API_ID });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    await waitFor(() => expect(queryClient.getQueryState(pageTwo)?.isInvalidated).toBe(true));
  });

  it('leaves another organization’s cache untouched', async () => {
    // Invalidation is prefix-based and every key is scoped, so a mutation in
    // one tenant must not reach another's entries.
    server.use(noContent('delete', `/rest-apis/${API_ID}`));

    const { result, queryClient } = renderApiHook(() => useDeleteRestApi());
    const otherOrgKey = restApiKeys.list('globex-org' as never, {
      projectId: 'retail',
    });
    queryClient.setQueryData(otherOrgKey, { count: 0, list: [], pagination: {} });

    result.current.mutate({ restApiId: API_ID });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(queryClient.getQueryState(otherOrgKey)?.isInvalidated).toBe(false);
  });
});
