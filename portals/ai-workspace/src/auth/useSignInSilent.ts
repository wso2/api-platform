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

import { useAuthContext } from '@asgardeo/auth-react';
import { useNavigate, useLocation } from 'react-router-dom';
import { getFidpId } from '../utils/cookies';
import { logger } from '../utils/logger';

// State param keys for preserving return URL
const RETURN_TO_URL = 'returnToUrl';

// Session storage key for storing the original URL before login
// URL to return to after login
const RETURN_URL_SESSION_KEY = 'ai_workspace_return_url';

/**
 * Hook to handle silent sign-in for restoring sessions across browser tabs.
 *
 * When using sessionStorage, each tab has its own storage. This hook:
 * 1. Tries `trySignInSilently()` to restore session via Asgardeo cookies
 * 2. If that fails, falls back to `signIn()` with stored fidpId to trigger
 *    a full OAuth flow (which generates new PKCE verifier for this tab)
 *
 * The fidpId is stored in a cookie (shared across tabs) so new tabs know
 * which identity provider the user originally signed in with.
 */
export const useSignInSilent = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const { trySignInSilently, signIn } = useAuthContext();

  const returnUrl = `${location.pathname}${location.search}`;

  /**
   * Store the return URL in sessionStorage so it persists through OAuth redirect
   */
  const storeReturnUrl = () => {
    if (returnUrl && returnUrl !== '/' && returnUrl !== '/login' && returnUrl !== '/signin') {
      sessionStorage.setItem(RETURN_URL_SESSION_KEY, returnUrl);
    }
  };

  /**
   * Redirect to login page with return URL preserved
   */
  const redirectToLogin = () => {
    storeReturnUrl();
    navigate('/login', {
      state: {
        returnToUrl: returnUrl,
      },
      replace: true,
    });
  };

  /**
   * Attempt silent sign-in, falling back to full sign-in if needed.
   *
   * Flow:
   * 1. Try `trySignInSilently()` - uses iframe to check Asgardeo session
   * 2. If fails (returns false), get stored fidpId from cookie
   * 3. If fidpId exists, call `signIn({fidp})` - redirects to Asgardeo
   * 4. User has active SSO session on Asgardeo → auto-completes OAuth
   * 5. Returns with auth code → new PKCE verifier for this tab
   *
   * This solves the PKCE + sessionStorage cross-tab issue because each tab
   * gets its own PKCE verifier through the full OAuth redirect flow.
   */
  const sleep = (ms: number) => new Promise((res) => setTimeout(res, ms));

  const generateUUID = (): string => {
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
      const r = (Math.random() * 16) | 0;
      const v = c === 'x' ? r : (r & 0x3) | 0x8;
      return v.toString(16);
    });
  };

  const signInSilent = async (): Promise<void> => {
    const MAX_RETRIES = 3;
    const RETRY_DELAY_MS = 300;

    const tokenRequestConfig = {
      tokenBindingId: generateUUID(),
    } as Record<string, string | boolean>;

    try {
      let success = false;
      let lastError: any = null;

      for (let attempt = 1; attempt <= MAX_RETRIES; attempt++) {
        try {
          const response = await trySignInSilently(tokenRequestConfig);
          if (response) {
            success = true;
            break;
          }
        } catch (err) {
          // Keep the error and retry a few times
          lastError = err;
        }

        if (attempt < MAX_RETRIES) {
          await sleep(RETRY_DELAY_MS);
        }
      }

      if (!success) {
        // Silent sign-in ultimately failed. Fallback to full sign-in if we have fidpId
        const fidpId = getFidpId();

        if (fidpId) {

          // Store return URL before OAuth redirect
          storeReturnUrl();

          const stateParam: Record<string, string> = {};
          stateParam[RETURN_TO_URL] = returnUrl;
          const encodedStateParam = btoa(JSON.stringify(stateParam));

          try {
            await signIn({
              fidp: fidpId,
              state: encodedStateParam,
            });
          } catch {
            redirectToLogin();
          }
        } else {
          // No fidpId stored —  redirect to login page
          redirectToLogin();
        }
      }
    } catch {
      redirectToLogin();
    }
  };

  return { signInSilent, redirectToLogin };
};

export default useSignInSilent;
