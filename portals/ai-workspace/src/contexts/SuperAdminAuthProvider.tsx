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

import React, { useMemo, useCallback } from 'react';
import { AppAuthContext } from './AppAuthContext';
import { clearStoredToken } from '../clients/choreoApiClient';

export const SUPER_ADMIN_SESSION_KEY = 'super_admin_session';

export function isSuperAdminSession(): boolean {
  return sessionStorage.getItem(SUPER_ADMIN_SESSION_KEY) === 'true';
}

export function clearSuperAdminSession() {
  sessionStorage.removeItem(SUPER_ADMIN_SESSION_KEY);
  clearStoredToken();
}

interface Props {
  children: React.ReactNode;
  onLogout: () => void;
}

export function SuperAdminAuthProvider({ children, onLogout }: Props) {
  const logout = useCallback(async () => {
    clearSuperAdminSession();
    onLogout();
  }, [onLogout]);

  const value = useMemo(
    () => ({
      isAuthenticated: true,
      isLoading: false,
      isSuperAdmin: true,
      user: {
        name: 'Super Admin',
        email: '',
        role: 'admin' as const,
        scopes: [],
        org: null,
      },
      accessToken: null,
      hasPermission: () => true,
      login: async () => {},
      logout,
    }),
    [logout]
  );

  return <AppAuthContext.Provider value={value}>{children}</AppAuthContext.Provider>;
}
