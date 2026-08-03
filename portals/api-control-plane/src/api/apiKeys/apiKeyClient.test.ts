import { http, HttpResponse } from 'msw';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { server } from '../../test/server';
import type { Api } from '../../types/domain';

vi.mock('@asgardeo/auth-react', () => {
  const instance = {
    getAccessToken: vi.fn().mockResolvedValue('tok'),
    refreshAccessToken: vi.fn(),
  };
  return { AsgardeoSPAClient: { getInstance: () => instance } };
});

const BASE = 'http://platform.test';

async function loadClient() {
  vi.stubEnv('VITE_PLATFORM_API_BASE_URL', BASE);
  vi.resetModules();
  return import('./apiKeyClient');
}

const ORG = 'acme';

const api: Api = {
  id: 'api-1',
  projectId: 'proj-1',
  name: 'orders-api',
  displayName: 'Orders API',
  handler: 'orders-api',
  kind: 'API_PROXY',
  status: 'ACTIVE',
};

describe('apiKeyClient (platform mode)', () => {
  beforeEach(() => vi.clearAllMocks());
  afterEach(() => vi.unstubAllEnvs());

  it('listApiKeys filters the user inventory down to this API', async () => {
    const { listApiKeys } = await loadClient();
    server.use(
      http.get(`${BASE}/api/v0.9/me/api-keys`, () =>
        HttpResponse.json({
          count: 2,
          items: [
            {
              name: 'prod-key',
              maskedApiKey: 'abcd****xy',
              artifactId: 'orders-api',
              status: 'ACTIVE',
              expiresAt: '2026-10-01T00:00:00Z',
            },
            {
              name: 'other-key',
              maskedApiKey: 'zzzz****zz',
              artifactId: 'another-api',
            },
          ],
        })
      )
    );
    const keys = await listApiKeys(ORG, api);
    expect(keys).toEqual([
      {
        name: 'prod-key',
        maskedApiKey: 'abcd****xy',
        status: 'ACTIVE',
        createdAt: undefined,
        expiresAt: '2026-10-01T00:00:00Z',
      },
    ]);
  });

  it('createApiKey posts displayName + apiKey to the API key path', async () => {
    const { createApiKey } = await loadClient();
    let body: Record<string, unknown> | undefined;
    server.use(
      http.post(
        `${BASE}/api/v0.9/rest-apis/orders-api/api-keys`,
        async ({ request }) => {
          body = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(
            { status: 'success', message: 'created' },
            { status: 201 }
          );
        }
      )
    );
    await createApiKey(ORG, api, {
      displayName: 'Production Key',
      apiKey: 'super-secret',
    });
    expect(body).toEqual({
      displayName: 'Production Key',
      apiKey: 'super-secret',
    });
  });

  it('revokeApiKey deletes the named key', async () => {
    const { revokeApiKey } = await loadClient();
    let called = false;
    server.use(
      http.delete(
        `${BASE}/api/v0.9/rest-apis/orders-api/api-keys/prod-key`,
        () => {
          called = true;
          return new HttpResponse(null, { status: 204 });
        }
      )
    );
    await revokeApiKey(ORG, api, 'prod-key');
    expect(called).toBe(true);
  });
});
