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

import React, { useCallback, useEffect, useMemo } from 'react';
import { useAuth } from 'react-oidc-context';
import { AppAuthContext, type AppUser } from './AppAuthContext';
import { OIDC_USERNAME_CLAIM, OIDC_EMAIL_CLAIM, PERMISSION_MODE } from '../config.env';
import { expandScopes, ROLE_SCOPES, checkPermission, isPlatformRole } from '../auth/permissions';
import type { PlatformRole } from '../auth/permissions';
import { handleLogout } from '../auth/logout';
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
  const scope = payload.scope;
  if (typeof scope === 'string') return scope.split(' ').filter(Boolean);
  if (Array.isArray(scope)) return scope as string[];
  return [];
}

function extractRoleFromJwt(token: string): PlatformRole | null {
  const payload = decodeJwtPayload(token);
  const role = payload.platform_role ?? payload.role;
  return isPlatformRole(role) ? role : null;
}

export function OIDCAppAuthProvider({ children }: { children: React.ReactNode }) {
  const auth = useAuth();

  useEffect(() => {
    const token = auth.user?.access_token;
    if (token) setStoredToken(token);
  }, [auth.user?.access_token]);

  const user: AppUser | null = useMemo(() => {
    if (!auth.user) return null;
    // ID token claims (profile) and access token claims may differ —
    // check both so given_name/email are found regardless of which token carries them.
    const idClaims = auth.user.profile as Record<string, unknown>;
    const token = auth.user.access_token;
    const atClaims = token ? decodeJwtPayload(token) : {};
    const rawScopes = token ? extractScopesFromJwt(token) : [];
    const role = token ? extractRoleFromJwt(token) : null;
    const scopes = PERMISSION_MODE === 'role' && role
      ? expandScopes(ROLE_SCOPES[role] ?? [])
      : expandScopes(rawScopes);
    const claim = (key: string) =>
      (idClaims[key] as string | undefined) || (atClaims[key] as string | undefined) || null;
    return {
      name: claim(OIDC_USERNAME_CLAIM),
      email: claim(OIDC_EMAIL_CLAIM),
      role,
      scopes,
    };
  }, [auth.user]);

  const login = useCallback(async () => {
    await auth.signinRedirect();
  }, [auth]);

  const logout = useCallback(async () => {
    await handleLogout(() => auth.signoutRedirect());
  }, [auth]);

  const hasPermission = useCallback(
    (scope: string) => checkPermission(user?.scopes ?? [], scope),
    [user]
  );

  const value = useMemo(
    () => ({
      isAuthenticated: auth.isAuthenticated,
      isLoading: auth.isLoading,
      user,
      accessToken: auth.user?.access_token ?? null,
      hasPermission,
      login,
      logout,
    }),
    [auth.isAuthenticated, auth.isLoading, auth.user?.access_token, user, hasPermission, login, logout]
  );

  return <AppAuthContext.Provider value={value}>{children}</AppAuthContext.Provider>;
}
