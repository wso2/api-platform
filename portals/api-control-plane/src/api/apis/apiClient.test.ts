import { http, HttpResponse } from 'msw';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { server } from '../../test/server';

const BASE = 'http://platform.test';

// platformApiBaseUrl is read at module load → set env then re-import.
async function loadClient() {
  vi.stubEnv('VITE_PLATFORM_API_BASE_URL', BASE);
  vi.resetModules();
  return import('./apiClient');
}

const ORG = 'acme';

describe('apiClient (platform mode)', () => {
  beforeEach(() => vi.clearAllMocks());
  afterEach(() => vi.unstubAllEnvs());

  it('listApis lists REST APIs', async () => {
    const { listApis } = await loadClient();
    server.use(
      http.get(`${BASE}/api/v0.9/rest-apis`, () =>
        HttpResponse.json({
          list: [{ id: 'orders-api', name: 'Orders' }],
        })
      )
    );

    const apis = await listApis(ORG, 'proj-uuid');
    expect(apis).toHaveLength(1);
    expect(apis.find((a) => a.id === 'orders-api')?.kind).toBe('API_PROXY');
  });

  it('getApi returns the REST API when it exists', async () => {
    const { getApi } = await loadClient();
    server.use(
      http.get(`${BASE}/api/v0.9/rest-apis/orders-api`, () =>
        HttpResponse.json({ id: 'orders-api', name: 'Orders' })
      )
    );
    const api = await getApi(ORG, 'proj-uuid', 'orders-api');
    expect(api).toMatchObject({ id: 'orders-api', kind: 'API_PROXY' });
  });

  it('getApi returns undefined when the REST API 404s', async () => {
    const { getApi } = await loadClient();
    server.use(
      http.get(`${BASE}/api/v0.9/rest-apis/missing`, () =>
        HttpResponse.json({}, { status: 404 })
      )
    );
    await expect(getApi(ORG, 'proj-uuid', 'missing')).resolves.toBeUndefined();
  });

  it('createApi (scratch) POSTs the REST API body with id=handle, projectId', async () => {
    const { createApi } = await loadClient();
    let posted: Record<string, unknown> | undefined;
    server.use(
      http.get(`${BASE}/api/v0.9/projects/proj-uuid`, () =>
        HttpResponse.json({ id: 'proj-uuid', displayName: 'Proj' })
      ),
      http.post(`${BASE}/api/v0.9/rest-apis`, async ({ request }) => {
        posted = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json({ id: 'orders-api', displayName: 'Orders' });
      })
    );

    const created = await createApi(ORG, 'proj-uuid', {
      name: 'orders-api',
      displayName: 'Orders',
      kind: 'API_PROXY',
      version: '1.0.0',
      apiContext: '/orders',
      prodUrl: 'https://backend',
    });

    expect(created).toMatchObject({ id: 'orders-api', kind: 'API_PROXY' });
    expect(posted).toMatchObject({
      id: 'orders-api',
      displayName: 'Orders',
      context: '/orders',
      projectId: 'proj-uuid',
      upstream: { main: { url: 'https://backend' } },
    });
  });

  it('createApi (import) POSTs a structured body with the parsed operations', async () => {
    // platform-api has no server-side OpenAPI import; the definition is parsed in
    // the browser and its operations arrive on the input, sent via POST /rest-apis.
    const { createApi } = await loadClient();
    let posted: Record<string, unknown> | undefined;
    server.use(
      http.get(`${BASE}/api/v0.9/projects/proj-uuid`, () =>
        HttpResponse.json({ id: 'proj-uuid' })
      ),
      http.post(`${BASE}/api/v0.9/rest-apis`, async ({ request }) => {
        posted = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json({
          id: 'imported-api',
          displayName: 'Imported',
        });
      })
    );

    const created = await createApi(ORG, 'proj-uuid', {
      name: 'imported-api',
      displayName: 'Imported',
      kind: 'API_PROXY',
      version: '1.0.0',
      prodUrl: 'https://backend',
      source: { mode: 'import-url', url: 'https://example.com/openapi.yaml' },
      operations: [{ name: 'getOrders', method: 'GET', path: '/orders' }],
    });

    expect(created.id).toBe('imported-api');
    expect(posted).toMatchObject({
      id: 'imported-api',
      displayName: 'Imported',
      operations: [
        { name: 'getOrders', request: { method: 'GET', path: '/orders' } },
      ],
    });
  });

  it('updateApi PUTs a REST API and returns the refreshed detail', async () => {
    const { updateApi } = await loadClient();
    let putBody: Record<string, unknown> | undefined;
    server.use(
      http.put(`${BASE}/api/v0.9/rest-apis/orders-api`, async ({ request }) => {
        putBody = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json({
          id: 'orders-api',
          context: '/orders',
          upstream: { main: { url: 'https://updated' } },
        });
      })
    );

    const result = await updateApi(ORG, 'proj-uuid', {
      id: 'orders-api',
      projectId: 'proj-uuid',
      name: 'orders-api',
      displayName: 'Orders',
      handler: 'orders-api',
      kind: 'API_PROXY',
      status: 'ACTIVE',
      context: '/orders',
      transport: ['https'],
      operations: [],
      policies: [],
      endpoints: { prodUrl: 'https://updated' },
      raw: { id: 'orders-api' },
    });

    expect(result.endpoints.prodUrl).toBe('https://updated');
    expect(putBody?.upstream).toEqual({ main: { url: 'https://updated' } });
  });

  it('deleteApi targets the rest-apis path', async () => {
    const { deleteApi } = await loadClient();
    const hits: string[] = [];
    server.use(
      http.delete(`${BASE}/api/v0.9/rest-apis/orders-api`, () => {
        hits.push('rest');
        return new HttpResponse(null, { status: 204 });
      })
    );

    await deleteApi(ORG, 'proj-uuid', {
      id: 'orders-api',
      projectId: 'p',
      name: 'orders-api',
      displayName: 'Orders',
      handler: 'orders-api',
      kind: 'API_PROXY',
      status: 'ACTIVE',
    });

    expect(hits).toEqual(['rest']);
  });
});
