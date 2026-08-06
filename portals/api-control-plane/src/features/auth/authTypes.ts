export type AuthMode = 'basic' | 'oidc';

export type AuthOrg = {
  id: string;
  name: string;
  handle: string;
};

export type AuthUser = {
  name: string;
  email: string;
  /**
   * Resolved server-side by the BFF from the session token's org claims (see
   * `bff/internal/session/claims.go`'s `UserFromClaims`) — never decoded from
   * a token in the browser, which never holds one.
   */
  org?: AuthOrg | null;
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
