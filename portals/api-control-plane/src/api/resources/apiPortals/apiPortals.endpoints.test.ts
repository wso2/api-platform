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

import { collection, recorder, type Recorder } from '../../../test/msw';
import { server } from '../../../test/server';
import { resetHttpClient } from '../../core/http';
import { listApiPortals } from './apiPortals.endpoints';

/**
 * Contract tests for `/api-portals`. Pins the path, the paging/sort/search
 * passthrough, and the org header — a rename or drift on any of those would
 * silently shift the org-overview count off the real resource.
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
