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

import type { AuthMode } from '../../config/authContract';

export type { AuthMode };

export type AuthOrg = {
  id: string;
  name: string;
  handle: string;
};

export type AuthUser = {
  name: string;
  email: string;
  /**
   * Resolved from the session's org claims: server-side by the BFF (see
   * `bff/internal/session/claims.go`'s `UserFromClaims`), or, behind a gateway
   * or auth proxy, from the claims its session endpoint returns (see
   * `userFromClaims.ts`). Never decoded from a token in the browser, which
   * never holds one.
   */
  org?: AuthOrg | null;
  /** Avatar URL from the IDP's picture claim; the header falls back to
   * initials when the IDP supplies none. */
  picture?: string;
  scopes?: string[],
  role?: string,
};

export type AuthStatus =
  | 'loading'
  | 'authenticated'
  | 'unauthenticated'
  | 'expired';

export type AuthState = {
  error?: string;
  mode: AuthMode;
  status: AuthStatus;
  isLoading: boolean;
  isAuthenticated: boolean;
  user?: AuthUser;
  /**
   * Redirect sign-in (`oidc`, `external`): sends the browser to the sign-in
   * endpoint, which runs the whole flow server-side. `returnTo` is a path
   * inside the app; it defaults to the current page.
   */
  login: (returnTo?: string) => void;
  /** Form sign-in (`basic`): posts credentials. Resolves false on invalid credentials. */
  loginWithCredentials: (
    username: string,
    password: string,
    returnTo?: string
  ) => Promise<boolean>;
  exchangeOrgToken: (orgHandle: string) => Promise<boolean>;
  logout: () => void;
};
