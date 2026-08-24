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
  accepts,
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
  createApiKey,
  listMyApiKeys,
  revokeApiKey,
  updateApiKey,
} from './apiKeys.endpoints';

/**
 * Contract tests for API keys.
 *
 * This resource is asymmetric by necessity, and the tests make that visible:
 * **writes** are scoped to one REST API (`/rest-apis/{id}/api-keys`), while the
 * only **read** is the caller-scoped `/me/api-keys`. The spec offers no way to
 * list the keys of a single REST API, so no test here can assert one — that is
 * a platform-api gap, not an omission.
 */

const API_ID = 'pizza-shack';
const COLLECTION = `/rest-apis/${API_ID}/api-keys`;

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('listMyApiKeys', () => {
  it('GETs the caller-scoped collection, not a per-API one', async () => {
    server.use(resource('/me/api-keys', listEnvelope([]), { record: requests }));

    await listMyApiKeys();

    expect(requests.last()?.method).toBe('GET');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/me/api-keys');
  });

  it('passes the artifact-type filter and paging through', async () => {
    server.use(resource('/me/api-keys', listEnvelope([]), { record: requests }));

    await listMyApiKeys({ query: { type: ['RestApi'], limit: 10 } });

    expect(requests.last()?.params.getAll('type')).toEqual(['RestApi']);
    expect(requests.last()?.params.get('limit')).toBe('10');
  });

  it('repeats the key for a multi-valued type filter', async () => {
    // The spec types `type` as an array; collapsing it to a comma-joined string
    // would filter on a value the server does not recognise.
    server.use(resource('/me/api-keys', listEnvelope([]), { record: requests }));

    await listMyApiKeys({ query: { type: ['RestApi', 'LlmProxy'] } });

    expect(requests.last()?.params.getAll('type')).toEqual(['RestApi', 'LlmProxy']);
  });

  it('scopes the request to the organization', async () => {
    server.use(resource('/me/api-keys', listEnvelope([]), { record: requests }));

    await listMyApiKeys({ orgId: 'acme-org' });

    expect(requests.last()?.headers.get('X-Org-Id')).toBe('acme-org');
  });
});

describe('createApiKey', () => {
  it('POSTs beneath the owning REST API', async () => {
    server.use(
      accepts('post', COLLECTION, { status: 'success', message: 'created' }, {
        record: requests,
      })
    );

    await createApiKey(API_ID, { name: 'ci-key' } as never);

    expect(requests.last()?.method).toBe('POST');
    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/rest-apis/pizza-shack/api-keys'
    );
    expect(JSON.parse(requests.last()!.body)).toMatchObject({ name: 'ci-key' });
  });

  it('percent-encodes the parent API handle', async () => {
    server.use(
      accepts('post', '/rest-apis/:restApiId/api-keys', { status: 'success' }, {
        record: requests,
      })
    );

    await createApiKey('weird/handle', { name: 'k' } as never);

    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/rest-apis/weird%2Fhandle/api-keys'
    );
  });
});

describe('updateApiKey', () => {
  it('PUTs to the key beneath its API', async () => {
    server.use(
      accepts('put', `${COLLECTION}/key-1`, { status: 'success' }, {
        record: requests,
      })
    );

    await updateApiKey(API_ID, 'key-1', { name: 'renamed' } as never);

    expect(requests.last()?.method).toBe('PUT');
    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/rest-apis/pizza-shack/api-keys/key-1'
    );
  });

  it('encodes both path segments', async () => {
    server.use(
      accepts('put', '/rest-apis/:restApiId/api-keys/:apiKeyId', { status: 'success' }, {
        record: requests,
      })
    );

    await updateApiKey('a/b', 'c d', {} as never);

    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/rest-apis/a%2Fb/api-keys/c%20d'
    );
  });
});

describe('revokeApiKey', () => {
  it('DELETEs the key and resolves to nothing on 204', async () => {
    server.use(noContent('delete', `${COLLECTION}/key-1`, { record: requests }));

    await expect(revokeApiKey(API_ID, 'key-1')).resolves.toBeUndefined();
    expect(requests.last()?.method).toBe('DELETE');
    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/rest-apis/pizza-shack/api-keys/key-1'
    );
  });

  it('rejects rather than resolving when revocation fails', async () => {
    // A revoke that silently "succeeded" would leave a live credential the UI
    // shows as gone — the worst possible outcome for this operation.
    server.use(failure('delete', `${COLLECTION}/key-1`, 403, 'FORBIDDEN'));

    await expect(revokeApiKey(API_ID, 'key-1')).rejects.toBeInstanceOf(ApiError);
  });
});

describe('failures', () => {
  it('labels each action distinctly for logs', async () => {
    server.use(failure('post', COLLECTION, 409, 'CONFLICT'));

    const error = (await createApiKey(API_ID, {} as never).catch(
      (e: unknown) => e
    )) as ApiError;

    expect(error.operation).toBe('CreateAPIKey');
  });
});
