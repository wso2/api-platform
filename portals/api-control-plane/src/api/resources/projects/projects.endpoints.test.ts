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
  aProject,
  accepts,
  collection,
  failure,
  manyProjects,
  noContent,
  recorder,
  resource,
  type Recorder,
} from '../../../test/msw';
import { server } from '../../../test/server';
import { ApiError } from '../../core/errors';
import { resetHttpClient } from '../../core/http';
import {
  createProject,
  deleteProject,
  getProject,
  listProjects,
  updateProject,
} from './projects.endpoints';

/**
 * Contract tests for `/projects`.
 *
 * The property worth pinning here is that the **organization never appears in
 * the URL or the body** — the collection is scoped entirely by the `X-Org-Id`
 * header the transport attaches. A caller that assumed otherwise would build a
 * path that does not exist, and one that forgot the header would read another
 * tenant's projects.
 */

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('listProjects', () => {
  it('GETs the collection', async () => {
    server.use(collection('/projects', [aProject()], { record: requests }));

    await listProjects();

    expect(requests.last()?.method).toBe('GET');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/projects');
  });

  it('scopes by header rather than by any URL segment or parameter', async () => {
    server.use(collection('/projects', [], { record: requests }));

    await listProjects({ orgId: 'acme-org' });

    expect(requests.last()?.headers.get('X-Org-Id')).toBe('acme-org');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/projects');
    expect(requests.last()?.params.get('organizationId')).toBeNull();
  });

  it('passes paging, sorting and search parameters through', async () => {
    server.use(collection('/projects', [], { record: requests }));

    await listProjects({
      query: { limit: 10, offset: 20, sortBy: 'name', sortOrder: 'asc', query: 'retail' },
    });

    expect(Object.fromEntries(requests.last()!.params)).toEqual({
      limit: '10',
      offset: '20',
      query: 'retail',
      sortBy: 'name',
      sortOrder: 'asc',
    });
  });

  it('pages the collection rather than returning everything', async () => {
    server.use(collection('/projects', manyProjects(30), { record: requests }));

    const response = await listProjects({ query: { limit: 10, offset: 10 } });

    expect(response.list).toHaveLength(10);
    expect(response.list[0].displayName).toBe('Project 11');
    expect(response.pagination).toMatchObject({ total: 30 });
  });
});

describe('getProject', () => {
  it('GETs one project by handle', async () => {
    server.use(resource('/projects/retail', aProject(), { record: requests }));

    await getProject('retail');

    expect(requests.last()?.url.pathname).toBe('/api/v0.9/projects/retail');
  });

  it('percent-encodes the handle', async () => {
    server.use(resource('/projects/:projectId', aProject(), { record: requests }));

    await getProject('weird/handle');

    expect(requests.last()?.url.pathname).toBe('/api/v0.9/projects/weird%2Fhandle');
  });
});

describe('createProject', () => {
  it('POSTs to the collection with the request body', async () => {
    server.use(accepts('post', '/projects', aProject(), { record: requests }));

    await createProject({ displayName: 'Wholesale' });

    expect(requests.last()?.method).toBe('POST');
    expect(JSON.parse(requests.last()!.body)).toEqual({ displayName: 'Wholesale' });
  });

  it('returns the created project', async () => {
    server.use(accepts('post', '/projects', aProject({ id: 'wholesale' })));

    await expect(createProject({ displayName: 'Wholesale' })).resolves.toMatchObject(
      { id: 'wholesale' }
    );
  });
});

describe('updateProject', () => {
  it('PUTs to the resource path with the request body', async () => {
    server.use(accepts('put', '/projects/retail', aProject(), { record: requests }));

    await updateProject('retail', aProject({ displayName: 'Retail (renamed)' }));

    expect(requests.last()?.method).toBe('PUT');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/projects/retail');
    expect(JSON.parse(requests.last()!.body)).toMatchObject({
      displayName: 'Retail (renamed)',
    });
  });
});

describe('deleteProject', () => {
  it('DELETEs the resource path', async () => {
    server.use(noContent('delete', '/projects/retail', { record: requests }));

    await deleteProject('retail');

    expect(requests.last()?.method).toBe('DELETE');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/projects/retail');
  });

  it('resolves to nothing on 204', async () => {
    server.use(noContent('delete', '/projects/retail'));

    await expect(deleteProject('retail')).resolves.toBeUndefined();
  });
});

describe('failures', () => {
  it('surfaces the guard that blocked a delete, with its code intact', async () => {
    // The backend refuses to delete the last project, or one that still owns
    // APIs. The UI can only explain which by reading `code`.
    server.use(
      failure('delete', '/projects/retail', 400, 'PROJECT_HAS_RESOURCES', {
        message: 'The project still contains APIs.',
      })
    );

    const error = (await deleteProject('retail').catch((e: unknown) => e)) as ApiError;

    expect(error.code).toBe('PROJECT_HAS_RESOURCES');
    expect(error.status).toBe(400);
  });

  it('carries field errors from a validation failure so a form can bind them', async () => {
    server.use(
      failure('post', '/projects', 400, 'VALIDATION_FAILED', {
        errors: [{ field: 'displayName', message: 'must not be blank' }],
      })
    );

    const error = (await createProject({ displayName: '' }).catch(
      (e: unknown) => e
    )) as ApiError;

    expect(error.fieldErrorMap()).toEqual({ displayName: 'must not be blank' });
  });
});
