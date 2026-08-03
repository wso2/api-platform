export type AuthMode = 'asgardeo' | 'local-file' | 'thunder';

export type AuthUser = {
  name: string;
  email: string;
};

export type LoginProvider = {
  id: string;
  label: string;
};

export type AuthStatus =
  | 'loading'
  | 'authenticated'
  | 'unauthenticated'
  | 'expired'
  | 'forbidden';

export type AuthState = {
  error?: string;
  mode: AuthMode;
  status: AuthStatus;
  isLoading: boolean;
  isAuthenticated: boolean;
  user?: AuthUser;
  token?: string;
  loginProviders: LoginProvider[];
  login: (returnTo?: string) => void;
  loginWithProvider: (fidp: string, returnTo?: string, username?: string) => void;
  exchangeOrgToken: (orgHandle: string) => Promise<boolean>;
  /**
   * Attempts to restore an existing session without user interaction (e.g. via
   * an Asgardeo silent iframe sign-in). Resolves `true` when a session is
   * available afterwards, `false` otherwise. Callers should fall back to an
   * interactive login on `false`.
   */
  signInSilently: () => Promise<boolean>;
  completeLoginFromRedirect: () => string;
  logout: () => void;
};

export type StoredAuthSession = {
  token: string;
  user: AuthUser;
  expiresAt: number;
};

export type LocalFileAuthSession = Partial<StoredAuthSession> & {
  accessToken?: string;
  email?: string;
  name?: string;
};
