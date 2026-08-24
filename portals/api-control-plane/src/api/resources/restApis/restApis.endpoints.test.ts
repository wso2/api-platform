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

import { server } from '../../../test/server';
import {
  aRestApi,
  accepts,
  collection,
  failure,
  noContent,
  recorder,
  resource,
  type Recorder,
} from '../../../test/msw';
import { ApiError } from '../../core/errors';
import { resetHttpClient } from '../../core/http';
import {
  createRestApi,
  deleteRestApi,
  getRestApi,
  listRestApis,
  updateRestApi,
} from './restApis.endpoints';

/**
 * Contract tests for the `/rest-apis` transport functions.
 *
 * These cover what is specific to *this* resource — which method, which URL,
 * where each argument lands, and what comes back. Behaviour shared by every
 * endpoint (CSRF headers, status-to-error mapping, timeouts, cancellation)
 * belongs to the transport and is covered once in `core/http.test.ts`; there is
 * no value in re-asserting it per resource.
 *
 * No React and no query client here: an endpoint is a plain async function, so
 * a failure in this file is unambiguously a transport-layer problem.
 */

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('listRestApis', () => {
  it('GETs the collection', async () => {
    server.use(collection('/rest-apis', [aRestApi()], { record: requests }));

    await listRestApis({ query: { projectId: 'retail' } });

    expect(requests.last()?.method).toBe('GET');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/rest-apis');
  });

  it('sends every list parameter on the query string', async () => {
    // These are what make server-side paging and search work at all; a silently
    // dropped parameter looks like an empty result, not an error.
    server.use(collection('/rest-apis', [], { record: requests }));

    await listRestApis({
      query: {
        projectId: 'retail',
        limit: 12,
        offset: 24,
        sortBy: 'createdAt',
        sortOrder: 'desc',
        query: 'pizza',
      },
    });

    expect(Object.fromEntries(requests.last()!.params)).toEqual({
      limit: '12',
      offset: '24',
      projectId: 'retail',
      query: 'pizza',
      sortBy: 'createdAt',
      sortOrder: 'desc',
    });
  });

  it('scopes the request to the caller’s organization', async () => {
    server.use(collection('/rest-apis', [], { record: requests }));

    await listRestApis({ orgId: 'acme-org', query: { projectId: 'retail' } });

    expect(requests.last()?.headers.get('X-Org-Id')).toBe('acme-org');
  });

  it('returns the collection envelope, pagination included', async () => {
    // The page reads `pagination.total`; unwrapping to a bare array here would
    // silently remove the only source of a correct count.
    server.use(collection('/rest-apis', [aRestApi(), aRestApi({ id: 'other' })]));

    const response = await listRestApis({ query: { projectId: 'retail' } });

    expect(response.list).toHaveLength(2);
    expect(response.pagination).toMatchObject({ total: 2 });
  });
});

describe('getRestApi', () => {
  it('GETs one API by handle', async () => {
    server.use(
      resource('/rest-apis/pizza-shack', aRestApi(), { record: requests })
    );

    await getRestApi('pizza-shack');

    expect(requests.last()?.method).toBe('GET');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/rest-apis/pizza-shack');
  });

  it('percent-encodes a handle so it cannot alter the path', async () => {
    // Handles are user-supplied. An unencoded "a/b" would address a different
    // resource entirely, and "?x=1" would inject a query parameter.
    server.use(
      resource('/rest-apis/:restApiId', aRestApi(), { record: requests })
    );

    await getRestApi('weird/handle?x=1');

    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/rest-apis/weird%2Fhandle%3Fx%3D1'
    );
    expect(requests.last()?.params.get('x')).toBeNull();
  });

  it('resolves to the API itself, not a wrapper', async () => {
    server.use(resource('/rest-apis/pizza-shack', aRestApi()));

    await expect(getRestApi('pizza-shack')).resolves.toMatchObject({
      displayName: 'Pizza Shack',
      id: 'pizza-shack',
    });
  });
});

describe('createRestApi', () => {
  it('POSTs to the collection with the request body', async () => {
    server.use(accepts('post', '/rest-apis', aRestApi(), { record: requests }));

    await createRestApi(aRestApi({ id: undefined, displayName: 'New API' }));

    expect(requests.last()?.method).toBe('POST');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/rest-apis');
    expect(JSON.parse(requests.last()!.body)).toMatchObject({
      displayName: 'New API',
    });
  });

  it('returns the created API, so the caller need not re-read it', async () => {
    server.use(accepts('post', '/rest-apis', aRestApi({ id: 'new-api' })));

    await expect(createRestApi(aRestApi())).resolves.toMatchObject({
      id: 'new-api',
    });
  });
});

describe('updateRestApi', () => {
  it('PUTs to the resource path with the request body', async () => {
    server.use(
      accepts('put', '/rest-apis/pizza-shack', aRestApi(), { record: requests })
    );

    await updateRestApi('pizza-shack', aRestApi({ displayName: 'Renamed' }));

    expect(requests.last()?.method).toBe('PUT');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/rest-apis/pizza-shack');
    expect(JSON.parse(requests.last()!.body)).toMatchObject({
      displayName: 'Renamed',
    });
  });

  it('returns the updated API', async () => {
    server.use(
      accepts('put', '/rest-apis/pizza-shack', aRestApi({ displayName: 'Renamed' }))
    );

    await expect(
      updateRestApi('pizza-shack', aRestApi())
    ).resolves.toMatchObject({ displayName: 'Renamed' });
  });
});

describe('deleteRestApi', () => {
  it('DELETEs the resource path', async () => {
    server.use(
      noContent('delete', '/rest-apis/pizza-shack', { record: requests })
    );

    await deleteRestApi('pizza-shack');

    expect(requests.last()?.method).toBe('DELETE');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/rest-apis/pizza-shack');
  });

  it('resolves to nothing on 204 rather than failing to parse an empty body', async () => {
    server.use(noContent('delete', '/rest-apis/pizza-shack'));

    await expect(deleteRestApi('pizza-shack')).resolves.toBeUndefined();
  });

  it('sends no request body', async () => {
    server.use(
      noContent('delete', '/rest-apis/pizza-shack', { record: requests })
    );

    await deleteRestApi('pizza-shack');

    expect(requests.last()?.body).toBe('');
  });
});

describe('failures', () => {
  it('labels the failing operation so a log line identifies it', async () => {
    // `operationName` is the only thing distinguishing one endpoint's failure
    // from another's once the error reaches telemetry.
    server.use(
      failure('get', '/rest-apis/pizza-shack', 404, 'REST_API_NOT_FOUND')
    );

    const error = await getRestApi('pizza-shack').catch((e: unknown) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).operation).toBe('GetRESTAPI');
  });

  it('surfaces the server’s error code rather than only a status', async () => {
    server.use(
      failure('get', '/rest-apis/pizza-shack', 404, 'REST_API_NOT_FOUND', {
        message: 'The requested REST API could not be found.',
      })
    );

    const error = (await getRestApi('pizza-shack').catch(
      (e: unknown) => e
    )) as ApiError;

    expect(error.code).toBe('REST_API_NOT_FOUND');
    expect(error.status).toBe(404);
  });

  it('rejects rather than resolving when a delete fails', async () => {
    // A void-returning endpoint must not swallow a failure into a silent
    // success — the caller would then invalidate caches for a delete that
    // never happened.
    server.use(failure('delete', '/rest-apis/pizza-shack', 409, 'CONFLICT'));

    await expect(deleteRestApi('pizza-shack')).rejects.toBeInstanceOf(ApiError);
  });
});
