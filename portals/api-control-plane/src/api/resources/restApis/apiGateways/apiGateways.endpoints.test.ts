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
  aGateway,
  accepts,
  collection,
  failure,
  recorder,
  type Recorder,
} from '../../../../test/msw';
import { server } from '../../../../test/server';
import { ApiError } from '../../../core/errors';
import { resetHttpClient } from '../../../core/http';
import { addGatewaysToApi, listRestApiGateways } from './apiGateways.endpoints';

/**
 * Contract tests for `/rest-apis/{restApiId}/gateways` — the association
 * between one API and the gateways it can be deployed to.
 *
 * Distinct from `resources/gateways`, which manages gateways as entities. The
 * property worth pinning is that the add is a **bulk POST taking an array**,
 * and returns the API's full gateway list rather than only the additions — a
 * caller that assumed a single-object body or an incremental response would be
 * wrong in a way no type error catches at the wire.
 */

const API_ID = 'pizza-shack';
const COLLECTION = `/rest-apis/${API_ID}/gateways`;

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('listRestApiGateways', () => {
  it('GETs the gateways of one API', async () => {
    server.use(collection(COLLECTION, [aGateway()], { record: requests }));

    await listRestApiGateways(API_ID);

    expect(requests.last()?.method).toBe('GET');
    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/rest-apis/pizza-shack/gateways'
    );
  });

  it('passes paging parameters through', async () => {
    server.use(collection(COLLECTION, [], { record: requests }));

    await listRestApiGateways(API_ID, { query: { limit: 5, offset: 5 } });

    expect(Object.fromEntries(requests.last()!.params)).toEqual({
      limit: '5',
      offset: '5',
    });
  });

  it('percent-encodes the parent API handle', async () => {
    server.use(
      collection('/rest-apis/:restApiId/gateways', [], { record: requests })
    );

    await listRestApiGateways('weird/handle');

    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/rest-apis/weird%2Fhandle/gateways'
    );
  });

  it('returns the collection envelope', async () => {
    server.use(collection(COLLECTION, [aGateway(), aGateway({ id: 'edge' })]));

    const response = await listRestApiGateways(API_ID);

    expect(response.list).toHaveLength(2);
  });
});

describe('addGatewaysToApi', () => {
  it('POSTs an array body, because the spec models this as a bulk add', async () => {
    server.use(
      accepts('post', COLLECTION, { count: 0, list: [], pagination: {} }, {
        record: requests,
        status: 200,
      })
    );

    await addGatewaysToApi(API_ID, [
      { gatewayId: 'shared-gateway' },
      { gatewayId: 'edge' },
    ] as never);

    expect(requests.last()?.method).toBe('POST');
    const body: unknown = JSON.parse(requests.last()!.body);
    expect(Array.isArray(body)).toBe(true);
    expect(body).toHaveLength(2);
  });

  it('returns the API’s full gateway list, not just the additions', async () => {
    // The deploy-target picker reads this response directly; treating it as a
    // delta would drop every previously associated gateway from the UI.
    server.use(
      accepts(
        'post',
        COLLECTION,
        {
          count: 2,
          list: [aGateway(), aGateway({ id: 'edge' })],
          pagination: { total: 2, offset: 0, limit: 20 },
        },
        { status: 200 }
      )
    );

    const response = await addGatewaysToApi(API_ID, [
      { gatewayId: 'edge' },
    ] as never);

    expect(response.list).toHaveLength(2);
  });
});

describe('failures', () => {
  it('surfaces the gateway-not-found code when associating an unknown gateway', async () => {
    server.use(failure('post', COLLECTION, 404, 'GATEWAY_NOT_FOUND'));

    const error = (await addGatewaysToApi(API_ID, [] as never).catch(
      (e: unknown) => e
    )) as ApiError;

    expect(error.code).toBe('GATEWAY_NOT_FOUND');
  });

  it('labels each action distinctly for logs', async () => {
    server.use(failure('get', COLLECTION, 403, 'FORBIDDEN'));

    const error = (await listRestApiGateways(API_ID).catch(
      (e: unknown) => e
    )) as ApiError;

    expect(error.operation).toBe('GetRESTAPIGateways');
  });
});
