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

export interface AppUser {
  name: string | null;
  email: string | null;
  role: PlatformRole | null;
  scopes: string[];
}

export interface AppAuthContextType {
  isAuthenticated: boolean;
  isLoading: boolean;
  user: AppUser | null;
  accessToken: string | null;
  hasPermission: (scope: string) => boolean;
  login: (credentials?: { username: string; password: string }) => Promise<void>;
  logout: () => Promise<void>;
}

export const AppAuthContext = createContext<AppAuthContextType>({
  isAuthenticated: false,
  isLoading: true,
  user: null,
  accessToken: null,
  hasPermission: () => false,
  login: async () => {},
  logout: async () => {},
});

export function useAppAuth(): AppAuthContextType {
  return useContext(AppAuthContext);
}
