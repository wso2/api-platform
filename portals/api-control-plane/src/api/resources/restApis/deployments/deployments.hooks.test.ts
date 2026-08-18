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
import { beforeEach, describe, expect, it, vi } from 'vitest';

import {
  aDeployRequest,
  aDeployment,
  accepts,
  collection,
  noContent,
  recorder,
  type Recorder,
} from '../../../../test/msw';
import { renderApiHook, settle } from '../../../../test/renderApiHook';
import { server } from '../../../../test/server';
import { resetHttpClient } from '../../../core/http';
import { useDeleteDeployment, useDeployApi, useDeployments } from './deployments.hooks';
import {
  deploymentQueries,
  hasTransitioningDeployment,
} from './deployments.queries';
import { restApiKeys } from '../restApis.queries';

/**
 * Hook-layer tests for the **sub-resource** shape — entries keyed beneath a
 * parent rather than under their own root.
 *
 * Stands in for `deployments` and `apiGateways`. The behaviour that matters and
 * that nothing else covers is the *key nesting*: a sub-resource filed under its
 * parent's detail key is evicted when the parent is, with no bookkeeping. That
 * claim is made in `DESIGN.md` and was, until this file, unproven.
 *
 * The polling interval itself is not exercised end to end. It backs off from
 * two seconds, so a faithful test would either sleep for real or fight fake
 * timers against MSW; the transition predicate it keys on is pure and is tested
 * directly instead.
 */

const API_ID = 'pizza-shack';
const DEPLOYMENTS = `/rest-apis/${API_ID}/deployments`;

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('useDeployments — scope and parent gating', () => {
  it('fetches once the organization and parent API are known', async () => {
    server.use(collection(DEPLOYMENTS, [aDeployment()], { record: requests }));

    const { result } = renderApiHook(() => useDeployments(API_ID));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(requests.count()).toBe(1);
  });

  it('issues no request without a parent API', async () => {
    // The parent is an argument rather than scope, because the API being acted
    // on is not always the one in the route — a dialog opened from a list acts
    // on a row.
    server.use(collection('/rest-apis/:restApiId/deployments', [], { record: requests }));

    renderApiHook(() => useDeployments(undefined));

    await settle();
    expect(requests.count()).toBe(0);
  });

  it('issues no request while the organization is unknown', async () => {
    server.use(collection(DEPLOYMENTS, [], { record: requests }));

    renderApiHook(() => useDeployments(API_ID), { orgId: undefined });

    await settle();
    expect(requests.count()).toBe(0);
  });
});

describe('cache keys nest under the parent API', () => {
  it('files deployments beneath the parent’s detail key', async () => {
    server.use(collection(DEPLOYMENTS, [aDeployment()]));

    const { result, queryClient, org } = renderApiHook(() => useDeployments(API_ID));
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const parentPrefix = restApiKeys.detail(org, API_ID);
    const cached = queryClient
      .getQueryCache()
      .findAll({ queryKey: parentPrefix })
      .map((query) => query.queryKey);

    expect(cached).toHaveLength(1);
    expect(cached[0].slice(0, parentPrefix.length)).toEqual([...parentPrefix]);
  });

  it('is evicted when the parent API is removed, with no separate bookkeeping', async () => {
    // This is the property the nesting exists for: deleting an API drops its
    // deployments in the same call.
    server.use(collection(DEPLOYMENTS, [aDeployment()]));

    const { result, queryClient, org } = renderApiHook(() => useDeployments(API_ID));
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    queryClient.removeQueries({ queryKey: restApiKeys.detail(org, API_ID) });

    expect(
      queryClient.getQueryCache().findAll({ queryKey: restApiKeys.detail(org, API_ID) })
    ).toHaveLength(0);
  });

  it('keeps another API’s deployments out of that prefix', async () => {
    server.use(collection('/rest-apis/:restApiId/deployments', [aDeployment()]));

    const { result, queryClient, org } = renderApiHook(() => useDeployments('other-api'));
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(
      queryClient.getQueryCache().findAll({ queryKey: restApiKeys.detail(org, API_ID) })
    ).toHaveLength(0);
  });
});

