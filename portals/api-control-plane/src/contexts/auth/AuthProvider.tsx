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

import { buildLoginUrl, withAppBasePath, type SessionFormat } from '../../config/authContract';
import { runtimeConfig } from '../../config/runtime';
import { CSRF_HEADER, CSRF_HEADER_VALUE } from './authConstants';
import { AuthStateContext } from './AuthStateContext';
import type { AuthState, AuthStatus, AuthUser } from './authTypes';
import { hasScopeClaim, userFromClaims } from './userFromClaims';
import { onSessionExpired } from '../../api/core/sessionEvents';

const contract = runtimeConfig.authContract;

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

type SessionCheck = { status: Exclude<AuthStatus, 'loading'>; user?: AuthUser };

const SIGNED_OUT: SessionCheck = { status: 'unauthenticated' };

const isObject = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

/**
 * This portal's BFF: `{authenticated, user}` with the user already resolved,
 * and a 401 with `reason: 'expired'` for a session that has expired.
 */
async function readBffSession(response: Response): Promise<SessionCheck> {
  const body = (await response.json().catch(() => ({}))) as SessionResponse;
  if (!response.ok) {
    return { status: body.reason === 'expired' ? 'expired' : 'unauthenticated' };
  }
  return { status: body.authenticated ? 'authenticated' : 'unauthenticated', user: body.user };
}

/**
 * A gateway or auth proxy that answers with the signed-in user's claims, and
 * with a non-2xx when nobody is signed in (it can't say "expired"). Scopes
 * come from the scope claim. Without one they stay unknown, not "none
 * granted": PermissionProvider then offers every action and leaves the
 * decision to the Platform API.
 */
async function readClaimsSession(response: Response): Promise<SessionCheck> {
  const claims: unknown = response.ok ? await response.json().catch(() => undefined) : undefined;
  if (!isObject(claims)) return SIGNED_OUT;
  const user = userFromClaims(claims, contract.claimMappings);
  if (!hasScopeClaim(claims, contract.claimMappings.scope)) delete user.scopes;
  return { status: 'authenticated', user };
}

/**
 * The ThunderID SDK's backend-mode session, as the gateway's `oauth2-session`
 * policy answers it: `{isSignedIn, user, expiresAt, scope}`, or
 * `{isSignedIn: false}` (or a non-2xx) when nobody is signed in. `user` holds
 * the ID token's claims. The granted scopes come beside it in `scope`,
 * space-separated as in a token response; the SDK's getGrantedPermissions()
 * reads the same field. Without `scope` they stay unknown, as above.
 */
async function readSdkSession(response: Response): Promise<SessionCheck> {
  const body: unknown = response.ok ? await response.json().catch(() => undefined) : undefined;
  if (!isObject(body) || body.isSignedIn !== true) return SIGNED_OUT;
  const user = userFromClaims(isObject(body.user) ? body.user : {}, contract.claimMappings);
  if (typeof body.scope === 'string') {
    user.scopes = body.scope.split(' ').filter((scope) => scope !== '');
  } else {
    delete user.scopes;
  }
  return { status: 'authenticated', user };
}

const SESSION_READERS: Record<SessionFormat, (response: Response) => Promise<SessionCheck>> = {
  bff: readBffSession,
  claims: readClaimsSession,
  sdk: readSdkSession,
};

/** Asks the session endpoint who is signed in. Never throws: a failed request reads as signed out. */
async function checkSession(): Promise<SessionCheck> {
  try {
    const response = await fetch(contract.sessionUrl, { credentials: 'same-origin' });
    return await SESSION_READERS[contract.sessionFormat](response);
  } catch {
    return SIGNED_OUT;
  }
}

