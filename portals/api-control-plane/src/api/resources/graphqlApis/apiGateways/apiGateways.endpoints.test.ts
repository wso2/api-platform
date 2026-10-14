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

import { aGateway, collection, recorder, type Recorder } from '../../../../test/msw';
import { server } from '../../../../test/server';
import { resetHttpClient } from '../../../core/http';
import { listGraphQLApiGateways } from './apiGateways.endpoints';

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
