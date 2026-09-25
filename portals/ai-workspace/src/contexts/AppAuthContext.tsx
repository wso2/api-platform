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

import { createContext, useContext } from 'react';
import type { PlatformRole } from '../auth/permissions';

export interface AppOrg {
  id: string;
  name: string;
  handle: string;
}

export interface AppUser {
  name: string | null;
  email: string | null;
  role: PlatformRole | null;
  scopes: string[];
  org: AppOrg | null;
  organizations: string[];
}

export interface AppAuthContextType {
  isAuthenticated: boolean;
  isLoading: boolean;
  // True when the BFF answered GET /api/session with 502: the session cookie is
  // valid and the session is alive, but the BFF cannot currently mint the token it
  // forwards upstream (the IDP is unreachable, or the exchange is misconfigured).
  //
  // Distinct from `!isAuthenticated`, and the distinction is the point: treating
  // this as "logged out" sends the user to /login over a transient blip, and the
  // login they then attempt fails identically, because the OIDC callback runs the
  // very same exchange. Route guards must hold the route, not redirect.
  sessionUnavailable: boolean;
  // Re-reads the session from the BFF and updates `user` from it. Awaitable, because
  // both callers need to know the context has caught up before they continue:
  //
  //  - the sessionUnavailable retry, which the provider also drives on a timer;
  //  - an org switch, after which the exchanged token — and therefore the caller's
  //    scopes AND which org they are in — has changed server-side. Re-reading the
  //    session is what brings those into the UI; the switch response alone reports
  //    scopes, and would leave the org stale.
  refreshSession: () => Promise<void>;
  user: AppUser | null;
  // Fetches the current raw JWT on demand. Unlike a cached snapshot, this stays
  // correct after the BFF proxy rotates the cookie token, so call-sites that
  // need the raw token always get the live value.
  getAccessToken: () => Promise<string | null>;
  hasPermission: (scope: string) => boolean;
  login: () => Promise<void>;
  logout: () => Promise<void>;
}

export const AppAuthContext = createContext<AppAuthContextType>({
  isAuthenticated: false,
  isLoading: true,
  sessionUnavailable: false,
  refreshSession: async () => {},
  user: null,
  getAccessToken: async () => null,
  hasPermission: () => false,
  login: async () => {},
  logout: async () => {},
});

export function useAppAuth(): AppAuthContextType {
  return useContext(AppAuthContext);
}
