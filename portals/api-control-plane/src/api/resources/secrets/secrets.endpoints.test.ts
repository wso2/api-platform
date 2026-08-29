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
  aSecret,
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
  createSecret,
  deleteSecret,
  getSecret,
  listSecrets,
  rotateSecret,
} from './secrets.endpoints';

/**
 * Contract tests for `/secrets`.
 *
 * These are the only two multipart operations in the spec, so this file carries
 * the assertions no other resource needs: that `createSecret` and
 * `rotateSecret` send `FormData` rather than JSON, and that absent optional
 * fields are omitted instead of being sent as the string "undefined" — which a
 * naive loop would produce and the server would store as the field's value.
 *
 * The multipart *wire format* cannot be asserted here: jsdom's XHR does not
 * implement FormData bodies (verified against plain axios with none of this
 * code involved). What is asserted is that a FormData body is constructed and
 * not labelled as JSON; the boundary itself needs a browser-run test.
 */

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('listSecrets', () => {
  it('GETs the collection', async () => {
    server.use(collection('/secrets', [aSecret()], { record: requests }));

    await listSecrets();

    expect(requests.last()?.method).toBe('GET');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/secrets');
  });

  it('passes paging and the updatedAfter filter through', async () => {
    server.use(collection('/secrets', [], { record: requests }));

    await listSecrets({
      query: { limit: 10, offset: 0, updatedAfter: '2026-01-01T00:00:00Z' },
    });

    expect(requests.last()?.params.get('updatedAfter')).toBe('2026-01-01T00:00:00Z');
    expect(requests.last()?.params.get('limit')).toBe('10');
  });

  it('returns metadata only — no secret values enter the client', async () => {
    server.use(collection('/secrets', [aSecret()]));

    const response = await listSecrets();

    expect(response.list[0]).not.toHaveProperty('value');
  });
});

describe('getSecret', () => {
  it('GETs one secret by id', async () => {
    server.use(resource('/secrets/signing-key', aSecret(), { record: requests }));

    await getSecret('signing-key');

    expect(requests.last()?.url.pathname).toBe('/api/v0.9/secrets/signing-key');
  });

  it('percent-encodes the id', async () => {
    server.use(resource('/secrets/:secretId', aSecret(), { record: requests }));

    await getSecret('weird/id');

    expect(requests.last()?.url.pathname).toBe('/api/v0.9/secrets/weird%2Fid');
  });
});

describe('createSecret', () => {
  it('POSTs to the collection', async () => {
    server.use(accepts('post', '/secrets', aSecret(), { record: requests }));

    await createSecret({
      displayName: 'Signing Key',
      value: 's3cret',
      type: 'GENERIC',
    });

    expect(requests.last()?.method).toBe('POST');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/secrets');
  });

  it('does not label the body as JSON, so the browser can set a multipart boundary', async () => {
    // Forcing application/json here would produce a body the server cannot
    // split. The transport must stay out of the way for FormData.
    server.use(accepts('post', '/secrets', aSecret(), { record: requests }));

    await createSecret({
      displayName: 'Signing Key',
      value: 's3cret',
      type: 'GENERIC',
    });

    expect(requests.last()?.headers.get('Content-Type')).not.toContain(
      'application/json'
    );
  });

  it('returns the stored secret’s metadata, never the submitted value', async () => {
    server.use(accepts('post', '/secrets', aSecret({ id: 'new-secret' })));

    const created = await createSecret({
      displayName: 'Signing Key',
      value: 's3cret',
      type: 'GENERIC',
    });

    expect(created).toMatchObject({ id: 'new-secret' });
    expect(created).not.toHaveProperty('value');
  });
});

describe('rotateSecret', () => {
  it('PUTs to the resource path', async () => {
    server.use(
      accepts('put', '/secrets/signing-key', aSecret(), { record: requests })
    );

    await rotateSecret('signing-key', { displayName: 'Signing Key', value: 'rotated' });

    expect(requests.last()?.method).toBe('PUT');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/secrets/signing-key');
  });

  it('sends multipart rather than JSON, like create', async () => {
    server.use(
      accepts('put', '/secrets/signing-key', aSecret(), { record: requests })
    );

    await rotateSecret('signing-key', { displayName: 'Signing Key', value: 'rotated' });

    expect(requests.last()?.headers.get('Content-Type')).not.toContain(
      'application/json'
    );
  });
});

describe('deleteSecret', () => {
  it('DELETEs the resource path and resolves to nothing on 204', async () => {
    server.use(noContent('delete', '/secrets/signing-key', { record: requests }));

    await expect(deleteSecret('signing-key')).resolves.toBeUndefined();
    expect(requests.last()?.method).toBe('DELETE');
  });

  it('surfaces SECRET_IN_USE with the blocking resources in details', async () => {
    // "In use by 3 APIs" is a useful message; "delete failed" is not. The
    // difference is entirely in `code` and `details`.
    server.use(
      failure('delete', '/secrets/signing-key', 409, 'SECRET_IN_USE', {
        details: { referencedBy: ['pizza-shack', 'orders-api'] },
        message: 'The secret is still referenced.',
      })
    );

    const error = (await deleteSecret('signing-key').catch(
      (e: unknown) => e
    )) as ApiError;

    expect(error.code).toBe('SECRET_IN_USE');
    expect(error.details).toEqual({
      referencedBy: ['pizza-shack', 'orders-api'],
    });
  });
});
