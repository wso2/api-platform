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

import { http as mswHttp, HttpResponse } from 'msw';
import { beforeEach, describe, expect, it } from 'vitest';

import { accepts, recorder, type Recorder } from '../../../../test/msw';
import { renderApiHook, TEST_ORG } from '../../../../test/renderApiHook';
import { server } from '../../../../test/server';
import { isApiError } from '../../../core/errors';
import { resetHttpClient } from '../../../core/http';
import { useInvokeRestApiOnGateway } from './apiGateways.hooks';

const INVOKE = '/gateway-relay/invoke';
const variables = {
  body: { restApiId: 'pizza-shack', gatewayId: 'prod-gw', method: 'GET', path: '/orders' },
};

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('useInvokeRestApiOnGateway', () => {
  it('sends the test request in the page’s organization', async () => {
    const relayed = {
      outcome: 'response',
      response: {
        status: 200,
        statusText: 'OK',
        headers: [],
        body: '[]',
        bodyEncoding: 'utf8',
        truncated: false,
        durationMs: 4,
      },
    };
    server.use(accepts('post', INVOKE, relayed, { record: requests, status: 200 }));

    const { result } = renderApiHook(() => useInvokeRestApiOnGateway());
    const response = await result.current.mutateAsync(variables);

    expect(response.response.body).toBe('[]');
    expect(requests.last()?.headers.get('X-Org-Id')).toBe(TEST_ORG);
  });

  it('never resends a test request on its own, even one that never left', async () => {
    let sent = 0;
    server.use(
      mswHttp.post(`${window.location.origin}/api/v0.9${INVOKE}`, () => {
        sent += 1;
        return HttpResponse.error();
      }),
    );

    const { result } = renderApiHook(() => useInvokeRestApiOnGateway());
    const error = await result.current.mutateAsync(variables).catch((e: unknown) => e);

    expect(isApiError(error) && error.kind).toBe('network');
    expect(sent).toBe(1);
  });
});
