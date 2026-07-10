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

import React, { useCallback, useMemo } from 'react';
import { useAuth } from 'react-oidc-context';
import { AppAuthContext, type AppUser, type AppOrg } from './AppAuthContext';
import {
  OIDC_USERNAME_CLAIM,
  OIDC_EMAIL_CLAIM,
  OIDC_ORG_ID_CLAIM,
  OIDC_ORG_NAME_CLAIM,
  OIDC_ORG_HANDLE_CLAIM,
} from '../config.env';
import { checkPermission, isPlatformRole } from '../auth/permissions';
import type { PlatformRole } from '../auth/permissions';
import { clearAuthData } from '../auth/logout';
import { setStoredToken } from '../clients/choreoApiClient';

function decodeJwtPayload(token: string): Record<string, unknown> {
  try {
    return JSON.parse(atob(token.split('.')[1]));
  } catch {
    return {};
  }
}

function extractScopesFromJwt(token: string): string[] {
  const payload = decodeJwtPayload(token);
  // Check 'scope' (standard) and 'scp' (Asgardeo / Azure AD variant) claims.
  const raw = payload.scope ?? payload.scp;
  if (typeof raw === 'string') return raw.split(' ').filter(Boolean);
  if (Array.isArray(raw)) return (raw as unknown[]).filter((s): s is string => typeof s === 'string');
  return [];
}

function extractRoleFromJwt(token: string): PlatformRole | null {
  const payload = decodeJwtPayload(token);
  const role = payload.platform_role ?? payload.role;
  return isPlatformRole(role) ? role : null;
}

export function OIDCAppAuthProvider({ children }: { children: React.ReactNode }) {
  const auth = useAuth();

  // Store synchronously during render so downstream contexts that run in
  // useEffect (which fires bottom-up, before this component's effects) always
  // find a valid token in sessionStorage when they first call the platform API.
  if (auth.user?.access_token) {
    setStoredToken(auth.user.access_token);
  }

  const user: AppUser | null = useMemo(() => {
    if (!auth.user) return null;
    // ID token claims (profile) and access token claims may differ —
    // check both so given_name/email are found regardless of which token carries them.
    const idClaims = auth.user.profile as Record<string, unknown>;
    const token = auth.user.access_token;
    const atClaims = token ? decodeJwtPayload(token) : {};
    const rawScopes = token ? extractScopesFromJwt(token) : [];
    const role = token ? extractRoleFromJwt(token) : null;
    const scopes = [...new Set(rawScopes)];
    const claim = (key: string) =>
      (idClaims[key] as string | undefined) || (atClaims[key] as string | undefined) || null;

    const orgId     = (atClaims[OIDC_ORG_ID_CLAIM]     as string | undefined) || null;
    const orgName   = (atClaims[OIDC_ORG_NAME_CLAIM]   as string | undefined) || null;
    const orgHandle = (atClaims[OIDC_ORG_HANDLE_CLAIM] as string | undefined) || null;
    const org: AppOrg | null = (orgId || orgHandle)
      ? { id: orgId ?? '', name: orgName ?? orgHandle ?? '', handle: orgHandle ?? '' }
      : null;

    return {
      name: claim(OIDC_USERNAME_CLAIM),
      email: claim(OIDC_EMAIL_CLAIM),
      role,
      scopes,
      org,
    };
  }, [auth.user]);

  const login = useCallback(async () => {
    await auth.signinRedirect();
  }, [auth]);

  const logout = useCallback(async () => {
    clearAuthData();
    // Always clear OIDC state locally first so the user is logged out of the
    // app regardless of what the IDP does next.
    const idToken = auth.user?.id_token;
    await auth.removeUser();
    try {
      // Pass id_token_hint so IDPs that require it (e.g. Thunder, Asgardeo)
      // don't show their own "Something Went Wrong" error page.
      await auth.signoutRedirect({ id_token_hint: idToken });
    } catch {
      // IDP has no end_session_endpoint or the call failed — user is already
      // logged out locally, just send them to the sign-in page.
      window.location.href = '/login';
    }
  }, [auth]);

  const hasPermission = useCallback(
    (scope: string) => checkPermission(user?.scopes ?? [], scope),
    [user]
  );

  // Read the live token off the auth user; react-oidc-context silently renews it,
  // so this never returns a stale snapshot.
  const getAccessToken = useCallback(
    async (): Promise<string | null> => auth.user?.access_token ?? null,
    [auth]
  );

  const value = useMemo(
    () => ({
      isAuthenticated: auth.isAuthenticated,
      isLoading: auth.isLoading,
      user,
      getAccessToken,
      hasPermission,
      login,
      logout,
    }),
    [auth.isAuthenticated, auth.isLoading, user, getAccessToken, hasPermission, login, logout]
  );

  return <AppAuthContext.Provider value={value}>{children}</AppAuthContext.Provider>;
}
