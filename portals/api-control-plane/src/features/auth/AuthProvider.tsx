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

import { ReactNode, useCallback, useContext, useEffect, useMemo, useState } from 'react';

import { useMockApi } from '../../api/shared/apiClientUtils';
import { runtimeConfig } from '../../config/runtime';
import { CSRF_HEADER, CSRF_HEADER_VALUE } from './authConstants';
import { AuthStateContext } from './AuthStateContext';
import type { AuthState, AuthStatus, AuthUser } from './authTypes';

const SESSION_URL = '/api/session';
const LOGIN_URL = '/api/login';
const LOGOUT_URL = '/api/logout';
const OIDC_LOGIN_URL = '/api/auth/login';

// Synthetic session used in mock-mode (VITE_USE_MOCK_API=true). The SPA never
// contacts a real BFF in this mode, so the AuthProvider hydrates from this
// object rather than /api/session. Local UI review only — real deployments
// leave VITE_USE_MOCK_API unset.
const MOCK_USER: AuthUser = {
  name: 'Mock Admin',
  email: 'admin@example.dev',
  org: { id: 'org-1', name: 'API Platform Demo', handle: 'api-platform-demo' },
};

type SessionResponse = {
  authenticated: boolean;
  user?: AuthUser;
  /** Set by the BFF only on a 401 for a token that existed but expired —
   * distinguishes that from never having had a session at all, so the SPA
   * can show a "your session expired" message instead of a plain login
   * screen. */
  reason?: 'expired';
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
    if (useMockApi()) {
      setUser(MOCK_USER);
      setStatus('authenticated');
      return;
    }
    try {
      const response = await fetch(SESSION_URL, { credentials: 'same-origin' });
      const body = (await response.json().catch(() => ({}))) as SessionResponse;
      if (!response.ok) {
        setUser(undefined);
        setStatus(body.reason === 'expired' ? 'expired' : 'unauthenticated');
        return;
      }
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
      // App.tsx mounts the SPA under runtimeConfig.appBasePath (already
      // normalized to '' or '/sub-path', see config/runtime.ts) as the router
      // basename — a bare "/login" would send the browser to the app's own
      // root path instead of {basePath}/login when that's set to a sub-path.
      window.location.assign(logoutUrl || `${runtimeConfig.appBasePath}/login`);
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
