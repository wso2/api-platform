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
import { BASE_PATH } from '../paths';

// BFF auth endpoints are same-origin (the BFF serves this SPA), and mounted under the
// same base path it serves the SPA on — these are absolute paths, not router routes,
// so the prefix has to be spelled out.
const SESSION_URL = `${BASE_PATH}/api/session`;
const LOGOUT_URL = `${BASE_PATH}/api/logout`;
const OIDC_LOGIN_URL = `${BASE_PATH}/api/auth/login`;

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
    organizations?: string[];
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
    organizations: u.organizations ?? [],
  };
}

/** Default headers for BFF control-plane calls (session/logout). */
function bffHeaders(): Record<string, string> {
  return { Accept: 'application/json', [CSRF_HEADER]: CSRF_VALUE };
}

export function BFFAuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<AppUser | null>(null);
  const [loading, setLoading] = useState(true);
  // A 502 from the BFF means it could not mint the upstream token — the session
  // itself is intact. Kept apart from `user === null` so route guards can hold the
  // route instead of redirecting to /login, which over a transient IDP blip would
  // log the user out of a perfectly good session and into a login that fails the
  // same way (the OIDC callback runs the same exchange).
  const [unavailable, setUnavailable] = useState(false);

  // One place that reads the session and decides what each status means, shared by
  // the initial hydrate, the retry timer, and the manual Retry button — so those
  // three can never drift into disagreeing about what a 502 is.
  const loadSession = useCallback(async (): Promise<void> => {
    try {
      const res = await fetch(SESSION_URL, { credentials: 'include', headers: bffHeaders() });
      if (res.status === 502) {
        // Leave `user` untouched: an already-hydrated session keeps working from
        // what it has, and a first load simply has nothing yet.
        setUnavailable(true);
        return;
      }
      setUnavailable(false);
      if (!res.ok) {
        setUser(null);
        return;
      }
      const body = (await res.json()) as SessionResponse;
      setUser(body.authenticated ? toAppUser(body.user) : null);
    } catch {
      setUser(null);
    }
  }, []);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        await loadSession();
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => { cancelled = true; };
  }, [loadSession]);

  // Recover on our own once the IDP comes back, so a blip costs a few seconds of
  // notice rather than a dead tab the user has to reload. Polling only while
  // unavailable keeps it off the healthy path entirely, and the BFF's own cooldown
  // gate answers these immediately instead of spending a full exchange timeout on
  // each — so this cannot pile load onto the component that is already failing.
  useEffect(() => {
    if (!unavailable) return undefined;
    const timer = setInterval(() => { void loadSession(); }, 5000);
    return () => clearInterval(timer);
  }, [unavailable, loadSession]);

  const refreshSession = useCallback(() => loadSession(), [loadSession]);

  // Fetch the current JWT fresh from the session endpoint rather than returning
  // a value captured at hydrate time: the BFF proxy rotates the cookie token
  // near expiry, so a cached snapshot would go stale. We also re-sync the user
  // in case claims changed across the rotation.
  const getAccessToken = useCallback(async (): Promise<string | null> => {
    try {
      const res = await fetch(SESSION_URL, { credentials: 'include', headers: bffHeaders() });
      // Same classification as loadSession: a 502 is the BFF failing to mint the
      // upstream token, not a dead session, so the caller gets no token but the
      // hydrated user stands.
      if (res.status === 502) {
        setUnavailable(true);
        return null;
      }
      setUnavailable(false);
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
    window.location.replace(`${BASE_PATH}/login`);
  }, []);

  const hasPermission = useCallback(
    (scope: string) => checkPermission(user?.scopes ?? [], scope),
    [user],
  );

  const value = useMemo(
    () => ({
      isAuthenticated: user !== null,
      isLoading: loading,
      sessionUnavailable: unavailable,
      refreshSession,
      user,
      getAccessToken, // fresh JWT from the BFF session (also injected by the proxy)
      hasPermission,
      login,
      logout,
    }),
    [user, loading, unavailable, refreshSession, getAccessToken, hasPermission, login, logout],
  );

  return <AppAuthContext.Provider value={value}>{children}</AppAuthContext.Provider>;
}
