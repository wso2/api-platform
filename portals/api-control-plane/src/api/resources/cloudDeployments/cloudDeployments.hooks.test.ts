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
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { resetHttpClient } from '@/api/core/http';
import { runtimeConfig } from '@/config/runtime';
import { failure, recorder, resource, type Recorder } from '@/test/msw';
import { renderApiHook, settle, TEST_ORG } from '@/test/renderApiHook';
import { server } from '@/test/server';

import { useCloudDeployments } from './cloudDeployments.hooks';
import { cloudDeploymentKeys } from './cloudDeployments.queries';

const PATH = '/projects/retail/apis/pizza-shack/deployments';

let requests: Recorder;
const original = runtimeConfig.cloudProxyEnabled;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
  runtimeConfig.cloudProxyEnabled = true;
});

afterEach(() => {
  runtimeConfig.cloudProxyEnabled = original;
});

const stages = {
  list: [
    {
      environment: 'development',
      gateways: [
        { gatewayId: 'gw-1', deploymentId: 'd-1', status: 'DEPLOYED', createdAt: '2026-11-08T10:00:00Z' },
        { gatewayId: 'gw-2', status: 'NOT_DEPLOYED' },
      ],
    },
    { environment: 'qa', gateways: [{ gatewayId: 'gw-3', status: 'DEPLOYING' }] },
    { environment: 'production' },
  ],
};

describe('useCloudDeployments', () => {
  it('is off outside cloud, so a standalone console sends no request', async () => {
    runtimeConfig.cloudProxyEnabled = false;
    server.use(resource(PATH, stages, { record: requests }));

    const { result } = renderApiHook(() => useCloudDeployments('RestApi', 'retail', 'pizza-shack'));

    await settle();
    expect(requests.count()).toBe(0);
    expect(result.current.data).toBeUndefined();
  });

  it('sends no request until the project and the API are known', async () => {
    server.use(resource(PATH, stages, { record: requests }));

    renderApiHook(() => useCloudDeployments('RestApi', undefined, 'pizza-shack'));
    renderApiHook(() => useCloudDeployments('RestApi', 'retail', undefined));

    await settle();
    expect(requests.count()).toBe(0);
  });

  it('reads the project pipeline route for the kind, scoped to the organization', async () => {
    server.use(resource(PATH, stages, { record: requests }));

    const { result } = renderApiHook(() => useCloudDeployments('GraphQLApi', 'retail', 'pizza-shack'));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(requests.last()?.params.get('kind')).toBe('GraphQLApi');
    expect(requests.last()?.headers.get('X-Org-Id')).toBe(TEST_ORG);
  });

  it('flattens every environment and drops gateways with no deployment', async () => {
    server.use(resource(PATH, stages));

    const { result } = renderApiHook(() => useCloudDeployments('RestApi', 'retail', 'pizza-shack'));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual([
      { gatewayId: 'gw-1', deploymentId: 'd-1', status: 'DEPLOYED', createdAt: '2026-11-08T10:00:00Z' },
      // No deployment id or timestamp on the wire: the gateway handle and '' stand in.
      { gatewayId: 'gw-3', deploymentId: 'gw-3', status: 'DEPLOYING', createdAt: '' },
    ]);
  });

  it('files the result under the organization, so an organization switch evicts it', async () => {
    server.use(resource(PATH, stages));

    const { result, queryClient, org } = renderApiHook(() =>
      useCloudDeployments('RestApi', 'retail', 'pizza-shack'),
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(
      queryClient.getQueryData(
        cloudDeploymentKeys.list(org, { project: 'retail', api: 'pizza-shack', kind: 'RestApi' }),
      ),
    ).toEqual(result.current.data);
  });

  it('yields an empty list when the pipeline reports nothing', async () => {
    server.use(resource(PATH, { list: [] }));

    const { result } = renderApiHook(() => useCloudDeployments('RestApi', 'retail', 'pizza-shack'));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual([]);
  });

  it('leaves data undefined on a refused call, which lets the page keep the platform list', async () => {
    server.use(failure('get', PATH, 403, 'FORBIDDEN'));

    const { result } = renderApiHook(() => useCloudDeployments('RestApi', 'retail', 'pizza-shack'));

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.data).toBeUndefined();
  });
});
