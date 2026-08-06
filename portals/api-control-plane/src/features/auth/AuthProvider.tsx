import { ReactNode, useCallback, useContext, useEffect, useMemo, useState } from 'react';

import { runtimeConfig } from '../../config/runtime';
import { CSRF_HEADER, CSRF_HEADER_VALUE } from './authConstants';
import { AuthStateContext } from './AuthStateContext';
import type { AuthState, AuthStatus, AuthUser } from './authTypes';

const SESSION_URL = '/api/session';
const LOGIN_URL = '/api/login';
const LOGOUT_URL = '/api/logout';
const OIDC_LOGIN_URL = '/api/auth/login';

type SessionResponse = {
  authenticated: boolean;
  user?: AuthUser;
};

type ErrorResponse = {
  message?: string;
};

const mutatingHeaders: HeadersInit = {
  'Content-Type': 'application/json',
  [CSRF_HEADER]: CSRF_HEADER_VALUE,
};

/**
 * The BFF owns authentication: it holds the session server-side in an
 * HttpOnly cookie, and the browser never sees a token. This provider only
 * ever talks to the BFF's own same-origin auth endpoints — never an IdP
 * SDK — so there is exactly one implementation regardless of auth mode.
 *
 *   - basic mode: loginWithCredentials() posts to /api/login directly.
 *   - oidc mode: login() redirects to /api/auth/login, which runs the whole
 *     code exchange server-side and redirects back with the session cookie
 *     already set — this provider only needs to (re-)hydrate from
 *     /api/session afterwards, same as on first load.
 */
export function AuthProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<AuthStatus>('loading');
  const [user, setUser] = useState<AuthUser>();
  const [error, setError] = useState<string>();

  const hydrate = useCallback(async () => {
    try {
      const response = await fetch(SESSION_URL, { credentials: 'same-origin' });
      if (!response.ok) {
        setStatus('unauthenticated');
        setUser(undefined);
        return;
      }
      const body = (await response.json()) as SessionResponse;
      setUser(body.user);
      setStatus(body.authenticated ? 'authenticated' : 'unauthenticated');
    } catch {
      setStatus('unauthenticated');
      setUser(undefined);
    }
  }, []);

  useEffect(() => {
    void hydrate();
  }, [hydrate]);

  const login = useCallback((returnTo = window.location.pathname) => {
    window.location.assign(`${OIDC_LOGIN_URL}?return=${encodeURIComponent(returnTo)}`);
  }, []);

  const loginWithCredentials = useCallback(
    async (username: string, password: string) => {
      setError(undefined);
      try {
        const response = await fetch(LOGIN_URL, {
          method: 'POST',
          credentials: 'same-origin',
          headers: mutatingHeaders,
          body: JSON.stringify({ username, password }),
        });
        if (!response.ok) {
          const body = (await response.json().catch(() => ({}))) as ErrorResponse;
          setError(body.message || 'Invalid credentials');
          return false;
        }
        const body = (await response.json()) as { user?: AuthUser };
        setUser(body.user);
        setStatus('authenticated');
        return true;
      } catch {
        setError('Unable to sign in. Please try again.');
        return false;
      }
    },
    []
  );

  const logout = useCallback(() => {
    void (async () => {
      let logoutUrl: string | undefined;
      try {
        const response = await fetch(LOGOUT_URL, {
          method: 'POST',
          credentials: 'same-origin',
          headers: mutatingHeaders,
        });
        if (response.status === 200) {
          const body = (await response.json()) as { logoutUrl?: string };
          logoutUrl = body.logoutUrl;
        }
      } catch {
        // Best-effort: proceed to clear local state and navigate away regardless.
      }
      setUser(undefined);
      setStatus('unauthenticated');
      window.location.assign(logoutUrl || '/login');
    })();
  }, []);

  // Platform API calls are always routed through the same-origin proxy,
  // which forwards the session's bearer token itself — there is no
  // console-side org-token exchange to perform.
  const exchangeOrgToken = useCallback(async () => true, []);

  const value = useMemo<AuthState>(
    () => ({
      mode: runtimeConfig.authMode,
      error,
      status,
      isLoading: status === 'loading',
      isAuthenticated: status === 'authenticated',
      user,
      login,
      loginWithCredentials,
      exchangeOrgToken,
      logout,
    }),
    [error, exchangeOrgToken, login, loginWithCredentials, logout, status, user]
  );

  return (
    <AuthStateContext.Provider value={value}>
      {children}
    </AuthStateContext.Provider>
  );
}

export const useAuth = () => {
  const context = useContext(AuthStateContext);
  if (!context) throw new Error('useAuth must be used within AuthProvider');
  return context;
};

export type { AuthState, AuthStatus, AuthUser } from './authTypes';
