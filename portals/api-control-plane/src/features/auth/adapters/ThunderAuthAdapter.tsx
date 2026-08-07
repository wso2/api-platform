import { ReactNode, useCallback, useEffect, useMemo, useState } from 'react';
import {
  User,
  UserManager,
  type UserManagerSettings,
  WebStorageStateStore,
} from 'oidc-client-ts';

import {
  setApiAccessToken,
  setApiHttpRequest,
  setPlatformTokenProvider,
  setPlatformTokenRefresher,
} from '../../../api/client';
import { runtimeConfig } from '../../../config/runtime';
import { AUTH_RETURN_TO_STORAGE_KEY, AUTH_SESSION_HINT_KEY } from '../authConstants';
import { AuthStateContext } from '../AuthStateContext';
import type { AuthState, AuthStatus, AuthUser } from '../authTypes';
import { getConfiguredLoginProviders } from '../authUtils';

const DEFAULT_SCOPE = 'openid profile email';

const trimTrailingSlash = (value: string) => value.replace(/\/$/, '');

const redirectUrl = (path: string) =>
  `${window.location.origin}${runtimeConfig.appBasePath || ''}${path}`;

/**
 * Builds the oidc-client-ts settings from runtime config. Thunder's OIDC
 * endpoints follow the `/oauth2/*` layout under `authBaseUrl`; explicit
 * overrides in `runtimeConfig.thunder` win. Discovery is bypassed (Thunder's
 * issuer is a bare name, not a URL) by supplying an explicit `metadata` block.
 */
const buildSettings = (): UserManagerSettings | undefined => {
  const base = trimTrailingSlash(runtimeConfig.authBaseUrl);
  if (!base || !runtimeConfig.authClientId) return undefined;
  const t = runtimeConfig.thunder ?? {};
  const scope = runtimeConfig.authScopes.join(' ') || DEFAULT_SCOPE;

  return {
    authority: base,
    metadata: {
      issuer: t.issuer || 'platform_idp',
      authorization_endpoint: t.authorizationEndpoint || `${base}/authorize`,
      token_endpoint: t.tokenEndpoint || `${base}/token`,
      userinfo_endpoint: t.userinfoEndpoint || `${base}/userinfo`,
      jwks_uri: t.jwksUri || `${base}/jwks`,
      ...(t.endSessionEndpoint
        ? { end_session_endpoint: t.endSessionEndpoint }
        : {}),
    },
    client_id: runtimeConfig.authClientId,
    redirect_uri: redirectUrl('/signin'),
    post_logout_redirect_uri: redirectUrl('/login'),
    response_type: 'code',
    scope,
    loadUserInfo: false,
    automaticSilentRenew: false,
    userStore: new WebStorageStateStore({ store: window.localStorage }),
  };
};

const toUser = (user: User): AuthUser => {
  const profile = user.profile ?? {};
  const given = (profile.given_name as string | undefined) ?? '';
  const family = (profile.family_name as string | undefined) ?? '';
  const email = (profile.email as string | undefined) ?? '';
  const name =
    (profile.name as string | undefined) ||
    `${given} ${family}`.trim() ||
    email;
  return { name, email };
};

