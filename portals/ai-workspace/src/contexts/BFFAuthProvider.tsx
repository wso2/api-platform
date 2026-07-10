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
 * This provider hydrates the user from the BFF session endpoint
 * (GET /api/session, sent with the HttpOnly cookie), which also returns the full
 * JWT for call-sites that need the raw token (the BFF proxy still injects the
 * same token upstream, so most API calls don't need it). Login and
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
  // The full JWT minted/obtained by the BFF. The BFF still injects this same
  // token when proxying (the browser need not send it), but it is surfaced here
  // for call-sites that require the raw token.
  accessToken?: string | null;
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

  // Fetch the current JWT fresh from the session endpoint rather than returning
  // a value captured at hydrate time: the BFF proxy rotates the cookie token
  // near expiry, so a cached snapshot would go stale. We also re-sync the user
  // in case claims changed across the rotation.
  const getAccessToken = useCallback(async (): Promise<string | null> => {
    try {
      const res = await fetch(SESSION_URL, { credentials: 'include', headers: bffHeaders() });
      if (!res.ok) return null;
      const body = (await res.json()) as SessionResponse;
      if (!body.authenticated) {
        setUser(null);
        return null;
      }
      setUser(toAppUser(body.user));
      return body.accessToken ?? null;
    } catch {
      return null;
    }
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
      getAccessToken, // fresh JWT from the BFF session (also injected by the proxy)
      hasPermission,
      login,
      logout,
    }),
    [user, loading, getAccessToken, hasPermission, login, logout],
  );

  return <AppAuthContext.Provider value={value}>{children}</AppAuthContext.Provider>;
}
