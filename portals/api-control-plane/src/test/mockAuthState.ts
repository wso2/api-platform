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

import { vi } from 'vitest';

import type { AuthState, AuthStatus } from '../features/auth/authTypes';

/**
 * Builds a complete `AuthState` for tests, with every callback as a `vi.fn()`
 * so component/route tests can render via `AuthStateContext.Provider`
 * without a real BFF. Defaults to an authenticated user.
 */
export function makeAuthState(overrides: Partial<AuthState> = {}): AuthState {
  const status: AuthStatus = overrides.status ?? 'authenticated';
  const isAuthenticated = overrides.isAuthenticated ?? status === 'authenticated';
  return {
    mode: 'basic',
    status,
    isLoading: status === 'loading',
    isAuthenticated,
    user: isAuthenticated
      ? { name: 'Test User', email: 'test.user@example.com' }
      : undefined,
    login: vi.fn(),
    loginWithCredentials: vi.fn().mockResolvedValue(true),
    exchangeOrgToken: vi.fn().mockResolvedValue(true),
    logout: vi.fn(),
    ...overrides,
  };
}

/** Named presets mirroring the legacy `MockedUsers` fixtures. */
export const authStatePresets = {
  authenticated: () => makeAuthState({ status: 'authenticated' }),
  unauthenticated: () =>
    makeAuthState({ status: 'unauthenticated', isAuthenticated: false }),
  loading: () => makeAuthState({ status: 'loading', isAuthenticated: false }),
  expired: () =>
    makeAuthState({ status: 'expired', isAuthenticated: false }),
};
