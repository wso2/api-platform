import { vi } from 'vitest';

import type { AuthState, AuthStatus } from '../features/auth/authTypes';

/**
 * Builds a complete `AuthState` for tests, with every callback as a `vi.fn()`
 * so component/route tests can render via `AuthStateContext.Provider` without
 * touching the real Asgardeo SDK. Defaults to an authenticated user.
 *
 * `signInSilently` resolves `true` by default; override per test as needed.
 */
export function makeAuthState(overrides: Partial<AuthState> = {}): AuthState {
  const status: AuthStatus = overrides.status ?? 'authenticated';
  const isAuthenticated = overrides.isAuthenticated ?? status === 'authenticated';
  return {
    mode: 'asgardeo',
    status,
    isLoading: status === 'loading',
    isAuthenticated,
    user: isAuthenticated
      ? { name: 'Test User', email: 'test.user@example.com' }
      : undefined,
    token: isAuthenticated ? 'test-access-token' : undefined,
    loginProviders: [],
    login: vi.fn(),
    loginWithProvider: vi.fn(),
    exchangeOrgToken: vi.fn().mockResolvedValue(true),
    signInSilently: vi.fn().mockResolvedValue(true),
    completeLoginFromRedirect: vi.fn(() => ''),
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
