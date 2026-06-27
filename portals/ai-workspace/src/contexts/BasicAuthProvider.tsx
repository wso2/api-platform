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

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { AppAuthContext, type AppUser } from './AppAuthContext';
import { checkPermission } from '../auth/permissions';
import { getStoredToken, setStoredToken, clearStoredToken } from '../clients/choreoApiClient';
import { PORTAL_API_BASE_URL } from '../config.env';

export const BASIC_AUTH_SESSION_KEY = 'basic_auth_session';

export function isBasicAuthSession(): boolean {
  return localStorage.getItem(BASIC_AUTH_SESSION_KEY) === 'true';
}

export function clearBasicAuthSession() {
  localStorage.removeItem(BASIC_AUTH_SESSION_KEY);
  clearStoredToken();
}

interface JWTClaims {
  sub?:          string;
  username?:     string;
  scope?:        string;
  organization?: string;
  org_name?:     string;
  org_handle?:   string;
  exp?:          number;
}

function decodeJWTPayload(token: string): JWTClaims | null {
  try {
    const parts = token.split('.');
    if (parts.length !== 3) return null;
    const payload = atob(parts[1].replace(/-/g, '+').replace(/_/g, '/'));
    return JSON.parse(payload) as JWTClaims;
  } catch {
    return null;
  }
}

function isTokenExpired(claims: JWTClaims): boolean {
  if (!claims.exp) return false;
  return Date.now() / 1000 > claims.exp;
}

function claimsToUser(claims: JWTClaims): AppUser {
  return {
    name:   claims.username ?? claims.sub ?? '',
    email:  null,
    role:   null,
    scopes: (claims.scope ?? '').split(' ').filter(Boolean),
    org: {
      id:     claims.organization ?? '',
      name:   claims.org_name ?? '',
      handle: claims.org_handle ?? '',
    },
  };
}

/** POST /api/portal/v0.9/auth/login — validates credentials and stores the returned JWT. */
export async function basicAuthLogin(username: string, password: string): Promise<void> {
  const body = new URLSearchParams({ username, password });
  const res = await fetch(`${PORTAL_API_BASE_URL}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: body.toString(),
  });

  if (!res.ok) {
    const data = await res.json().catch(() => ({})) as Record<string, unknown>;
    throw new Error((data.error as string | undefined) ?? 'Login failed');
  }

  const { token } = await res.json() as { token: string };
  setStoredToken(token);
  localStorage.setItem(BASIC_AUTH_SESSION_KEY, 'true');
}

interface Props {
  children: React.ReactNode;
  onLogout: () => void;
}

export function BasicAuthProvider({ children, onLogout }: Props) {
  const [user, setUser]       = useState<AppUser | null>(null);
  const [token, setToken]     = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const stored = getStoredToken();
    if (!stored) {
      clearBasicAuthSession();
      onLogout();
      return;
    }

    const claims = decodeJWTPayload(stored);
    if (!claims || isTokenExpired(claims)) {
      clearBasicAuthSession();
      onLogout();
      setLoading(false);
      return;
    }

    setToken(stored);
    setUser(claimsToUser(claims));
    setLoading(false);
  }, [onLogout]);

  const logout = useCallback(() => {
    clearBasicAuthSession();
    setToken(null);
    setUser(null);
    onLogout();
  }, [onLogout]);

  const hasPermission = useCallback(
    (scope: string) => checkPermission(user?.scopes ?? [], scope),
    [user],
  );

  const value = useMemo(
    () => ({
      isAuthenticated: token !== null && user !== null,
      isLoading: loading,
      user,
      accessToken: token,
      hasPermission,
      login: async () => {},
      logout,
    }),
    [token, user, loading, hasPermission, logout],
  );

  return <AppAuthContext.Provider value={value}>{children}</AppAuthContext.Provider>;
}
