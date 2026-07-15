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

import { clearStoredToken } from '../clients/choreoApiClient';
import { clearBasicAuthSession } from '../contexts/BasicAuthProvider';
import { CSRF_HEADER, CSRF_VALUE } from '../config.env';
import { logger } from '../utils/logger';

type SignOutFunction = () => Promise<void>;

const BFF_LOGOUT_URL = '/api/logout';
const LOGOUT_REQUEST_TIMEOUT_MS = 5000;

const AUTH_SESSION_KEYS = [
  'platform_auth_token',
  'platform_org_token',
  'ai_workspace_return_url',
] as const;

/**
 * Perform sign out via OIDC end-session endpoint.
 */
export const handleLogout = async (signoutRedirect: SignOutFunction): Promise<void> => {
  try {
    clearAuthData();
    await signoutRedirect();
  } catch (error) {
    logger.error('Sign out error:', error);
    throw error;
  }
};

/**
 * Clear auth-related data from storage (local-only, no redirect).
 * Only removes known auth keys — does not wipe unrelated sessionStorage entries.
 */
export const clearAuthData = (): void => {
  clearStoredToken();
  clearBasicAuthSession();
  AUTH_SESSION_KEYS.forEach((key) => sessionStorage.removeItem(key));
};

/**
 * Best-effort wipe of the browser's Cache Storage ("site cache").
 * Used when forcing a logout after an expired/invalid session so a stale
 * cached response can't keep the user in a broken authenticated state.
 */
const clearSiteCache = async (): Promise<void> => {
  try {
    if (typeof caches !== 'undefined') {
      const keys = await caches.keys();
      await Promise.all(keys.map((key) => caches.delete(key)));
    }
  } catch (error) {
    logger.warn('Failed to clear site cache:', error);
  }
};

/**
 * Force a logout and redirect to the login page.
 *
 * Used when the platform API rejects a request with 401 (expired/invalid
 * session): retrying is futile. We first ask the BFF to tear down the session
 * (`POST /api/logout`) so the HttpOnly `_ai_workspace_session` cookie is cleared
 * server-side — clearing client storage alone leaves that cookie intact, which
 * would silently re-hydrate the dead session on the next `/login` and loop the
 * user back to the error screen. We then clear all client-side session state and
 * the site cache and do a full-page redirect (which also discards in-memory
 * React state tied to the dead session).
 */
export const forceLogoutAndRedirect = async (): Promise<void> => {
  // Each cleanup step is isolated so a failure in one doesn't skip the rest;
  // the redirect below must happen regardless of what was wiped successfully.
  const runStep = (label: string, step: () => void): void => {
    try {
      step();
    } catch (error) {
      logger.warn(`Force logout: failed to ${label}:`, error);
    }
  };

  let logoutUrl: string | undefined;
  const controller = new AbortController();
  const timeoutId = window.setTimeout(() => controller.abort(), LOGOUT_REQUEST_TIMEOUT_MS);
  try {
    const res = await fetch(BFF_LOGOUT_URL, {
      method: 'POST',
      credentials: 'include',
      headers: { Accept: 'application/json', [CSRF_HEADER]: CSRF_VALUE },
      signal: controller.signal,
    });
    if (res.ok) {
      const body = (await res.json().catch(() => ({}))) as { logoutUrl?: string };
      logoutUrl = body.logoutUrl;
    }
  } catch (error) {
    logger.warn('Force logout: BFF logout call failed:', error);
  } finally {
    window.clearTimeout(timeoutId);
  }

  runStep('clear auth data', clearAuthData);
  // Wipe any remaining client-side state so nothing carries over the session.
  runStep('clear sessionStorage', () => sessionStorage.clear());
  runStep('clear localStorage', () => localStorage.clear());
  await clearSiteCache();

  // Prefer the IDP end-session URL (fully logs out at the IDP); otherwise fall
  // back to /login. Replace (not assign) so the broken page isn't left in history.
  if (logoutUrl) {
    window.location.replace(logoutUrl);
    return;
  }
  window.location.replace('/login');
};
