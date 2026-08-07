import { ReactNode, useCallback, useMemo, useState } from 'react';

import { setApiAccessToken, setApiHttpRequest } from '../../../api/client';
import { runtimeConfig } from '../../../config/runtime';
import { AUTH_RETURN_TO_STORAGE_KEY } from '../authConstants';
import type {
  AuthState,
  AuthStatus,
  LocalFileAuthSession,
  StoredAuthSession,
} from '../authTypes';
import {
  clearStoredAuth,
  getConfiguredLoginProviders,
  makeLocalSession,
  normalizeLocalFileSession,
  persistSession,
  readRedirectParams,
  readStoredSession,
} from '../authUtils';
import { AuthStateContext } from '../AuthStateContext';

const loadSessionFile = async (): Promise<StoredAuthSession> => {
  if (!runtimeConfig.localAuthFileUrl) {
    return makeLocalSession('local-development-token');
  }

  const response = await fetch(runtimeConfig.localAuthFileUrl, {
    headers: { Accept: 'application/json' },
  });
  if (!response.ok) {
    throw new Error(`Failed to load local auth file: ${response.status}`);
  }
  return normalizeLocalFileSession((await response.json()) as LocalFileAuthSession);
};

export function LocalFileAuthAdapter({ children }: { children: ReactNode }) {
  const [initialAuth] = useState(() => {
    if (
      runtimeConfig.authMode === 'local-file' ||
      runtimeConfig.enableLocalAuthFallback
    ) {
      return readStoredSession();
    }

    clearStoredAuth();
    setApiAccessToken(undefined);
    setApiHttpRequest(undefined);
    return { session: null, status: 'unauthenticated' as AuthStatus };
  });
  const [session, setSession] = useState<StoredAuthSession | null>(() => {
    setApiAccessToken(initialAuth.session?.token);
    setApiHttpRequest(undefined);
    return initialAuth.session;
  });
  const [status, setStatus] = useState<AuthStatus>(initialAuth.status);
  const [loginError, setLoginError] = useState<string>();
  const loginProviders = useMemo(getConfiguredLoginProviders, []);

  const activateSession = useCallback((nextSession: StoredAuthSession) => {
    persistSession(nextSession);
    setApiAccessToken(nextSession.token);
    setApiHttpRequest(undefined);
    setSession(nextSession);
    setStatus('authenticated');
  }, []);

  const loginWithProvider = useCallback(
    (_fidp: string, returnTo = '/') => {
      sessionStorage.setItem(AUTH_RETURN_TO_STORAGE_KEY, returnTo);
      setLoginError(undefined);
      if (
        runtimeConfig.authMode !== 'local-file' &&
        !runtimeConfig.enableLocalAuthFallback
      ) {
        clearStoredAuth();
        setApiAccessToken(undefined);
        setApiHttpRequest(undefined);
        setSession(null);
        setStatus('unauthenticated');
        return;
      }

      setStatus('loading');
      loadSessionFile()
        .then(activateSession)
        .catch((error) => {
          // eslint-disable-next-line no-console
          console.error('Failed to initialize local file auth', error);
          clearStoredAuth();
          setApiAccessToken(undefined);
          setSession(null);
          setLoginError('Unable to load local auth session file.');
          setStatus('unauthenticated');
        });
    },
    [activateSession]
  );

  const login = useCallback(
    (returnTo = '/') => loginWithProvider(runtimeConfig.fidpEmail, returnTo),
    [loginWithProvider]
  );

  const completeLoginFromRedirect = useCallback(() => {
    if (
      runtimeConfig.authMode !== 'local-file' &&
      !runtimeConfig.enableLocalAuthFallback
    ) {
      clearStoredAuth();
      setApiAccessToken(undefined);
      setApiHttpRequest(undefined);
      setSession(null);
      setStatus('unauthenticated');
      return '/login';
    }

    const params = readRedirectParams();
    const returnTo =
      params.state || sessionStorage.getItem(AUTH_RETURN_TO_STORAGE_KEY) || '/';
    sessionStorage.removeItem(AUTH_RETURN_TO_STORAGE_KEY);

    if (params.error === 'access_denied') {
      clearStoredAuth();
      setApiAccessToken(undefined);
      setApiHttpRequest(undefined);
      setSession(null);
      setStatus('forbidden');
      return '/unauthorized';
    }

    const token = params.token || (params.code ? `auth-code:${params.code}` : '');
    if (!token) {
      setStatus(session ? 'authenticated' : 'unauthenticated');
      return session ? returnTo : '/login';
    }

    activateSession(makeLocalSession(token));
    return returnTo;
  }, [activateSession, session]);

  // Local-file mode does not use JWT/org scoping, so there is no token to
  // exchange; the stored token is treated as already org-scoped.
  const exchangeOrgToken = useCallback(async () => true, []);

  // No interactive IdP in local-file mode: a session is "restored" iff one was
  // already loaded from storage / the local auth file.
  const signInSilently = useCallback(async () => !!session, [session]);

  const logout = useCallback(() => {
    clearStoredAuth();
    setApiAccessToken(undefined);
    setApiHttpRequest(undefined);
    setSession(null);
    setStatus('unauthenticated');
  }, []);

  const value = useMemo<AuthState>(
    () => ({
      mode: 'local-file',
      error: loginError,
      status,
      isLoading: status === 'loading',
      isAuthenticated: !!session,
      user: session?.user,
      token: session?.token,
      loginProviders,
      login,
      loginWithProvider,
      exchangeOrgToken,
      signInSilently,
      completeLoginFromRedirect,
      logout,
    }),
    [
      completeLoginFromRedirect,
      exchangeOrgToken,
      login,
      loginError,
      loginProviders,
      loginWithProvider,
      logout,
      session,
      signInSilently,
      status,
    ]
  );

  return (
    <AuthStateContext.Provider value={value}>
      {children}
    </AuthStateContext.Provider>
  );
}
