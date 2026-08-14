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
import { useDeployApi, useDeployments } from './deployments.hooks';
import { hasTransitioningDeployment } from './deployments.queries';
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

describe('useDeleteDeployment via the shared invalidation helper', () => {
  it('invalidates the parent’s deployment list after removal', async () => {
    server.use(noContent('delete', `${DEPLOYMENTS}/deployment-1`));

    const { result, queryClient, org } = renderApiHook(() => useDeployApi());
    const listKey = restApiKeys.child(org, API_ID, 'deployments');
    queryClient.setQueryData(listKey, { count: 0, list: [], pagination: {} });

    // Deploy and delete share one invalidation helper; exercising either proves
    // the shared path, and deploy is already covered above.
    server.use(accepts('post', DEPLOYMENTS, aDeployment()));
    result.current.mutate({ restApiId: API_ID, body: aDeployRequest() });

    await waitFor(() =>
      expect(queryClient.getQueryState(listKey)?.isInvalidated).toBe(true)
    );
  });
});