describe('useDeployApi — invalidation reaches the parent too', () => {
  it('invalidates the deployment list', async () => {
    server.use(accepts('post', DEPLOYMENTS, aDeployment({ status: 'DEPLOYING' })));

    const { result, queryClient, org } = renderApiHook(() => useDeployApi());
    const listKey = restApiKeys.child(org, API_ID, 'deployments');
    queryClient.setQueryData(listKey, { count: 0, list: [], pagination: {} });

    result.current.mutate({ restApiId: API_ID, body: aDeployRequest() });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    await waitFor(() =>
      expect(queryClient.getQueryState(listKey)?.isInvalidated).toBe(true)
    );
  });

  it('also invalidates the API itself, whose summary carries deployment state', async () => {
    // Without this the API detail keeps showing the previous deployment status
    // while the deployment list has already moved on.
    server.use(accepts('post', DEPLOYMENTS, aDeployment({ status: 'DEPLOYING' })));

    const { result, queryClient, org } = renderApiHook(() => useDeployApi());
    const parentKey = restApiKeys.detail(org, API_ID);
    queryClient.setQueryData(parentKey, { id: API_ID });

    result.current.mutate({ restApiId: API_ID, body: aDeployRequest() });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    await waitFor(() =>
      expect(queryClient.getQueryState(parentKey)?.isInvalidated).toBe(true)
    );
  });

  it('leaves another API’s deployments alone', async () => {
    server.use(accepts('post', DEPLOYMENTS, aDeployment()));

    const { result, queryClient, org } = renderApiHook(() => useDeployApi());
    const otherKey = restApiKeys.child(org, 'other-api', 'deployments');
    queryClient.setQueryData(otherKey, { count: 0, list: [], pagination: {} });

    result.current.mutate({ restApiId: API_ID, body: aDeployRequest() });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(queryClient.getQueryState(otherKey)?.isInvalidated).toBe(false);
  });

  it('performs no optimistic write — the server assigns the id and status', async () => {
    server.use(accepts('post', DEPLOYMENTS, aDeployment({ status: 'DEPLOYING' })));

    const { result, queryClient, org } = renderApiHook(() => useDeployApi());

    result.current.mutate({ restApiId: API_ID, body: aDeployRequest() });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(
      queryClient.getQueryData(restApiKeys.child(org, API_ID, 'deployments'))
    ).toBeUndefined();
  });
});

describe('one invalidation reaches every deployment key', () => {
  /**
   * Verifies both invalidations: one makes filtered deployment lists stale,
   * and one makes a single deployment detail stale. The cache assertions below
   * cover the end-to-end behavior, while the key-passing prefix behavior is
   * tested separately in `core/queryKeys.test.ts`.
   */
  it('reaches a list the user has filtered', async () => {
    server.use(accepts('post', DEPLOYMENTS, aDeployment({ status: 'DEPLOYING' })));

    const { result, queryClient, org } = renderApiHook(() => useDeployApi());
    const filteredKey = deploymentQueries.list(org, API_ID, {
      status: 'DEPLOYED',
    }).queryKey;
    // Typed from the query's own key, so the envelope has to be a real one.
    queryClient.setQueryData(filteredKey, {
      count: 0,
      list: [],
      pagination: { total: 0, offset: 0, limit: 10 },
    });

    result.current.mutate({ restApiId: API_ID, body: aDeployRequest() });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    await waitFor(() =>
      expect(queryClient.getQueryState(filteredKey)?.isInvalidated).toBe(true)
    );
  });

  it('reaches a single deployment’s detail entry', async () => {
    // What `useDeployment` renders. Keyed outside the prefix it would keep
    // showing the status the deployment held before this mutation.
    server.use(accepts('post', DEPLOYMENTS, aDeployment({ status: 'DEPLOYING' })));

    const { result, queryClient, org } = renderApiHook(() => useDeployApi());
    const detailKey = deploymentQueries.detail(org, API_ID, 'deployment-1').queryKey;
    queryClient.setQueryData(detailKey, aDeployment({ status: 'DEPLOYED' }));

    result.current.mutate({ restApiId: API_ID, body: aDeployRequest() });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    await waitFor(() =>
      expect(queryClient.getQueryState(detailKey)?.isInvalidated).toBe(true)
    );
  });

  it('files that detail entry under the deployments prefix, not beside it', async () => {
    const { org } = renderApiHook(() => useDeployApi());
    const prefix = restApiKeys.children(org, API_ID, 'deployments');
    const detailKey = deploymentQueries.detail(org, API_ID, 'deployment-1').queryKey;

    expect(detailKey.slice(0, prefix.length)).toEqual([...prefix]);
  });

  it('invalidates the deployments sub-resource by a prefix, not by an exact key', async () => {
    // Ensures the helper invalidates the deployments prefix, not just an
    // unfiltered list or a single detail entry.
    server.use(accepts('post', DEPLOYMENTS, aDeployment()));

    const { result, queryClient, org } = renderApiHook(() => useDeployApi());
    const invalidateQueries = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate({ restApiId: API_ID, body: aDeployRequest() });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const filters = invalidateQueries.mock.calls.map(([call]) => call?.queryKey);
    expect(filters).toContainEqual(restApiKeys.children(org, API_ID, 'deployments'));
  });

  it('does not extend over a sibling sub-resource of the same API', async () => {
    // Structural assertion avoids the broader parent-detail invalidation.
    const { org } = renderApiHook(() => useDeployApi());
    const prefix = restApiKeys.children(org, API_ID, 'deployments');
    const gatewaysKey = restApiKeys.child(org, API_ID, 'gateways');

    expect(gatewaysKey.slice(0, prefix.length)).not.toEqual([...prefix]);
  });
});

