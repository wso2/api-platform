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

import { beforeEach, describe, expect, it } from 'vitest';

import {
  aCustomPolicy,
  accepts,
  collection,
  failure,
  noContent,
  recorder,
  resource,
  type Recorder,
} from '../../../test/msw';
import { server } from '../../../test/server';
import { ApiError } from '../../core/errors';
import { resetHttpClient } from '../../core/http';
import {
  deleteGatewayCustomPolicy,
  getGatewayCustomPolicy,
  listGatewayCustomPolicies,
  syncCustomPolicy,
} from './gatewayCustomPolicies.endpoints';

/**
 * Contract tests for `/gateway-custom-policies`.
 *
 * Two things make this resource unlike the others, and both are pinned below:
 * policies are addressed by **id *and* version**, and `syncCustomPolicy` is a
 * POST carrying **three required query parameters** with no path segment and no
 * body — a shape nothing else in the layer has.
 */

const POLICY_ID = 'rate-limit';
const VERSION = 'v1';
const RESOURCE = `/gateway-custom-policies/${POLICY_ID}/versions/${VERSION}`;

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('listGatewayCustomPolicies', () => {
  it('GETs the collection', async () => {
    server.use(
      collection('/gateway-custom-policies', [aCustomPolicy()], { record: requests })
    );

    await listGatewayCustomPolicies();

    expect(requests.last()?.method).toBe('GET');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/gateway-custom-policies');
  });

  it('passes paging parameters through', async () => {
    server.use(collection('/gateway-custom-policies', [], { record: requests }));

    await listGatewayCustomPolicies({ query: { limit: 25, offset: 25 } });

    expect(Object.fromEntries(requests.last()!.params)).toEqual({
      limit: '25',
      offset: '25',
    });
  });
});

describe('getGatewayCustomPolicy', () => {
  it('addresses a policy by id and version, not by id alone', async () => {
    // A bare id does not identify a definition; two versions of one policy are
    // different resources.
    server.use(resource(RESOURCE, aCustomPolicy(), { record: requests }));

    await getGatewayCustomPolicy(POLICY_ID, VERSION);

    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/gateway-custom-policies/rate-limit/versions/v1'
    );
  });

  it('encodes both the id and the version', async () => {
    server.use(
      resource(
        '/gateway-custom-policies/:gatewayCustomPolicyId/versions/:version',
        aCustomPolicy(),
        { record: requests }
      )
    );

    await getGatewayCustomPolicy('a/b', '1.0 beta');

    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/gateway-custom-policies/a%2Fb/versions/1.0%20beta'
    );
  });
});

describe('syncCustomPolicy', () => {
  it('POSTs to the sync path', async () => {
    server.use(
      accepts('post', '/gateway-custom-policies/sync', aCustomPolicy(), {
        record: requests,
        status: 200,
      })
    );

    await syncCustomPolicy({
      query: { gatewayId: 'shared-gateway', policyName: 'rate-limit', policyVersion: 'v1' },
    });

    expect(requests.last()?.method).toBe('POST');
    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/gateway-custom-policies/sync'
    );
  });

  it('carries all three identifiers as query parameters, with no body', async () => {
    // Unusual shape: the operation is fully specified by the query string.
    // A caller that put these in a body would get a 400 with no hint why.
    server.use(
      accepts('post', '/gateway-custom-policies/sync', aCustomPolicy(), {
        record: requests,
        status: 200,
      })
    );

    await syncCustomPolicy({
      query: { gatewayId: 'shared-gateway', policyName: 'rate-limit', policyVersion: 'v1' },
    });

    expect(Object.fromEntries(requests.last()!.params)).toEqual({
      gatewayId: 'shared-gateway',
      policyName: 'rate-limit',
      policyVersion: 'v1',
    });
    expect(requests.last()?.body).toBe('');
  });

  it('returns the synced policy definition', async () => {
    server.use(
      accepts('post', '/gateway-custom-policies/sync', aCustomPolicy({ version: 'v2' }), {
        status: 200,
      })
    );

    await expect(
      syncCustomPolicy({
        query: { gatewayId: 'g', policyName: 'p', policyVersion: 'v2' },
      })
    ).resolves.toMatchObject({ version: 'v2' });
  });
});

describe('deleteGatewayCustomPolicy', () => {
  it('DELETEs one version, addressed by id and version', async () => {
    server.use(noContent('delete', RESOURCE, { record: requests }));

    await deleteGatewayCustomPolicy(POLICY_ID, VERSION);

    expect(requests.last()?.method).toBe('DELETE');
    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/gateway-custom-policies/rate-limit/versions/v1'
    );
  });

  it('resolves to nothing on 204', async () => {
    server.use(noContent('delete', RESOURCE));

    await expect(
      deleteGatewayCustomPolicy(POLICY_ID, VERSION)
    ).resolves.toBeUndefined();
  });
});

describe('failures', () => {
  it('surfaces the invalid-state code when a policy cannot be synced', async () => {
    server.use(
      failure('post', '/gateway-custom-policies/sync', 409, 'POLICY_INVALID_STATE')
    );

    const error = (await syncCustomPolicy({
      query: { gatewayId: 'g', policyName: 'p', policyVersion: 'v1' },
    }).catch((e: unknown) => e)) as ApiError;

    expect(error.code).toBe('POLICY_INVALID_STATE');
  });

  it('labels the failing operation', async () => {
    server.use(failure('get', RESOURCE, 404, 'NOT_FOUND'));

    const error = (await getGatewayCustomPolicy(POLICY_ID, VERSION).catch(
      (e: unknown) => e
    )) as ApiError;

    expect(error.operation).toBe('GetGatewayCustomPolicy');
  });
});
