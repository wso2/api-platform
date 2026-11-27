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

import { accepts, failure, recorder, type Recorder } from '../../../test/msw';
import { server } from '../../../test/server';
import { ApiError } from '../../core/errors';
import { resetHttpClient } from '../../core/http';
import { invokeApiOnGateway } from './gatewayRelay.endpoints';

/** Contract tests for `POST /gateway-relay/invoke`. */

const INVOKE = '/gateway-relay/invoke';
const relayed = {
  outcome: 'response',
  response: {
    status: 502,
    statusText: 'Bad Gateway',
    headers: [],
    body: '',
    bodyEncoding: 'utf8',
    truncated: false,
    durationMs: 3,
  },
};

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('invokeApiOnGateway', () => {
  it('POSTs a REST API test request, naming the API and gateway in the body', async () => {
    server.use(accepts('post', INVOKE, relayed, { record: requests, status: 200 }));
    const body = {
      apiType: 'rest-api' as const,
      apiId: 'pizza-shack',
      gatewayId: 'prod-gw',
      method: 'GET',
      path: '/orders',
    };

    await invokeApiOnGateway(body);

    expect(requests.last()?.method).toBe('POST');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/gateway-relay/invoke');
    expect(JSON.parse(requests.last()!.body)).toEqual(body);
  });

  it('POSTs a GraphQL API test request to the same operation', async () => {
    server.use(accepts('post', INVOKE, relayed, { record: requests, status: 200 }));
    const body = {
      apiType: 'graphql-api' as const,
      apiId: 'countries-api',
      gatewayId: 'prod-gw',
      body: '{"query":"{ __typename }"}',
    };

    await invokeApiOnGateway(body);

    expect(requests.last()?.url.pathname).toBe('/api/v0.9/gateway-relay/invoke');
    expect(JSON.parse(requests.last()!.body)).toEqual(body);
  });

  it('resolves with a gateway error status: the relay itself succeeded', async () => {
    server.use(accepts('post', INVOKE, relayed, { status: 200 }));

    const result = await invokeApiOnGateway({
      apiType: 'rest-api',
      apiId: 'pizza-shack',
      gatewayId: 'prod-gw',
    });

    expect(result.response.status).toBe(502);
  });

  it('rejects with the code when the request was not relayed', async () => {
    server.use(failure('post', INVOKE, 403, 'REST_API_INVOKE_TARGET_NOT_ALLOWED'));

    const error = await invokeApiOnGateway({
      apiType: 'rest-api',
      apiId: 'pizza-shack',
      gatewayId: 'prod-gw',
    }).catch((e: unknown) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).code).toBe('REST_API_INVOKE_TARGET_NOT_ALLOWED');
  });
});
