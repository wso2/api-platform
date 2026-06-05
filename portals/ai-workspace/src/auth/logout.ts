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
  AUTH_SESSION_KEYS.forEach((key) => sessionStorage.removeItem(key));
};
