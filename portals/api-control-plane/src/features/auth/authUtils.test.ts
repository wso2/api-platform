import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  AUTH_RETURN_TO_STORAGE_KEY,
  AUTH_SESSION_STORAGE_KEY,
} from './authConstants';
import {
  buildEncodedState,
  clearStoredAuth,
  generateTokenBindingId,
  getConfiguredLoginProviders,
  makeLocalSession,
  normalizeLocalFileSession,
  normalizeUser,
  persistSession,
  readRedirectParams,
  readStoredSession,
  withTimeout,
} from './authUtils';

describe('buildEncodedState', () => {
  it('base64-encodes the fidp + returnTo into the expected shape', () => {
    const decoded = JSON.parse(atob(buildEncodedState('google', '/home')));
    expect(decoded).toEqual({
      fidpId: 'google',
      returnToUrl: '/home',
      returnToOrg: '',
    });
  });

  it('defaults returnTo to /', () => {
    expect(JSON.parse(atob(buildEncodedState('email'))).returnToUrl).toBe('/');
  });
});

describe('normalizeUser', () => {
  it('prefers email then username then sub', () => {
    expect(normalizeUser({ email: 'a@b.com' }).email).toBe('a@b.com');
    expect(normalizeUser({ username: 'user1' }).email).toBe('user1');
    expect(normalizeUser({ sub: 'abc' }).email).toBe('abc');
  });

  it('prefers displayName then name then username for the name', () => {
    expect(normalizeUser({ email: 'a@b.com', displayName: 'Ann' }).name).toBe(
      'Ann'
    );
    expect(normalizeUser({ email: 'a@b.com', name: 'Bob' }).name).toBe('Bob');
  });

  it('falls back to a default user with no info', () => {
    expect(normalizeUser()).toEqual({
      email: 'user@wso2.com',
      name: 'user@wso2.com',
    });
  });
});

describe('local session helpers', () => {
  it('makeLocalSession applies defaults', () => {
    const session = makeLocalSession('tok');
    expect(session.token).toBe('tok');
    expect(session.user).toEqual({
      name: 'Oxygen Developer',
      email: 'developer@wso2.com',
    });
    expect(session.expiresAt).toBeGreaterThan(Date.now());
  });

  it('normalizeLocalFileSession honors token/accessToken + name precedence', () => {
    expect(
      normalizeLocalFileSession({ accessToken: 'at', email: 'x@y.com' })
    ).toMatchObject({ token: 'at', user: { email: 'x@y.com', name: 'x@y.com' } });
    expect(normalizeLocalFileSession({}).token).toBe('local-development-token');
  });
});

describe('stored session', () => {
  beforeEach(() => {
    // Node's experimental Web Storage lacks .clear(); remove the keys we use.
    localStorage.removeItem(AUTH_SESSION_STORAGE_KEY);
    sessionStorage.removeItem(AUTH_RETURN_TO_STORAGE_KEY);
  });

  it('returns unauthenticated when nothing is stored', () => {
    expect(readStoredSession()).toEqual({
      session: null,
      status: 'unauthenticated',
    });
  });

  it('round-trips a persisted, live session as authenticated', () => {
    const session = makeLocalSession('tok');
    persistSession(session);
    expect(readStoredSession()).toEqual({ session, status: 'authenticated' });
  });

  it('treats an expired session as expired and clears it', () => {
    persistSession(makeLocalSession('tok', undefined, Date.now() - 1000));
    expect(readStoredSession().status).toBe('expired');
    expect(localStorage.getItem(AUTH_SESSION_STORAGE_KEY)).toBeNull();
  });

  it('treats malformed JSON as unauthenticated and clears it', () => {
    localStorage.setItem(AUTH_SESSION_STORAGE_KEY, '{not json');
    expect(readStoredSession().status).toBe('unauthenticated');
    expect(localStorage.getItem(AUTH_SESSION_STORAGE_KEY)).toBeNull();
  });

  it('clearStoredAuth removes the stored session', () => {
    persistSession(makeLocalSession('tok'));
    clearStoredAuth();
    expect(localStorage.getItem(AUTH_SESSION_STORAGE_KEY)).toBeNull();
  });
});

describe('readRedirectParams', () => {
  afterEach(() => window.history.pushState({}, '', '/'));

  it('reads code + state from the query string', () => {
    window.history.pushState({}, '', '/login/callback?code=abc&state=xyz');
    expect(readRedirectParams()).toMatchObject({ code: 'abc', state: 'xyz' });
  });

  it('reads token + error from the hash fragment', () => {
    window.history.pushState({}, '', '/login#access_token=tok&error=denied');
    const params = readRedirectParams();
    expect(params.token).toBe('tok');
    expect(params.error).toBe('denied');
  });
});

describe('withTimeout', () => {
  it('resolves with the value when the promise wins', async () => {
    await expect(withTimeout(Promise.resolve('ok'), 'slow', 1000)).resolves.toBe(
      'ok'
    );
  });

  it('rejects with the message when the timeout wins', async () => {
    await expect(
      withTimeout(new Promise<never>(() => {}), 'too slow', 20)
    ).rejects.toThrow('too slow');
  });
});

describe('generateTokenBindingId', () => {
  it('returns a non-empty string', () => {
    expect(typeof generateTokenBindingId()).toBe('string');
    expect(generateTokenBindingId().length).toBeGreaterThan(0);
  });
});

describe('getConfiguredLoginProviders', () => {
  afterEach(() => vi.unstubAllEnvs());

  it('returns no providers with the default (asgardeo) test config', () => {
    expect(getConfiguredLoginProviders()).toEqual([]);
  });

  it('includes email + google when configured', async () => {
    vi.stubEnv('VITE_ENABLE_LOCAL_AUTH_FALLBACK', 'true');
    vi.stubEnv('VITE_FIDP_GOOGLE', 'google-idp');
    vi.resetModules();
    const { getConfiguredLoginProviders: fresh } = await import('./authUtils');
    const providers = fresh();
    expect(providers).toContainEqual({ id: 'LOCAL', label: 'Email' });
    expect(providers).toContainEqual({ id: 'google-idp', label: 'Google' });
  });
});
