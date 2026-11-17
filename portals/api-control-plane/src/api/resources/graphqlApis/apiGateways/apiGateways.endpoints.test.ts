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
import { invokeGraphQLApiOnGateway, listGraphQLApiGateways } from './apiGateways.endpoints';

/** Contract tests for `/graphql-apis/{graphqlApiId}/gateways`. */

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('listGraphQLApiGateways', () => {
  it('GETs the gateways of one GraphQL API', async () => {
    server.use(
      collection('/graphql-apis/countries-api/gateways', [aGateway()], { record: requests }),
    );

    await listGraphQLApiGateways('countries-api');

    expect(requests.last()?.method).toBe('GET');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/graphql-apis/countries-api/gateways');
  });

  it('percent-encodes the parent API handle', async () => {
    server.use(collection('/graphql-apis/:graphqlApiId/gateways', [], { record: requests }));

    await listGraphQLApiGateways('a/b');

    expect(requests.last()?.url.pathname).toBe('/api/v0.9/graphql-apis/a%2Fb/gateways');
  });
});

describe('invokeGraphQLApiOnGateway', () => {
  const INVOKE = '/gateway-relay/invoke-graphql';
  const request = {
    graphqlApiId: 'countries-api',
    gatewayId: 'prod-gw',
    body: '{"query":"{ __typename }"}',
  };
  const relayed = {
    outcome: 'response',
    response: {
      status: 200,
      statusText: 'OK',
      headers: [],
      body: '{"data":{"__typename":"Query"}}',
      bodyEncoding: 'utf8',
      truncated: false,
      durationMs: 3,
    },
  };

  it('POSTs the GraphQL request, naming the API and gateway in the body', async () => {
    server.use(accepts('post', INVOKE, relayed, { record: requests, status: 200 }));

    const result = await invokeGraphQLApiOnGateway(request);

    expect(requests.last()?.method).toBe('POST');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/gateway-relay/invoke-graphql');
    expect(JSON.parse(requests.last()!.body)).toEqual(request);
    expect(result.response.status).toBe(200);
  });

  it('rejects with the code when the request was not relayed', async () => {
    server.use(failure('post', INVOKE, 404, 'GRAPHQL_API_INVOKE_GATEWAY_NOT_DEPLOYED'));

    const error = await invokeGraphQLApiOnGateway(request).catch((e: unknown) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).code).toBe('GRAPHQL_API_INVOKE_GATEWAY_NOT_DEPLOYED');
  });
});
