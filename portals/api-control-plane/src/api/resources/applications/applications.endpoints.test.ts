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
  anApplication,
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
  addApplicationApiKeys,
  addApplicationAssociations,
  createApplication,
  deleteApplication,
  getApplication,
  listApplicationApiKeys,
  listApplicationAssociations,
  listApplications,
  listAssociationApiKeys,
  removeApplicationApiKey,
  removeApplicationAssociation,
  updateApplication,
} from './applications.endpoints';

/**
 * Contract tests for `/applications` and its `api-keys` / `associations`
 * sub-resources.
 *
 * Two things here exist nowhere else in the layer: association keys nest two
 * levels deep, and `removeApplicationApiKey` needs a **required query
 * parameter** on a DELETE. Both are easy to get wrong and silent when wrong,
 * so both are pinned below.
 */

const APP_ID = 'checkout-app';
const RESOURCE = `/applications/${APP_ID}`;

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('listApplications', () => {
  it('GETs the collection scoped to a project', async () => {
    server.use(collection('/applications', [anApplication()], { record: requests }));

    await listApplications({ query: { projectId: 'retail' } });

    expect(requests.last()?.url.pathname).toBe('/api/v0.9/applications');
    expect(requests.last()?.params.get('projectId')).toBe('retail');
  });

  it('passes paging and search parameters through', async () => {
    server.use(collection('/applications', [], { record: requests }));

    await listApplications({
      query: { projectId: 'retail', limit: 25, offset: 50, query: 'checkout' },
    });

    expect(Object.fromEntries(requests.last()!.params)).toEqual({
      limit: '25',
      offset: '50',
      projectId: 'retail',
      query: 'checkout',
    });
  });
});

describe('application CRUD', () => {
  it('GETs one application by handle', async () => {
    server.use(resource(RESOURCE, anApplication(), { record: requests }));

    await getApplication(APP_ID);

    expect(requests.last()?.url.pathname).toBe('/api/v0.9/applications/checkout-app');
  });

  it('POSTs a new application with the request body', async () => {
    server.use(accepts('post', '/applications', anApplication(), { record: requests }));

    await createApplication({ displayName: 'New App', projectId: 'retail', type: 'genai' });

    expect(requests.last()?.method).toBe('POST');
    expect(JSON.parse(requests.last()!.body)).toMatchObject({ displayName: 'New App' });
  });

  it('PUTs an update to the resource path', async () => {
    server.use(accepts('put', RESOURCE, anApplication(), { record: requests }));

    await updateApplication(APP_ID, anApplication({ displayName: 'Renamed' }));

    expect(requests.last()?.method).toBe('PUT');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/applications/checkout-app');
  });

  it('DELETEs and resolves to nothing on 204', async () => {
    server.use(noContent('delete', RESOURCE, { record: requests }));

    await expect(deleteApplication(APP_ID)).resolves.toBeUndefined();
    expect(requests.last()?.method).toBe('DELETE');
  });

  it('percent-encodes the handle', async () => {
    server.use(resource('/applications/:applicationId', anApplication(), { record: requests }));

    await getApplication('weird/handle');

    expect(requests.last()?.url.pathname).toBe('/api/v0.9/applications/weird%2Fhandle');
  });
});

describe('api-keys', () => {
  it('GETs the keys mapped to one application', async () => {
    // Unlike REST APIs, applications do expose a per-parent key listing.
    server.use(
      resource(`${RESOURCE}/api-keys`, listEnvelope([]), { record: requests })
    );

    await listApplicationApiKeys(APP_ID);

    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/applications/checkout-app/api-keys'
    );
  });

  it('POSTs a bulk key mapping', async () => {
    server.use(
      accepts('post', `${RESOURCE}/api-keys`, listEnvelope([]), {
        record: requests,
        status: 200,
      })
    );

    await addApplicationApiKeys(APP_ID, { apiKeys: [{ entityID: 'pizza-shack' }] } as never);

    expect(requests.last()?.method).toBe('POST');
    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/applications/checkout-app/api-keys'
    );
  });

  it('sends entityID as a query parameter when unmapping a key', async () => {
    // The key id alone does not identify the mapping; the spec requires
    // entityID on this DELETE, and omitting it is a 400 rather than a no-op.
    server.use(
      noContent('delete', `${RESOURCE}/api-keys/key-1`, { record: requests })
    );

    await removeApplicationApiKey(APP_ID, 'key-1', {
      query: { entityID: 'pizza-shack' },
    });

    expect(requests.last()?.method).toBe('DELETE');
    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/applications/checkout-app/api-keys/key-1'
    );
    expect(requests.last()?.params.get('entityID')).toBe('pizza-shack');
  });
});

describe('associations', () => {
  it('GETs the associations of one application', async () => {
    server.use(
      resource(`${RESOURCE}/associations`, listEnvelope([]), { record: requests })
    );

    await listApplicationAssociations(APP_ID);

    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/applications/checkout-app/associations'
    );
  });

  it('POSTs a bulk association add', async () => {
    server.use(
      accepts('post', `${RESOURCE}/associations`, listEnvelope([]), {
        record: requests,
        status: 200,
      })
    );

    await addApplicationAssociations(APP_ID, { associations: [] } as never);

    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/applications/checkout-app/associations'
    );
  });

  it('DELETEs one association beneath its application', async () => {
    server.use(
      noContent('delete', `${RESOURCE}/associations/provider-1`, { record: requests })
    );

    await removeApplicationAssociation(APP_ID, 'provider-1');

    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/applications/checkout-app/associations/provider-1'
    );
  });

  it('GETs keys nested two levels deep, under an association', async () => {
    // The deepest path in the layer; an off-by-one segment here reads a
    // different association's keys.
    server.use(
      resource(`${RESOURCE}/associations/provider-1/api-keys`, listEnvelope([]), {
        record: requests,
      })
    );

    await listAssociationApiKeys(APP_ID, 'provider-1');

    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/applications/checkout-app/associations/provider-1/api-keys'
    );
  });

  it('encodes every segment of the nested path', async () => {
    server.use(
      resource(
        '/applications/:applicationId/associations/:associationId/api-keys',
        listEnvelope([]),
        { record: requests }
      )
    );

    await listAssociationApiKeys('a/b', 'c d');

    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/applications/a%2Fb/associations/c%20d/api-keys'
    );
  });
});

describe('failures', () => {
  it('labels each sub-resource action distinctly for logs', async () => {
    server.use(failure('get', `${RESOURCE}/associations`, 403, 'FORBIDDEN'));

    const error = (await listApplicationAssociations(APP_ID).catch(
      (e: unknown) => e
    )) as ApiError;

    expect(error.operation).toBe('ListApplicationAssociations');
  });

  it('rejects rather than resolving when an unmap fails', async () => {
    server.use(failure('delete', `${RESOURCE}/api-keys/key-1`, 404, 'NOT_FOUND'));

    await expect(
      removeApplicationApiKey(APP_ID, 'key-1', { query: { entityID: 'x' } })
    ).rejects.toBeInstanceOf(ApiError);
  });
});
