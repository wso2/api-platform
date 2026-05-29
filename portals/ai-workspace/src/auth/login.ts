/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/*
 * Login functionality for AI Workspace using Asgardeo SDK
 */

import {
  FIDP_GOOGLE,
  FIDP_GITHUB,
  FIDP_MICROSOFT,
  FIDP_ENTERPRISE,
} from '../config.env';
import { logger } from '../utils/logger';
import { setFidpId } from '../utils/cookies';

// Using 'any' to avoid type conflicts with Asgardeo SDK
// eslint-disable-next-line @typescript-eslint/no-explicit-any
type SignInFunction = (...args: any[]) => Promise<any>;

interface BasicUserInfo {
  displayName?: string;
  username?: string;
  email?: string;
  allowedScopes?: string | string[];
}

/**
 * Generate a UUID for token binding
 * @returns UUID string
 */
const generateUUID = (): string => {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === 'x' ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
};

/**
 * Create state parameter object and encode it
 * @param fidpId - Federated IDP identifier
 * @param returnToUrl - URL to return to after login
 * @returns Base64 encoded state parameter
 */
const createStateParam = (fidpId: string, returnToUrl: string = '/'): string => {
  const stateParamObj = {
    fidpId,
    returnToUrl,
  };
  return btoa(JSON.stringify(stateParamObj));
};

/**
 * Perform sign in with the specified federated IDP
 * @param signIn - The signIn function from useAuthContext
 * @param fidp - Federated IDP identifier
 * @param returnToUrl - URL to return to after login
 */
export const handleSignIn = async (
  signIn: SignInFunction,
  fidp: string,
  returnToUrl: string = '/',
  username?: string
): Promise<void> => {
  try {
    const encodedStateParam = createStateParam(fidp, returnToUrl);

    const config: Record<string, string> = {
      fidp,
      state: encodedStateParam,
    };

    if (username) {
      config.username = username;
    }

    const tokenRequestConfig = {
      params: {
        tokenBindingId: generateUUID(),
      },
    };

    // This allows cross-tab session restoration via useSignInSilent
    setFidpId(fidp);

    await signIn(config, undefined, undefined, undefined, undefined, tokenRequestConfig);
  } catch (error) {
    logger.error('Sign in error:', error);
    throw error;
  }
};

/**
 * Handle Google login
 * @param signIn - The signIn function from useAuthContext
 * @param returnToUrl - URL to return to after login
 */
export const handleGoogleLogin = (signIn: SignInFunction, returnToUrl: string = '/'): void => {
  handleSignIn(signIn, FIDP_GOOGLE, returnToUrl);
};

/**
 * Handle GitHub login
 * @param signIn - The signIn function from useAuthContext
 * @param returnToUrl - URL to return to after login
 */
export const handleGithubLogin = (signIn: SignInFunction, returnToUrl: string = '/'): void => {
  handleSignIn(signIn, FIDP_GITHUB, returnToUrl);
};

/**
 * Handle Microsoft login
 * @param signIn - The signIn function from useAuthContext
 * @param returnToUrl - URL to return to after login
 */
export const handleMicrosoftLogin = (signIn: SignInFunction, returnToUrl: string = '/'): void => {
  handleSignIn(signIn, FIDP_MICROSOFT, returnToUrl);
};

/**
 * Handle Enterprise login
 * @param signIn - The signIn function from useAuthContext
 * @param returnToUrl - URL to return to after login
 * @param username - Enterprise user email address
 */
export const handleEnterpriseLogin = (signIn: SignInFunction, returnToUrl: string = '/', username?: string): void => {
  handleSignIn(signIn, FIDP_ENTERPRISE, returnToUrl, username);
};

/**
 * Store user info in localStorage after successful authentication
 * @param userInfo - User information from Asgardeo
 */
export const storeUserInfo = (userInfo: BasicUserInfo | null): void => {
  if (userInfo) {
    // Handle allowedScopes being either string or string[]
    let scopes: string[] = [];
    if (userInfo.allowedScopes) {
      scopes = typeof userInfo.allowedScopes === 'string' 
        ? userInfo.allowedScopes.split(' ') 
        : userInfo.allowedScopes;
    }
  }
};
