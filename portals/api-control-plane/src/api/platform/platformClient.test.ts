import { http, HttpResponse } from 'msw';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { server } from '../../test/server';

const BASE = 'http://platform.test';
// Clients build full URLs as `${PLATFORM_API_BASE}/...`; sendOnce fetches them
// verbatim (no base prepend), so PATH here is the full request URL.
const PATH = `${BASE}/api/v0.9/test`;

// platformClient reads runtimeConfig (platformApiBaseUrl) at module load, so the
// env must be set before importing it. Re-import fresh per test, and register
// the token provider/refresher seam (what an auth adapter would do at runtime).
async function loadClient() {
  vi.stubEnv('VITE_PLATFORM_API_BASE_URL', BASE);
  vi.resetModules();
  const clientMod = await import('../client');
  const getAccessToken = vi.fn<() => Promise<string | undefined>>();
  const refreshAccessToken = vi.fn<() => Promise<string | undefined>>();
  clientMod.setPlatformTokenProvider(getAccessToken);
  clientMod.setPlatformTokenRefresher(refreshAccessToken);
  const mod = await import('./platformClient');
  return { client: { getAccessToken, refreshAccessToken }, ...mod };
}

describe('platformClient.platformRequest', () => {
  beforeEach(() => vi.clearAllMocks());
  afterEach(() => vi.unstubAllEnvs());

  it('refreshes the token once and retries on a 401, then succeeds', async () => {
    const { client, platformGet } = await loadClient();
    client.getAccessToken.mockResolvedValueOnce('stale-token');
    // The refresher returns the fresh token used for the retry.
    client.refreshAccessToken.mockResolvedValue('fresh-token');

    const seenAuth: (string | null)[] = [];
    let calls = 0;
    server.use(
      http.get(PATH, ({ request }) => {
        calls += 1;
        seenAuth.push(request.headers.get('Authorization'));
        if (calls === 1) {
          return HttpResponse.json(
            { message: 'token expired' },
            { status: 401 }
          );
        }
        return HttpResponse.json({ ok: true });
      })
    );

    await expect(platformGet(PATH, 'org-1')).resolves.toEqual({ ok: true });
    expect(client.refreshAccessToken).toHaveBeenCalledTimes(1);
    expect(calls).toBe(2);
    expect(seenAuth).toEqual(['Bearer stale-token', 'Bearer fresh-token']);
  });

  it('surfaces the original 401 as an UNAUTHORIZED ApiError when refresh fails', async () => {
    const { client, platformGet } = await loadClient();
    client.getAccessToken.mockResolvedValue('stale-token');
    client.refreshAccessToken.mockRejectedValue(new Error('no refresh'));

    server.use(
      http.get(PATH, () =>
        HttpResponse.json({ message: 'token expired' }, { status: 401 })
      )
    );

    await expect(platformGet(PATH, 'org-1')).rejects.toMatchObject({
      name: 'ApiError',
      code: 'UNAUTHORIZED',
      status: 401,
      message: 'token expired',
    });
  });

  it('returns undefined for a 204 response', async () => {
    const { client, platformDelete } = await loadClient();
    client.getAccessToken.mockResolvedValue('tok');
    server.use(
      http.delete(PATH, () => new HttpResponse(null, { status: 204 }))
    );
    await expect(platformDelete(PATH, 'org-1')).resolves.toBeUndefined();
  });

  it('falls back to a generic message on a non-JSON error body', async () => {
    const { client, platformGet } = await loadClient();
    client.getAccessToken.mockResolvedValue('tok');
    server.use(
      http.get(
        PATH,
        () => new HttpResponse('upstream exploded', { status: 500 })
      )
    );
    await expect(platformGet(PATH, 'org-1')).rejects.toMatchObject({
      code: 'SERVER_ERROR',
      status: 500,
      message: 'platform-api request failed (500)',
    });
  });

  it('sends the X-Org-Id header', async () => {
    const { client, platformGet } = await loadClient();
    client.getAccessToken.mockResolvedValue('tok');
    let orgHeader: string | null = null;
    server.use(
      http.get(PATH, ({ request }) => {
        orgHeader = request.headers.get('X-Org-Id');
        return HttpResponse.json({ ok: true });
      })
    );
    await platformGet(PATH, 'org-42');
    expect(orgHeader).toBe('org-42');
  });
});