describe('hasTransitioningDeployment — the predicate polling keys on', () => {
  it.each([
    ['DEPLOYING', true],
    ['UNDEPLOYING', true],
  ] as const)('treats %s as still settling', (status, expected) => {
    expect(hasTransitioningDeployment([aDeployment({ status })])).toBe(expected);
  });

  it.each([
    ['DEPLOYED', false],
    ['UNDEPLOYED', false],
    ['FAILED', false],
    ['ARCHIVED', false],
  ] as const)('treats %s as settled, so polling stops', (status, expected) => {
    expect(hasTransitioningDeployment([aDeployment({ status })])).toBe(expected);
  });

  it('keeps polling while any single deployment is still settling', async () => {
    expect(
      hasTransitioningDeployment([
        aDeployment({ status: 'DEPLOYED' }),
        aDeployment({ status: 'DEPLOYING' }),
      ])
    ).toBe(true);
  });

  it('is false for an empty or absent list, so nothing polls forever on no data', () => {
    expect(hasTransitioningDeployment([])).toBe(false);
    expect(hasTransitioningDeployment(undefined)).toBe(false);
  });
});

describe('useDeleteDeployment', () => {
  const DEPLOYMENT_ID = 'deployment-1';

  it('invalidates the parent’s deployment list after removal', async () => {
    server.use(
      noContent('delete', `${DEPLOYMENTS}/${DEPLOYMENT_ID}`, { record: requests })
    );

    const { result, queryClient, org } = renderApiHook(() => useDeleteDeployment());
    const listKey = restApiKeys.child(org, API_ID, 'deployments');
    queryClient.setQueryData(listKey, { count: 0, list: [], pagination: {} });

    result.current.mutate({ restApiId: API_ID, deploymentId: DEPLOYMENT_ID });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    // The delete really went out, the invalidation below is this mutation's,
    // not a deploy standing in for it through the shared helper.
    expect(requests.last()?.method).toBe('DELETE');
    await waitFor(() =>
      expect(queryClient.getQueryState(listKey)?.isInvalidated).toBe(true)
    );
  });

  it('drops the deleted deployment’s detail entry rather than refetching it', async () => {
    // Delete's own branch, and the one the shared helper cannot cover:
    // invalidating this key would refetch a deployment that is gone and show
    // the user a 404 on the way past.
    server.use(noContent('delete', `${DEPLOYMENTS}/${DEPLOYMENT_ID}`));

    const { result, queryClient, org } = renderApiHook(() => useDeleteDeployment());
    // The query's own key, not a hand-built copy: if the hook and the query
    // definition ever disagree about the shape, this fails instead of removing
    // a key nothing reads.
    const detailKey = deploymentQueries.detail(org, API_ID, DEPLOYMENT_ID).queryKey;
    queryClient.setQueryData(detailKey, aDeployment({ deploymentId: DEPLOYMENT_ID }));

    result.current.mutate({ restApiId: API_ID, deploymentId: DEPLOYMENT_ID });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(queryClient.getQueryData(detailKey)).toBeUndefined();
  });
});
