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
  aCreateApiPortalBody,
  accepts,
  anApiPortal,
  collection,
  noContent,
  recorder,
  resource,
  type Recorder,
} from '../../../test/msw';
import { server } from '../../../test/server';
import { resetHttpClient } from '../../core/http';
import {
  createApiPortal,
  deleteApiPortal,
  getApiPortal,
  listApiPortals,
  updateApiPortal,
} from './apiPortals.endpoints';

/**
 * Contract tests for `/api-portals`. Pins the paths, the paging/sort/search
 * passthrough and the org header, and that the shared key travels only in
 * request bodies: it is write-only and the registry pages rely on that.
 */

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('listApiPortals', () => {
  it('GETs the collection', async () => {
    server.use(collection('/api-portals', [], { record: requests }));

    await listApiPortals();

    expect(requests.last()?.method).toBe('GET');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/api-portals');
  });

  it('passes paging, sorting and search parameters through', async () => {
    server.use(collection('/api-portals', [], { record: requests }));

    await listApiPortals({
      query: { limit: 5, offset: 10, sortBy: 'createdAt', sortOrder: 'desc', query: 'prod' },
    });

    expect(Object.fromEntries(requests.last()!.params)).toEqual({
      limit: '5',
      offset: '10',
      query: 'prod',
      sortBy: 'createdAt',
      sortOrder: 'desc',
    });
  });

  it('scopes the request to the organization', async () => {
    server.use(collection('/api-portals', [], { record: requests }));

    await listApiPortals({ orgId: 'acme-org' });

    expect(requests.last()?.headers.get('X-Org-Id')).toBe('acme-org');
  });
});

const PORTAL_ID = 'partner-portal';
const RESOURCE = `/api-portals/${PORTAL_ID}`;

describe('getApiPortal', () => {
  it('GETs one portal by handle', async () => {
    server.use(resource(RESOURCE, anApiPortal(), { record: requests }));

    await expect(getApiPortal(PORTAL_ID)).resolves.toMatchObject({ id: PORTAL_ID });
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/api-portals/partner-portal');
  });

  it('percent-encodes the handle', async () => {
    server.use(resource('/api-portals/:apiPortalId', anApiPortal(), { record: requests }));

    await getApiPortal('weird/id');

    expect(requests.last()?.url.pathname).toBe('/api/v0.9/api-portals/weird%2Fid');
  });
});

describe('createApiPortal / updateApiPortal / deleteApiPortal', () => {
  it('POSTs a new portal with its shared key in the body', async () => {
    server.use(accepts('post', '/api-portals', anApiPortal(), { record: requests }));

    await createApiPortal(aCreateApiPortalBody({ sharedKey: 'b'.repeat(64) }));

    expect(requests.last()?.method).toBe('POST');
    expect(JSON.parse(requests.last()!.body)).toMatchObject({
      handle: 'partner-portal',
      sharedKey: 'b'.repeat(64),
    });
  });

  it('PUTs an update to the resource path, rotating the key when one is given', async () => {
    server.use(accepts('put', RESOURCE, anApiPortal(), { record: requests }));

    await updateApiPortal(PORTAL_ID, { sharedKey: 'c'.repeat(64) });

    expect(requests.last()?.method).toBe('PUT');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/api-portals/partner-portal');
    expect(JSON.parse(requests.last()!.body)).toEqual({ sharedKey: 'c'.repeat(64) });
  });

  it('DELETEs the resource path', async () => {
    server.use(noContent('delete', RESOURCE, { record: requests }));

    await deleteApiPortal(PORTAL_ID);

    expect(requests.last()?.method).toBe('DELETE');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/api-portals/partner-portal');
  });
});
