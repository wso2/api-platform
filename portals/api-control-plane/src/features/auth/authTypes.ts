export type AuthMode = 'basic' | 'oidc';

export type AuthUser = {
  name: string;
  email: string;
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
  /** oidc mode: redirects to the BFF, which performs the whole flow server-side. */
  login: (returnTo?: string) => void;
  /** basic mode: posts credentials to the BFF. Resolves false on invalid credentials. */
  loginWithCredentials: (
    username: string,
    password: string,
    returnTo?: string
  ) => Promise<boolean>;
  exchangeOrgToken: (orgHandle: string) => Promise<boolean>;
  logout: () => void;
};
