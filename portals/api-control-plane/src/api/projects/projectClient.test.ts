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

import { http, HttpResponse } from 'msw';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { server } from '../../test/server';

const BASE = 'http://platform.test';

// platformApiBaseUrl is read at module load → set env then re-import.
async function loadClient() {
  vi.stubEnv('VITE_PLATFORM_API_BASE_URL', BASE);
  vi.resetModules();
  return import('./projectClient');
}

const ORG = 'acme';

describe('projectClient.createProject (platform mode)', () => {
  beforeEach(() => vi.clearAllMocks());
  afterEach(() => vi.unstubAllEnvs());

  it('POSTs name + description and adapts the created project', async () => {
    const { createProject } = await loadClient();
    let captured: { body: unknown; orgId: string | null } | undefined;
    server.use(
      http.post(`${BASE}/api/v0.9/projects`, async ({ request }) => {
        captured = {
          body: await request.json(),
          orgId: request.headers.get('X-Org-Id'),
        };
        return HttpResponse.json(
          {
            id: 'proj-uuid',
            name: 'Billing',
            organizationId: 'org-uuid',
            description: 'Invoices',
          },
          { status: 201 }
        );
      })
    );

    const project = await createProject(ORG, {
      name: 'Billing',
      description: 'Invoices',
    });

    expect(captured?.orgId).toBe(ORG);
    expect(captured?.body).toEqual({
      displayName: 'Billing',
      description: 'Invoices',
    });
    expect(project).toMatchObject({
      id: 'proj-uuid',
      name: 'Billing',
      orgId: 'org-uuid',
      handler: 'proj-uuid',
    });
  });

  it('omits an empty description from the request body', async () => {
    const { createProject } = await loadClient();
    let body: unknown;
    server.use(
      http.post(`${BASE}/api/v0.9/projects`, async ({ request }) => {
        body = await request.json();
        return HttpResponse.json({ id: 'p', name: 'Solo' }, { status: 201 });
      })
    );

    await createProject(ORG, { name: '  Solo  ', description: '   ' });
    expect(body).toEqual({ displayName: 'Solo' });
  });

  it('surfaces a 409 as a CONFLICT error', async () => {
    const { createProject } = await loadClient();
    server.use(
      http.post(`${BASE}/api/v0.9/projects`, () =>
        HttpResponse.json(
          { message: 'Project already exists in organization' },
          { status: 409 }
        )
      )
    );

    await expect(
      createProject(ORG, { name: 'Retail APIs' })
    ).rejects.toMatchObject({
      code: 'CONFLICT',
      message: 'Project already exists in organization',
    });
  });
});

const PROJECT = {
  id: 'proj-uuid',
  orgId: 'org-uuid',
  name: 'Billing',
  handler: 'proj-uuid',
};

describe('projectClient.deleteProject (platform mode)', () => {
  beforeEach(() => vi.clearAllMocks());
  afterEach(() => vi.unstubAllEnvs());

  it('DELETEs the project by id and resolves on 204', async () => {
    const { deleteProject } = await loadClient();
    let captured: { url: string; orgId: string | null } | undefined;
    server.use(
      http.delete(`${BASE}/api/v0.9/projects/:id`, ({ request, params }) => {
        captured = {
          url: String(params.id),
          orgId: request.headers.get('X-Org-Id'),
        };
        return new HttpResponse(null, { status: 204 });
      })
    );

    await expect(deleteProject(ORG, PROJECT)).resolves.toBeUndefined();
    expect(captured).toEqual({ url: 'proj-uuid', orgId: ORG });
  });

  it('surfaces a backend guard (400) with its message', async () => {
    const { deleteProject } = await loadClient();
    server.use(
      http.delete(`${BASE}/api/v0.9/projects/:id`, () =>
        HttpResponse.json(
          { message: 'Project has associated APIs' },
          { status: 400 }
        )
      )
    );

    await expect(deleteProject(ORG, PROJECT)).rejects.toMatchObject({
      message: 'Project has associated APIs',
    });
  });
});
