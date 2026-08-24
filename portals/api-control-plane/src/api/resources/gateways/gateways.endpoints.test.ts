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
  aCreateGatewayBody,
  aGateway,
  accepts,
  collection,
  failure,
  listEnvelope,
  noContent,
  recorder,
  resource,
  type Recorder,
} from '../../../test/msw';
import { server } from '../../../test/server';
import { ApiError } from '../../core/errors';
import { resetHttpClient } from '../../core/http';
import {
  createGateway,
  deleteGateway,
  getGateway,
  getGatewayManifest,
  listGatewayTokens,
  listGateways,
  revokeGatewayToken,
  rotateGatewayToken,
  updateGateway,
} from './gateways.endpoints';

/**
 * Contract tests for `/gateways` and its `tokens` / `manifest` sub-resources.
 *
 * The sub-resources are the interesting part: they hang off one gateway, and
 * `rotateGatewayToken` returns a plaintext credential that is never retrievable
 * again. The tests below pin the URLs and make that one-shot property explicit,
 * since it is the reason the hook layer deliberately does not cache the result.
 */

const GATEWAY_ID = 'shared-gateway';
const RESOURCE = `/gateways/${GATEWAY_ID}`;

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('listGateways', () => {
  it('GETs the collection', async () => {
    server.use(collection('/gateways', [aGateway()], { record: requests }));

    await listGateways();

    expect(requests.last()?.method).toBe('GET');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/gateways');
  });

  it('passes paging, sorting and search parameters through', async () => {
    server.use(collection('/gateways', [], { record: requests }));

    await listGateways({
      query: { limit: 5, offset: 5, sortBy: 'name', sortOrder: 'desc', query: 'prod' },
    });

    expect(Object.fromEntries(requests.last()!.params)).toEqual({
      limit: '5',
      offset: '5',
      query: 'prod',
      sortBy: 'name',
      sortOrder: 'desc',
    });
  });

  it('scopes the request to the organization', async () => {
    server.use(collection('/gateways', [], { record: requests }));

    await listGateways({ orgId: 'acme-org' });

    expect(requests.last()?.headers.get('X-Org-Id')).toBe('acme-org');
  });
});

describe('getGateway', () => {
  it('GETs one gateway by id', async () => {
    server.use(resource(RESOURCE, aGateway(), { record: requests }));

    await getGateway(GATEWAY_ID);

    expect(requests.last()?.url.pathname).toBe('/api/v0.9/gateways/shared-gateway');
  });

  it('reports connection state, which the setup screen polls on', async () => {
    server.use(resource(RESOURCE, aGateway({ isActive: false })));

    await expect(getGateway(GATEWAY_ID)).resolves.toMatchObject({ isActive: false });
  });

  it('percent-encodes the id', async () => {
    server.use(resource('/gateways/:gatewayId', aGateway(), { record: requests }));

    await getGateway('weird/id');

    expect(requests.last()?.url.pathname).toBe('/api/v0.9/gateways/weird%2Fid');
  });
});

describe('createGateway / updateGateway / deleteGateway', () => {
  it('POSTs a new gateway with the request body', async () => {
    server.use(accepts('post', '/gateways', aGateway(), { record: requests }));

    await createGateway(aCreateGatewayBody({ displayName: 'Edge Gateway' }));

    expect(requests.last()?.method).toBe('POST');
    expect(JSON.parse(requests.last()!.body)).toMatchObject({
      displayName: 'Edge Gateway',
    });
  });

  it('PUTs an update to the resource path', async () => {
    server.use(accepts('put', RESOURCE, aGateway(), { record: requests }));

    await updateGateway(GATEWAY_ID, aGateway({ displayName: 'Renamed' }));

    expect(requests.last()?.method).toBe('PUT');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/gateways/shared-gateway');
  });

  it('DELETEs and resolves to nothing on 204', async () => {
    server.use(noContent('delete', RESOURCE, { record: requests }));

    await expect(deleteGateway(GATEWAY_ID)).resolves.toBeUndefined();
    expect(requests.last()?.method).toBe('DELETE');
  });
});

describe('tokens', () => {
  it('GETs the tokens of one gateway', async () => {
    server.use(
      resource(`${RESOURCE}/tokens`, listEnvelope([]), { record: requests })
    );

    await listGatewayTokens(GATEWAY_ID);

    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/gateways/shared-gateway/tokens'
    );
  });

  it('POSTs to rotate, with no request body', async () => {
    // The gateway is identified entirely by the path; there is nothing to send.
    server.use(
      accepts('post', `${RESOURCE}/tokens`, { token: 'plaintext-once' }, {
        record: requests,
      })
    );

    await rotateGatewayToken(GATEWAY_ID);

    expect(requests.last()?.method).toBe('POST');
    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/gateways/shared-gateway/tokens'
    );
    expect(requests.last()?.body).toBe('');
  });

  it('returns the plaintext token, which the server will never send again', async () => {
    // This is why the hook layer does not write the result into the cache: it
    // is a one-shot secret, not resource state.
    server.use(
      accepts('post', `${RESOURCE}/tokens`, { token: 'plaintext-once' })
    );

    await expect(rotateGatewayToken(GATEWAY_ID)).resolves.toMatchObject({
      token: 'plaintext-once',
    });
  });

  it('DELETEs one token beneath its gateway', async () => {
    server.use(
      noContent('delete', `${RESOURCE}/tokens/token-1`, { record: requests })
    );

    await revokeGatewayToken(GATEWAY_ID, 'token-1');

    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/gateways/shared-gateway/tokens/token-1'
    );
  });

  it('encodes both segments of the token path', async () => {
    server.use(
      noContent('delete', '/gateways/:gatewayId/tokens/:tokenId', {
        record: requests,
      })
    );

    await revokeGatewayToken('a/b', 'c d');

    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/gateways/a%2Fb/tokens/c%20d'
    );
  });
});

describe('manifest', () => {
  it('GETs the manifest of one gateway', async () => {
    server.use(
      resource(`${RESOURCE}/manifest`, { apis: [] }, { record: requests })
    );

    await getGatewayManifest(GATEWAY_ID);

    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/gateways/shared-gateway/manifest'
    );
  });
});

describe('failures', () => {
  it('surfaces the gateway-not-found code', async () => {
    server.use(failure('get', RESOURCE, 404, 'GATEWAY_NOT_FOUND'));

    const error = (await getGateway(GATEWAY_ID).catch((e: unknown) => e)) as ApiError;

    expect(error.code).toBe('GATEWAY_NOT_FOUND');
    expect(error.isNotFound).toBe(true);
  });

  it('labels each sub-resource action distinctly for logs', async () => {
    server.use(failure('post', `${RESOURCE}/tokens`, 403, 'FORBIDDEN'));

    const error = (await rotateGatewayToken(GATEWAY_ID).catch(
      (e: unknown) => e
    )) as ApiError;

    expect(error.operation).toBe('rotateGatewayToken');
  });
});