export function ThunderAuthAdapter({ children }: { children: ReactNode }) {
  const settings = useMemo(buildSettings, []);
  const manager = useMemo(
    () => (settings ? new UserManager(settings) : undefined),
    [settings]
  );
  // Capture the redirect-callback URL synchronously on first render, BEFORE the
  // AuthCallbackPage navigates away (which would strip the ?code). The token
  // exchange below runs against this captured URL.
  const [callbackUrl] = useState(() => {
    const params = new URLSearchParams(window.location.search);
    return params.has('code') && params.has('state')
      ? window.location.href
      : undefined;
  });
  const [token, setToken] = useState<string>();
  const [user, setUser] = useState<AuthUser>();
  const [status, setStatus] = useState<AuthStatus>(
    manager ? 'loading' : 'unauthenticated'
  );
  const [loginError, setLoginError] = useState<string>();
  const loginProviders = useMemo(getConfiguredLoginProviders, []);

  const clearSession = useCallback(() => {
    setApiAccessToken(undefined);
    setApiHttpRequest(undefined);
    setPlatformTokenProvider(undefined);
    setPlatformTokenRefresher(undefined);
    setToken(undefined);
    setUser(undefined);
  }, []);

  const activate = useCallback(
    (oidcUser: User) => {
      const accessToken = oidcUser.access_token;
      setApiAccessToken(accessToken);
      setApiHttpRequest(undefined);
      // Platform/BML (REST) token seam. The provider reads the latest stored
      // token; the refresher performs a silent renew and returns the new token.
      setPlatformTokenProvider(async () => {
        const current = await manager?.getUser();
        return current?.access_token;
      });
      setPlatformTokenRefresher(async () => {
        const renewed = await manager?.signinSilent();
        return renewed?.access_token;
      });
      setToken(accessToken);
      setUser(toUser(oidcUser));
      localStorage.setItem(AUTH_SESSION_HINT_KEY, '1');
      setStatus('authenticated');
    },
    [manager]
  );

  useEffect(() => {
    if (!manager) return;
    let isMounted = true;

    const init = async () => {
      if (callbackUrl) {
        const signedIn = await manager.signinRedirectCallback(callbackUrl);
        if (isMounted) activate(signedIn);
        return;
      }
      const existing = await manager.getUser();
      if (!isMounted) return;
      if (existing && !existing.expired) {
        activate(existing);
      } else {
        setStatus('unauthenticated');
      }
    };

    init().catch((error) => {
      if (!isMounted) return;
      // eslint-disable-next-line no-console
      console.error('Thunder sign-in failed', error);
      clearSession();
      setStatus('unauthenticated');
      if (callbackUrl) setLoginError('Unable to complete sign in.');
    });

    return () => {
      isMounted = false;
    };
  }, [manager, callbackUrl, activate, clearSession]);

  const loginWithProvider = useCallback(
    (_fidp: string, returnTo = '/') => {
      if (!manager) return;
      setLoginError(undefined);
      sessionStorage.setItem(AUTH_RETURN_TO_STORAGE_KEY, returnTo);
      manager.signinRedirect({ state: returnTo }).catch((error) => {
        // eslint-disable-next-line no-console
        console.error('Failed to start Thunder sign in', error);
        setLoginError('Unable to start sign in. Please try again.');
      });
    },
    [manager]
  );

  const login = useCallback(
    (returnTo = '/') => loginWithProvider('', returnTo),
    [loginWithProvider]
  );

  const signInSilently = useCallback(async () => {
    if (!manager) return false;
    if (localStorage.getItem(AUTH_SESSION_HINT_KEY) !== '1') return false;
    try {
      const renewed = await manager.signinSilent();
      if (renewed && !renewed.expired) {
        activate(renewed);
        return true;
      }
      return false;
    } catch (error) {
      // eslint-disable-next-line no-console
      console.warn('Thunder silent sign-in failed', error);
      return false;
    }
  }, [manager, activate]);

  // BML owns org-scoping (the raw Thunder token carries ouId), so there is no
  // console-side org token exchange.
  const exchangeOrgToken = useCallback(async () => true, []);

  const completeLoginFromRedirect = useCallback(() => {
    const returnTo =
      sessionStorage.getItem(AUTH_RETURN_TO_STORAGE_KEY) || '/';
    sessionStorage.removeItem(AUTH_RETURN_TO_STORAGE_KEY);
    return returnTo;
  }, []);

  const logout = useCallback(() => {
    sessionStorage.removeItem(AUTH_RETURN_TO_STORAGE_KEY);
    localStorage.removeItem(AUTH_SESSION_HINT_KEY);
    clearSession();
    setStatus('unauthenticated');
    if (!manager) return;
    void manager.removeUser();
    if (runtimeConfig.thunder?.endSessionEndpoint) {
      manager.signoutRedirect().catch((error) => {
        // eslint-disable-next-line no-console
        console.warn('Thunder sign-out redirect failed', error);
      });
    }
  }, [manager, clearSession]);

  const isAuthenticated = status === 'authenticated' && !!token;

  const value = useMemo<AuthState>(
    () => ({
      mode: 'thunder',
      error: loginError,
      status,
      isLoading: status === 'loading',
      isAuthenticated,
      user,
      token,
      loginProviders,
      login,
      loginWithProvider,
      exchangeOrgToken,
      signInSilently,
      completeLoginFromRedirect,
      logout,
    }),
    [
      completeLoginFromRedirect,
      exchangeOrgToken,
      isAuthenticated,
      login,
      loginError,
      loginProviders,
      loginWithProvider,
      logout,
      signInSilently,
      status,
      token,
      user,
    ]
  );

  return (
    <AuthStateContext.Provider value={value}>
      {children}
    </AuthStateContext.Provider>
  );
}
