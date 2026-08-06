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
  forbidden: () =>
    makeAuthState({ status: 'forbidden', isAuthenticated: false }),
};
