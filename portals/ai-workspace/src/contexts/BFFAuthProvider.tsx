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

/*
 * BFFAuthProvider — the single auth provider for both file-based and OIDC modes.
 *
 * The browser never holds a token. This provider hydrates the user from the BFF
 * session endpoint (GET /api/session, sent with the HttpOnly cookie). Login and
 * logout are delegated to the BFF:
 *   - OIDC: full-page redirect to /api/auth/login (the BFF does the code exchange)
 *   - basic: the login page POSTs /api/login, then the app reloads to re-hydrate
 *   - logout: POST /api/logout, then follow the IDP end-session URL (OIDC) or /login
 */

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { AppAuthContext, type AppUser, type AppOrg } from './AppAuthContext';
import { checkPermission, isPlatformRole } from '../auth/permissions';
import { AUTH_MODE, CSRF_HEADER, CSRF_VALUE } from '../config.env';

// BFF auth endpoints are same-origin (the BFF serves this SPA).
const SESSION_URL = '/api/session';
const LOGOUT_URL = '/api/logout';
const OIDC_LOGIN_URL = '/api/auth/login';

interface SessionResponse {
  authenticated: boolean;
  user?: {
    name?: string | null;
    email?: string | null;
    role?: string | null;
    scopes?: string[];
    org?: { id: string; name: string; handle: string } | null;
  };
}

function toAppUser(u: SessionResponse['user']): AppUser | null {
  if (!u) return null;
  const org: AppOrg | null = u.org && (u.org.id || u.org.handle)
    ? { id: u.org.id ?? '', name: u.org.name ?? u.org.handle ?? '', handle: u.org.handle ?? '' }
    : null;
  return {
    name: u.name ?? null,
    email: u.email ?? null,
    role: isPlatformRole(u.role) ? u.role : null,
    scopes: u.scopes ?? [],
    org,
  };
}

/** Default headers for BFF control-plane calls (session/logout). */
function bffHeaders(): Record<string, string> {
  return { Accept: 'application/json', [CSRF_HEADER]: CSRF_VALUE };
}

export function BFFAuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<AppUser | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const res = await fetch(SESSION_URL, { credentials: 'include', headers: bffHeaders() });
        if (!cancelled && res.ok) {
          const body = (await res.json()) as SessionResponse;
          setUser(body.authenticated ? toAppUser(body.user) : null);
        } else if (!cancelled) {
          setUser(null);
        }
      } catch {
        if (!cancelled) setUser(null);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => { cancelled = true; };
  }, []);

  const login = useCallback(async () => {
    if (AUTH_MODE === 'oidc') {
      const ret = encodeURIComponent(window.location.pathname + window.location.search);
      window.location.href = `${OIDC_LOGIN_URL}?return=${ret}`;
    }
    // Basic mode: the login page calls POST /api/login and reloads on success.
  }, []);

  const logout = useCallback(async () => {
    try {
      const res = await fetch(LOGOUT_URL, {
        method: 'POST',
        credentials: 'include',
        headers: bffHeaders(),
      });
      if (res.ok) {
        const body = (await res.json().catch(() => ({}))) as { logoutUrl?: string };
        if (body.logoutUrl) {
          window.location.href = body.logoutUrl;
          return;
        }
      }
    } catch {
      /* fall through to local redirect */
    }
    window.location.replace('/login');
  }, []);

  const hasPermission = useCallback(
    (scope: string) => checkPermission(user?.scopes ?? [], scope),
    [user],
  );

  const value = useMemo(
    () => ({
      isAuthenticated: user !== null,
      isLoading: loading,
      user,
      accessToken: null, // tokens live in the BFF session, never in the browser
      hasPermission,
      login,
      logout,
    }),
    [user, loading, hasPermission, login, logout],
  );

  return <AppAuthContext.Provider value={value}>{children}</AppAuthContext.Provider>;
}