/**
 * Something in front of the console owns authentication — this portal's BFF,
 * or a gateway or auth proxy acting as the BFF — and holds the session
 * server-side behind an HttpOnly cookie; the browser never sees a token. This
 * provider only ever talks to that component's same-origin endpoints — never
 * an IdP SDK — so there is exactly one implementation for every auth mode.
 * Which endpoints, and how they answer, is `runtimeConfig.authContract`
 * (see config/authContract.ts):
 *
 *   - basic mode: loginWithCredentials() posts to the BFF's /api/login.
 *   - oidc mode: login() redirects to the BFF's /api/auth/login, which runs
 *     the whole code exchange server-side and redirects back with the session
 *     cookie already set — this provider only needs to (re-)hydrate from
 *     /api/session afterwards, same as on first load.
 *   - external mode: the same redirect, to the gateway's or proxy's sign-in
 *     URL. The session endpoint answers with the user's claims, directly
 *     (`claims` format) or in the ThunderID SDK's backend-mode session
 *     (`sdk`), and userFromClaims() resolves them exactly as the BFF would.
 */
export function AuthProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<AuthStatus>('loading');
  const [user, setUser] = useState<AuthUser>();
  const [error, setError] = useState<string>();

  const hydrate = useCallback(async () => {
    const session = await checkSession();
    setUser(session.user);
    setStatus(session.status);
  }, []);

  useEffect(() => {
    void hydrate();
  }, [hydrate]);

  /**
   * Re-checks the session when the API layer reports a 401.
   *
   * Without this, an expired session surfaced only as individual failed
   * requests: the user sat on a screen where nothing worked and nothing
   * explained why. Re-hydrating asks the session endpoint what is actually
   * true; it may have renewed the session server-side, in which case the answer is
   * "still authenticated" and nothing further happens; otherwise `status`
   * becomes unauthenticated (or `expired`) and `ProtectedRoute` redirects.
   *
   * The transport publishes rather than calling in, so it never imports this
   * provider; that would be a cycle and would make the client untestable. It
   * also debounces, so a screen firing eight parallel requests against a dead
   * session triggers one re-check, not eight.
   */
  useEffect(() => onSessionExpired(() => void hydrate()), [hydrate]);

  // Every sign-in endpoint redirects back to a path from the host root, while
  // callers pass a path inside the app; add the base path the app is mounted
  // under (App.tsx's router basename) so a console under a sub-path comes back
  // to itself.
  const login = useCallback((returnTo?: string) => {
    const target =
      returnTo === undefined
        ? window.location.pathname
        : withAppBasePath(returnTo, runtimeConfig.appBasePath);
    window.location.assign(buildLoginUrl(contract, target));
  }, []);

  const loginWithCredentials = useCallback(
    async (username: string, password: string) => {
      setError(undefined);
      try {
        const response = await fetch(contract.credentialsUrl, {
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
    // A gateway or proxy ends the session on a plain navigation and redirects
    // on its own. Leave without touching local state, for the same reason as
    // the IDP case below.
    if (contract.logoutStyle === 'navigate') {
      window.location.assign(contract.logoutUrl);
      return;
    }
    void (async () => {
      let logoutUrl: string | undefined;
      try {
        const response = await fetch(contract.logoutUrl, {
          method: 'POST',
          credentials: 'same-origin',
          headers: mutatingHeaders,
        });
        // Any 2xx ends the session server-side. The body is only read for the
        // IDP end-session URL, which a file-based logout (204, no body) has not
        // got — so a parse failure is expected there, not an error. Mirrors the
        // AI Workspace BFF provider.
        if (response.ok) {
          const body = (await response.json().catch(() => ({}))) as { logoutUrl?: string };
          logoutUrl = body.logoutUrl;
        }
      } catch {
        // Best-effort: proceed to clear local state and navigate away regardless.
      }
      // Leave for the IDP before touching local state. Flipping status to
      // 'unauthenticated' re-renders the app, and a guard reacting to that sends
      // the browser to /login, which preempts this navigation and tears down the
      // in-flight end-session request — the IDP logs "context canceled" and the
      // session it was asked to end survives.
      if (logoutUrl) {
        window.location.assign(logoutUrl);
        return;
      }
      setUser(undefined);
      setStatus('unauthenticated');
      // App.tsx mounts the SPA under runtimeConfig.appBasePath (already
      // normalized to '' or '/sub-path', see config/runtime.ts) as the router
      // basename — a bare "/login" would send the browser to the app's own
      // root path instead of {basePath}/login when that's set to a sub-path.
      window.location.assign(`${runtimeConfig.appBasePath}/login`);
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
