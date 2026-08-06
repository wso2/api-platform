import { http, HttpResponse } from 'msw';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { CSRF_HEADER, CSRF_HEADER_VALUE } from '../../features/auth/authConstants';
import { server } from '../../test/server';

const BASE = 'http://platform.test';
// Clients build full URLs as `${PLATFORM_API_BASE}/...`; sendOnce fetches them
// verbatim (no base prepend), so PATH here is the full request URL.
const PATH = `${BASE}/api/v0.9/test`;

// platformClient reads runtimeConfig (platformApiBaseUrl) at module load, so
// the env must be set before importing it. Re-import fresh per test.
async function loadClient() {
  vi.stubEnv('VITE_PLATFORM_API_BASE_URL', BASE);
  vi.resetModules();
  return import('./platformClient');
}

describe('platformClient.platformRequest', () => {
  beforeEach(() => vi.clearAllMocks());
  afterEach(() => vi.unstubAllEnvs());

  // The BFF injects the bearer token server-side (from the session cookie)
  // before ever forwarding the request — this client never holds or sends
  // one, and never retries on 401 (the BFF already renews a near-expiry OIDC
  // session before proxying, so a 401 reaching here means the session is
  // genuinely gone).
  it('never sends an Authorization header', async () => {
    const { platformGet } = await loadClient();
    let authHeader: string | null = 'unset';
    server.use(
      http.get(PATH, ({ request }) => {
        authHeader = request.headers.get('Authorization');
        return HttpResponse.json({ ok: true });
      })
    );
    await platformGet(PATH, 'org-1');
    expect(authHeader).toBeNull();
  });

  it('sends the CSRF header on a mutating request but not on GET', async () => {
    const { platformGet, platformPost } = await loadClient();
    let getCsrfHeader: string | null = 'unset';
    let postCsrfHeader: string | null = 'unset';
    server.use(
      http.get(PATH, ({ request }) => {
        getCsrfHeader = request.headers.get(CSRF_HEADER);
        return HttpResponse.json({ ok: true });
      }),
      http.post(PATH, ({ request }) => {
        postCsrfHeader = request.headers.get(CSRF_HEADER);
        return HttpResponse.json({ ok: true });
      })
    );
    await platformGet(PATH, 'org-1');
    await platformPost(PATH, 'org-1', { name: 'x' });
    expect(getCsrfHeader).toBeNull();
    expect(postCsrfHeader).toBe(CSRF_HEADER_VALUE);
  });

  it('surfaces a 401 as an UNAUTHORIZED ApiError, with no retry', async () => {
    const { platformGet } = await loadClient();
    let calls = 0;
    server.use(
      http.get(PATH, () => {
        calls += 1;
        return HttpResponse.json({ message: 'session expired' }, { status: 401 });
      })
    );

    await expect(platformGet(PATH, 'org-1')).rejects.toMatchObject({
      name: 'ApiError',
      code: 'UNAUTHORIZED',
      status: 401,
      message: 'session expired',
    });
    expect(calls).toBe(1);
  });

  it('returns undefined for a 204 response', async () => {
    const { platformDelete } = await loadClient();
    server.use(
      http.delete(PATH, () => new HttpResponse(null, { status: 204 }))
    );
    await expect(platformDelete(PATH, 'org-1')).resolves.toBeUndefined();
  });

  it('falls back to a generic message on a non-JSON error body', async () => {
    const { platformGet } = await loadClient();
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
    const { platformGet } = await loadClient();
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
