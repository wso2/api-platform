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

import { ReactNode, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';

import {
  buildLoginUrl,
  supportsOrganizationSwitch,
  withAppBasePath,
  type SessionFormat,
} from '../../config/authContract';
import { runtimeConfig } from '../../config/runtime';
import { CSRF_HEADER, CSRF_HEADER_VALUE } from './authConstants';
import { AuthStateContext } from './AuthStateContext';
import {
  OrganizationSwitchError,
  type AuthOrg,
  type AuthState,
  type AuthStatus,
  type AuthUser,
} from './authTypes';
import { hasScopeClaim, userFromClaims } from './userFromClaims';
import { onUnauthorized } from '../../api/core/sessionEvents';

const contract = runtimeConfig.authContract;
const canSwitchOrganization = supportsOrganizationSwitch(contract);

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
const UNAVAILABLE: SessionCheck = { status: 'unavailable' };

/**
 * How long a session check may take. Above the gateway's own upstream timeout,
 * so its 504 normally arrives first; this covers a connection that hangs.
 */
const SESSION_CHECK_TIMEOUT_MS = 15_000;

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
  if (!response.ok) return SIGNED_OUT;
  const claims: unknown = await response.json();
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
  if (!response.ok) return SIGNED_OUT;
  const body: unknown = await response.json();
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

/** Asks a session endpoint who is signed in. Never throws. */
async function askSession(url: string): Promise<SessionCheck> {
  let response: Response;
  try {
    response = await fetch(url, { credentials: 'same-origin' });
  } catch {
    return UNAVAILABLE;
  }
  if (response.status >= 500) return UNAVAILABLE;
  try {
    return await SESSION_READERS[contract.sessionFormat](response);
  } catch {
    // A 2xx whose body breaks off or isn't JSON is no answer either. Signed
    // out would send a signed-in browser to sign-in, and in a loop when a
    // misrouted session URL serves a page instead.
    return UNAVAILABLE;
  }
}

/**
 * Asks with a time limit. A 5xx, no answer or no answer in time is
 * `unavailable`, not signed out: the session may well be alive.
 */
async function checkSession(url: string): Promise<SessionCheck> {
  return within(askSession(url), SESSION_CHECK_TIMEOUT_MS, UNAVAILABLE);
}

/** Settles `request` with `onTimeout` if it takes longer than `ms`. */
async function within<T>(request: Promise<T>, ms: number, onTimeout: T): Promise<T> {
  let timer: ReturnType<typeof setTimeout> | undefined;
  const timedOut = new Promise<T>((resolve) => {
    timer = setTimeout(() => resolve(onTimeout), ms);
  });
  try {
    return await Promise.race([request, timedOut]);
  } finally {
    clearTimeout(timer);
  }
}

const isOrganization = (value: unknown): value is AuthOrg =>
  isObject(value) &&
  typeof value.id === 'string' &&
  value.id !== '' &&
  typeof value.handle === 'string' &&
  value.handle !== '';

/**
 * The organizations the user can switch to, as ThunderID's `/users/me/ous`
 * answers them: `{organizationUnits: [{id, handle, name}]}`.
 */
async function fetchOrganizations(url: string): Promise<AuthOrg[]> {
  const response = await fetch(url, { credentials: 'same-origin' });
  if (!response.ok) throw new OrganizationSwitchError('unavailable');
  const body: unknown = await response.json();
  const units = isObject(body) && Array.isArray(body.organizationUnits) ? body.organizationUnits : [];
  return units.filter(isOrganization).map(({ id, handle, name }) => ({
    id,
    handle,
    name: typeof name === 'string' && name !== '' ? name : handle,
  }));
}

/**
 * Moves one session into the organization `organizationId`, as the ThunderID
 * SDK's backend-mode `ouSwitch` asks: the unit in the query and the JSON body.
 * Resolves once it has.
 */
async function requestOrganizationSwitch(url: string, organizationId: string): Promise<void> {
  const separator = url.includes('?') ? '&' : '?';
  const send = fetch(`${url}${separator}ouId=${encodeURIComponent(organizationId)}`, {
    method: 'POST',
    credentials: 'same-origin',
    headers: mutatingHeaders,
    body: JSON.stringify({ ouId: organizationId }),
  }).catch(() => undefined);
  const response = await within(send, SESSION_CHECK_TIMEOUT_MS, undefined);
  if (response?.status === 403) throw new OrganizationSwitchError('forbidden');
  if (!response?.ok) throw new OrganizationSwitchError('unavailable');
}

/** Field for field, so a check that changes nothing doesn't re-render the app. */
const sameUser = (a?: AuthUser, b?: AuthUser) => JSON.stringify(a) === JSON.stringify(b);

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

  // Set once logout starts: a session check answering later would redirect to
  // sign-in and cancel the logout's own navigation.
  const leaving = useRef(false);

  // Only the newest check may set state: an older one answering late would
  // undo it.
  const latestCheck = useRef(0);

  // The organization the session is in, as the newest check reported it. A ref
  // so a switch can read it the moment the check answers, before a re-render.
  const sessionOrgHandle = useRef<string | undefined>(undefined);

  /**
   * Takes `url`'s answer and resolves whether the session is alive. Each caller
   * gets its own check's answer, even when a newer check set the state.
   */
  const hydrate = useCallback(async (url: string = contract.sessionUrl) => {
    const id = ++latestCheck.current;
    let session = await checkSession(url);
    // A check can't renew an ended session; the session read can (Native SSO), so ask it first.
    if (url !== contract.sessionUrl && (session.status === 'unauthenticated' || session.status === 'expired')) {
      session = await checkSession(contract.sessionUrl);
    }
    if (leaving.current) return false;
    if (id !== latestCheck.current) return session.status === 'authenticated';
    if (session.status === 'unavailable') {
      setStatus((current) => (current === 'authenticated' ? current : 'unavailable'));
      return false;
    }
    sessionOrgHandle.current = session.user?.org?.handle;
    setUser((current) => (sameUser(current, session.user) ? current : session.user));
    setStatus(session.status);
    return session.status === 'authenticated';
  }, []);

  const [organizations, setOrganizations] = useState<AuthOrg[]>();
  const organizationsRequest = useRef<Promise<AuthOrg[]> | undefined>(undefined);

  /** Loads the organization list once per sign-in; a failed load is retried on next use. */
  const loadOrganizations = useCallback(() => {
    organizationsRequest.current ??= fetchOrganizations(contract.organizationsUrl).then(
      (list) => {
        setOrganizations(list);
        return list;
      },
      (error: unknown) => {
        organizationsRequest.current = undefined;
        throw error;
      },
    );
    return organizationsRequest.current;
  }, []);

  useEffect(() => {
    if (!canSwitchOrganization) return;
    if (status === 'authenticated') {
      loadOrganizations().catch(() => undefined);
    } else if (status === 'unauthenticated' || status === 'expired') {
      organizationsRequest.current = undefined;
      setOrganizations(undefined);
    }
  }, [loadOrganizations, status]);

  // Switches run one after another, so the last one asked for is the one the
  // session ends up in.
  const switchQueue = useRef<Promise<unknown>>(Promise.resolve());

  const exchangeOrgToken = useCallback(
    (orgHandle: string) => {
      if (!canSwitchOrganization) return Promise.resolve(true);
      const switchTo = async () => {
        if (sessionOrgHandle.current === orgHandle) return true;
        const target = (await loadOrganizations()).find((org) => org.handle === orgHandle);
        if (!target) throw new OrganizationSwitchError('forbidden');
        for (const url of contract.switchOrganizationUrls) {
          await requestOrganizationSwitch(url, target.id);
        }
        // The switched session's claims name its new organization.
        await hydrate();
        if (sessionOrgHandle.current !== orgHandle) throw new OrganizationSwitchError('unavailable');
        return true;
      };
      const result = switchQueue.current.then(switchTo);
      switchQueue.current = result.catch(() => undefined);
      return result;
    },
    [hydrate, loadOrganizations],
  );

  useEffect(() => {
    void hydrate();
  }, [hydrate]);

  /**
   * Re-checks the session on a 401 and tells the transport whether to send the
   * request again. Behind the gateway the check itself refreshes a rejected
   * token, or signs the browser back in through Native SSO, so the answer is
   * often yes. Otherwise `status` changes and `ProtectedRoute` redirects.
   */
  useEffect(() => onUnauthorized(() => hydrate()), [hydrate]);

  /**
   * While signed in, polls `checkSessionUrl` when the tab is visible, and
   * checks when it is shown again, so an open tab notices a session that ended
   * elsewhere (a sign-out in another tab, the idle timeout).
   */
  useEffect(() => {
    const url = contract.checkSessionUrl;
    if (!url || status !== 'authenticated') return undefined;
    let checking = false;
    const check = () => {
      if (checking || document.visibilityState !== 'visible') return;
      checking = true;
      void hydrate(url).finally(() => {
        checking = false;
      });
    };
    const timer = setInterval(check, contract.checkSessionInterval * 1000);
    document.addEventListener('visibilitychange', check);
    return () => {
      clearInterval(timer);
      document.removeEventListener('visibilitychange', check);
    };
  }, [hydrate, status]);

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
        sessionOrgHandle.current = body.user?.org?.handle;
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
    // A second sign-out could navigate to /login and cancel the first one's
    // navigation to the IDP.
    if (leaving.current) return;
    leaving.current = true;
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
        // AI Workspace BFF provider. `redirectUrl` is the gateway policy's name.
        if (response.ok) {
          const body = (await response.json().catch(() => ({}))) as {
            logoutUrl?: string;
            redirectUrl?: string;
          };
          logoutUrl = body.logoutUrl ?? body.redirectUrl;
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
      canSwitchOrganization,
      organizations,
      exchangeOrgToken,
      logout,
    }),
    [error, exchangeOrgToken, login, loginWithCredentials, logout, organizations, status, user]
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
