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
 * Logout functionality for AI Workspace using Asgardeo SDK
 */

import { removeFidpId } from '../utils/cookies';
import { logger } from '../utils/logger';

type SignOutFunction = () => Promise<boolean>;

/**
 * Perform sign out
 * @param signOut - The signOut function from useAuthContext
 */
export const handleLogout = async (signOut: SignOutFunction): Promise<void> => {
  try {
    // Clear fidpId cookie so user isn't auto-redirected to the same IDP on next visit
    removeFidpId();
    sessionStorage.removeItem('idpId');
    sessionStorage.removeItem('currentOrgHandle');
    // Call Asgardeo signOut
    await signOut();
  } catch (error) {
    logger.error('Sign out error:', error);
    throw error;
  }
};

/**
 * Clear all auth-related data from storage
 * This can be used for local-only logout without redirecting to Asgardeo
 */
export const clearAuthData = (): void => {
  // Clear fidpId cookie
  removeFidpId();
  // Clear session storage items that might be set by Asgardeo
  sessionStorage.clear();
};
