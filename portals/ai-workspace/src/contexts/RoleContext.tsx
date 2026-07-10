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

import React, { createContext, useContext, useMemo } from 'react';
import { useAppAuth } from './AppAuthContext';
import type { PlatformRole } from '../auth/permissions';

export type UserRole = PlatformRole;

type RoleContextValue = {
  role: UserRole;
  setRole: (role: UserRole) => void;
};

const RoleContext = createContext<RoleContextValue>({
  role: 'viewer',
  setRole: () => {},
});

export function RoleProvider({ children }: { children: React.ReactNode }) {
  const { user } = useAppAuth();
  const role: UserRole = user?.role ?? 'viewer';
  const value = useMemo(() => ({ role, setRole: () => {} }), [role]);
  return <RoleContext.Provider value={value}>{children}</RoleContext.Provider>;
}

export function useRole(): RoleContextValue {
  return useContext(RoleContext);
}
