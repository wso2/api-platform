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
  anOrganization,
  collection,
  failure,
  recorder,
  resource,
  type Recorder,
} from '../../../test/msw';
import { server } from '../../../test/server';
import { ApiError } from '../../core/errors';
import { resetHttpClient, type RequestOptions } from '../../core/http';
import {
  getOrganization,
  listOrganizations,
  registerOrganization,
} from './organizations.endpoints';

/**
 * Contract tests for `/organizations`.
 *
 * The distinguishing property of this resource is that it is **not
 * organization-scoped**: the list is what populates the org switcher, so it is
 * read before any organization is active. Sending `X-Org-Id` here would be
 * asking the server to scope a request to the very thing being looked up.
 */

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

/**
 * An org scope these endpoints must drop, plus a second option that must
 * survive the dropping.
 *
 * Deliberately typed as the full `RequestOptions` instead of cast: that is how
 * a stray scope actually arrives, since the endpoints' narrower
 * `Omit<RequestOptions, 'orgId'>` parameter still accepts a wider value
 * structurally. Only the runtime strip keeps the header off the wire.
 */
const strayOrgScope: RequestOptions = {
  orgId: 'acme-org',
  query: { limit: 5 },
};

describe('listOrganizations', () => {
  it('GETs the collection', async () => {
    server.use(collection('/organizations', [anOrganization()], { record: requests }));

    await listOrganizations();

    expect(requests.last()?.method).toBe('GET');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/organizations');
  });

  it('sends no organization header, because none is known yet', async () => {
    // This is the one endpoint that must work before scope exists. A stray
    // X-Org-Id here would be scoping the lookup to its own answer.
    server.use(collection('/organizations', [], { record: requests }));

    await listOrganizations();

    expect(requests.last()?.headers.get('X-Org-Id')).toBeNull();
  });

  it('passes paging parameters through', async () => {
    server.use(collection('/organizations', [], { record: requests }));

    await listOrganizations({ query: { limit: 50, offset: 100 } });

    expect(Object.fromEntries(requests.last()!.params)).toEqual({
      limit: '50',
      offset: '100',
    });
  });

  it('drops a caller-supplied organization scope, keeping the rest', async () => {
    server.use(collection('/organizations', [], { record: requests }));

    await listOrganizations(strayOrgScope);

    expect(requests.last()?.headers.get('X-Org-Id')).toBeNull();
    expect(Object.fromEntries(requests.last()!.params)).toEqual({ limit: '5' });
  });

  it('returns the collection envelope, pagination included', async () => {
    server.use(
      collection('/organizations', [anOrganization(), anOrganization({ id: 'globex' })])
    );

    const response = await listOrganizations();

    expect(response.list).toHaveLength(2);
    expect(response.pagination).toMatchObject({ total: 2 });
  });
});

describe('getOrganization', () => {
  it('GETs one organization by id', async () => {
    server.use(
      resource('/organizations/acme-org', anOrganization(), { record: requests })
    );

    await getOrganization('acme-org');

    expect(requests.last()?.url.pathname).toBe('/api/v0.9/organizations/acme-org');
  });

  it('percent-encodes the id', async () => {
    server.use(
      resource('/organizations/:organizationId', anOrganization(), {
        record: requests,
      })
    );

    await getOrganization('weird/id');

    expect(requests.last()?.url.pathname).toBe('/api/v0.9/organizations/weird%2Fid');
  });

  it('drops a caller-supplied organization scope, keeping the rest', async () => {
    server.use(
      resource('/organizations/acme-org', anOrganization(), { record: requests })
    );

    await getOrganization('acme-org', strayOrgScope);

    expect(requests.last()?.headers.get('X-Org-Id')).toBeNull();
    expect(Object.fromEntries(requests.last()!.params)).toEqual({ limit: '5' });
  });

  it('resolves to the organization itself', async () => {
    server.use(resource('/organizations/acme-org', anOrganization()));

    await expect(getOrganization('acme-org')).resolves.toMatchObject({
      displayName: 'Acme Corp',
      id: 'acme-org',
    });
  });
});

describe('registerOrganization', () => {
  it('POSTs to the collection with the request body', async () => {
    server.use(
      accepts('post', '/organizations', anOrganization(), { record: requests })
    );

    await registerOrganization(anOrganization({ displayName: 'New Corp' }));

    expect(requests.last()?.method).toBe('POST');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/organizations');
    expect(JSON.parse(requests.last()!.body)).toMatchObject({
      displayName: 'New Corp',
    });
  });

  it('drops a caller-supplied organization scope, keeping the rest', async () => {
    server.use(
      accepts('post', '/organizations', anOrganization(), { record: requests })
    );

    await registerOrganization(anOrganization(), strayOrgScope);

    expect(requests.last()?.headers.get('X-Org-Id')).toBeNull();
    expect(Object.fromEntries(requests.last()!.params)).toEqual({ limit: '5' });
  });

  it('returns the registered organization, so onboarding can route into it', async () => {
    server.use(accepts('post', '/organizations', anOrganization({ id: 'new-org' })));

    await expect(registerOrganization(anOrganization())).resolves.toMatchObject({
      id: 'new-org',
    });
  });
});

describe('failures', () => {
  it('surfaces a conflict when the handle is taken', async () => {
    server.use(
      failure('post', '/organizations', 409, 'CONFLICT', {
        message: 'An organization with that handle already exists.',
      })
    );

    const error = (await registerOrganization(anOrganization()).catch(
      (e: unknown) => e
    )) as ApiError;

    expect(error.code).toBe('CONFLICT');
    expect(error.isConflict).toBe(true);
  });

  it('labels the failing operation', async () => {
    server.use(failure('get', '/organizations/acme-org', 404, 'NOT_FOUND'));

    const error = (await getOrganization('acme-org').catch(
      (e: unknown) => e
    )) as ApiError;

    expect(error.operation).toBe('GetOrganization');
  });
});
