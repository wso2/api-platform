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
import { logger } from '../utils/logger';

type SignOutFunction = () => Promise<void>;

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
 * Force a logout without an IDP round-trip and redirect to the login page.
 *
 * Used when the platform API rejects a request with 401 (expired/invalid
 * session): retrying is futile, so we clear all client-side session state and
 * the site cache, then do a full-page redirect to /login. The full reload
 * (rather than a router navigation) also discards any in-memory React state
 * tied to the dead session.
 */
export const forceLogoutAndRedirect = async (): Promise<void> => {
  try {
    clearAuthData();
    // Wipe any remaining client-side state so nothing carries over the session.
    sessionStorage.clear();
    localStorage.clear();
    await clearSiteCache();
  } catch (error) {
    logger.error('Force logout error:', error);
  } finally {
    // Replace (not assign) so the broken page isn't left in history.
    window.location.replace('/login');
  }
};
