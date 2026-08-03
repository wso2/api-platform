import {
  ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { AsgardeoSPAClient, Hooks, useAuthContext } from '@asgardeo/auth-react';

import {
  setApiAccessToken,
  setApiHttpRequest,
  setPlatformTokenProvider,
  setPlatformTokenRefresher,
} from '../../../api/client';
import { runtimeConfig } from '../../../config/runtime';
import {
  AUTH_RETURN_TO_STORAGE_KEY,
  AUTH_SESSION_HINT_KEY,
  SESSION_LOAD_TIMEOUT_MS,
  SILENT_SIGN_IN_TIMEOUT_MS,
} from '../authConstants';
import { AuthStateContext } from '../AuthStateContext';
import type { AuthState, AuthUser } from '../authTypes';
import {
  buildEncodedState,
  generateTokenBindingId,
  getConfiguredLoginProviders,
  normalizeUser,
  withTimeout,
} from '../authUtils';

type SignInFn = (...args: unknown[]) => unknown;
type SignOutFn = (...args: unknown[]) => unknown;
type TrySignInSilentlyFn = (...args: unknown[]) => Promise<unknown>;
type AccessTokenFn = (...args: unknown[]) => Promise<string>;
type BasicUserInfoFn = (...args: unknown[]) => Promise<Record<string, unknown>>;
type DecodedIdTokenFn = (
  ...args: unknown[]
) => Promise<Record<string, unknown>>;
type UpdateConfigFn = (config: Record<string, unknown>) => Promise<void>;
type RequestCustomGrantFn = (config: Record<string, unknown>) => void;
type RegisterHookFn = (
  hook: string,
  callback: () => Promise<void> | void,
  id?: string
) => void;

export function AsgardeoAuthAdapter({ children }: { children: ReactNode }) {
  const {
    state,
    signIn,
    signOut,
    trySignInSilently,
    getAccessToken,
    getBasicUserInfo,
    getDecodedIDToken,
    on,
    requestCustomGrant,
    updateConfig,
  } = useAuthContext() as ReturnType<typeof useAuthContext> & {
    trySignInSilently?: TrySignInSilentlyFn;
    getDecodedIDToken?: DecodedIdTokenFn;
    on?: RegisterHookFn;
    requestCustomGrant?: RequestCustomGrantFn;
    updateConfig?: UpdateConfigFn;
  };
  // Holds the resolver for the in-flight org token exchange. The CustomGrant
  // hook is registered once (below) and resolves whichever exchange is pending,
  // so handlers are not re-registered on every org switch.
  const customGrantResolverRef = useRef<(() => void) | null>(null);
  // The Asgardeo SDK does not guarantee referentially-stable function identities
  // across renders. Holding them in refs lets the session/token effects depend
  // only on stable primitives (isAuthenticated) instead of these functions,
  // which would otherwise re-fire the effects every render → recursive token
  // calls.
  const getAccessTokenRef = useRef(getAccessToken);
  getAccessTokenRef.current = getAccessToken;
  const getBasicUserInfoRef = useRef(getBasicUserInfo);
  getBasicUserInfoRef.current = getBasicUserInfo;
  const [token, setToken] = useState<string>();
  const [user, setUser] = useState<AuthUser>();
  const [loginError, setLoginError] = useState<string>();
  const [isSessionLoading, setIsSessionLoading] = useState(false);
  const [sessionLoadFailed, setSessionLoadFailed] = useState(false);
  const isAuthenticated = Boolean(state?.isAuthenticated);
  const isSdkLoading = Boolean(state?.isLoading) && !isAuthenticated;
  const loginProviders = useMemo(getConfiguredLoginProviders, []);

  useEffect(() => {
    let isMounted = true;

    const loadSession = async () => {
      if (!isAuthenticated) {
        setApiAccessToken(undefined);
        setApiHttpRequest(undefined);
        setPlatformTokenProvider(undefined);
        setPlatformTokenRefresher(undefined);
        setToken(undefined);
        setUser(undefined);
        setIsSessionLoading(false);
        setSessionLoadFailed(false);
        return;
      }

      setIsSessionLoading(true);
      setSessionLoadFailed(false);
      const accessToken = await withTimeout(
        (getAccessTokenRef.current as AccessTokenFn)(),
        'Timed out while loading access token',
        SESSION_LOAD_TIMEOUT_MS
      );
      const basicUserInfo = await withTimeout(
        (getBasicUserInfoRef.current as BasicUserInfoFn)(),
        'Timed out while loading user profile',
        SESSION_LOAD_TIMEOUT_MS
      );

      if (!isMounted) return;

      setApiAccessToken(accessToken);
      setApiHttpRequest(async (config) => {
        const response =
          await AsgardeoSPAClient.getInstance()?.httpRequest(config);
        if (!response) throw new Error('Asgardeo HTTP client is not available');
        return response;
      });
      // Platform/BML (REST) token seam: provide the current token and a
      // refresher so platformClient can recover from a 401 without coupling to
      // the Asgardeo SDK.
      setPlatformTokenProvider(() =>
        (getAccessTokenRef.current as AccessTokenFn)()
      );
      setPlatformTokenRefresher(async () => {
        await AsgardeoSPAClient.getInstance()?.refreshAccessToken();
        return (getAccessTokenRef.current as AccessTokenFn)();
      });
      setToken(accessToken);
      setUser(normalizeUser(basicUserInfo));
      // Remember that a session was established in this browser, so a future
      // load knows a silent restoration attempt is worthwhile.
      localStorage.setItem(AUTH_SESSION_HINT_KEY, '1');
      setIsSessionLoading(false);
    };

    loadSession().catch((error) => {
      if (!isMounted) return;
      // eslint-disable-next-line no-console
      console.error('Failed to initialize API Platform session', error);
      setApiAccessToken(undefined);
      setApiHttpRequest(undefined);
      setPlatformTokenProvider(undefined);
      setPlatformTokenRefresher(undefined);
      setToken(undefined);
      setUser(undefined);
      setIsSessionLoading(false);
      setSessionLoadFailed(true);
    });

    return () => {
      isMounted = false;
    };
    // Depends only on the stable `isAuthenticated` primitive; the SDK functions
    // are read through refs to avoid re-running (and re-fetching tokens) on
    // every render when their identities change.
  }, [isAuthenticated]);

  // Register the CustomGrant completion hook exactly once (guarded by a ref so a
  // churning `on` identity cannot re-register it). It resolves whatever org
  // token exchange is currently in flight.
  const customGrantHookRegisteredRef = useRef(false);
  useEffect(() => {
    if (!on || customGrantHookRegisteredRef.current) return;
    customGrantHookRegisteredRef.current = true;
    on(
      Hooks.CustomGrant,
      () => {
        customGrantResolverRef.current?.();
      },
      runtimeConfig.tokenExchangeConfig?.id as string | undefined
    );
  }, [on]);

  const exchangeOrgToken = useCallback(
    async (orgHandle: string) => {
      if (!orgHandle || !isAuthenticated) return false;
      // Platform-API/BML mode: BML performs the org token exchange itself, and
      // expects the RAW Asgardeo access token. Skip the console-side STS custom
      // grant so getAccessToken() keeps returning the IdP token.
      if (runtimeConfig.platformApiBaseUrl) return true;
      // No token-exchange configured (e.g. local/STS-less setups): the base user
      // token already carries the needed scope, so there is nothing to exchange.
      if (
        !runtimeConfig.tokenExchangeConfig ||
        !on ||
        !requestCustomGrant ||
        !updateConfig
      ) {
        return true;
      }

      // The exchanged STS token has a different issuer/audience than the IdP ID
      // token, so the SDK's ID-token validation must be disabled for the custom
      // grant. The STS token is still validated server-side by the resource APIs.
      await updateConfig({ validateIDToken: false });
      if (getDecodedIDToken) {
        try {
          await getDecodedIDToken();
        } catch (error) {
          // eslint-disable-next-line no-console
          console.warn(
            'Unable to decode ID token before org token exchange',
            error
          );
        }
      }

      await new Promise<void>((resolve, reject) => {
        const timeout = window.setTimeout(() => {
          customGrantResolverRef.current = null;
          reject(new Error('Timed out while exchanging organization token'));
        }, SESSION_LOAD_TIMEOUT_MS);
        customGrantResolverRef.current = () => {
          window.clearTimeout(timeout);
          customGrantResolverRef.current = null;
          resolve();
        };
        try {
          requestCustomGrant({
            ...runtimeConfig.tokenExchangeConfig,
            data: {
              ...((runtimeConfig.tokenExchangeConfig?.data as
                | object
                | undefined) || {}),
              orgHandle,
            },
          });
        } catch (error) {
          window.clearTimeout(timeout);
          customGrantResolverRef.current = null;
          reject(error);
        }
      });

      const exchangedToken = await withTimeout(
        (getAccessToken as AccessTokenFn)(),
        'Timed out while loading exchanged access token',
        SESSION_LOAD_TIMEOUT_MS
      );
      setApiAccessToken(exchangedToken);
      setToken(exchangedToken);
      return true;
    },
    [
      getAccessToken,
      getDecodedIDToken,
      isAuthenticated,
      on,
      requestCustomGrant,
      updateConfig,
    ]
  );

  const signInSilently = useCallback(async () => {
    if (!trySignInSilently) return false;
    // Nothing to restore for a first-time visitor — skip the prompt=none iframe
    // and let the route fall through to /login immediately.
    if (localStorage.getItem(AUTH_SESSION_HINT_KEY) !== '1') return false;
    try {
      const result = await withTimeout(
        Promise.resolve(trySignInSilently()),
        'Timed out during silent sign-in',
        SILENT_SIGN_IN_TIMEOUT_MS
      );
      return Boolean(result);
    } catch (error) {
      // eslint-disable-next-line no-console
      console.warn('Silent sign-in attempt failed', error);
      return false;
    }
  }, [trySignInSilently]);

  const loginWithProvider = useCallback(
    (fidp: string, returnTo = '/', username?: string) => {
      setLoginError(undefined);
      sessionStorage.setItem(AUTH_RETURN_TO_STORAGE_KEY, returnTo);
      const tokenRequestConfig = {
        params: {
          tokenBindingId: generateTokenBindingId(),
        },
      };
      const signInConfig: Record<string, string> = {
        fidp,
        state: buildEncodedState(fidp, returnTo),
      };
      if (username) {
        signInConfig.username = username;
      }

      const signInResult = (signIn as SignInFn)(
        signInConfig,
        undefined,
        undefined,
        undefined,
        undefined,
        tokenRequestConfig
      );
      if (
        signInResult &&
        typeof (signInResult as Promise<unknown>).catch === 'function'
      ) {
        (signInResult as Promise<unknown>).catch((error) => {
          // eslint-disable-next-line no-console
          console.error('Failed to start API Platform sign in', error);
          setLoginError('Unable to start sign in. Please try again.');
        });
      }
    },
    [signIn]
  );

  const login = useCallback(
    (returnTo = '/') => loginWithProvider(runtimeConfig.fidpEmail, returnTo),
    [loginWithProvider]
  );

  const completeLoginFromRedirect = useCallback(() => {
    const returnTo = sessionStorage.getItem(AUTH_RETURN_TO_STORAGE_KEY) || '/';
    sessionStorage.removeItem(AUTH_RETURN_TO_STORAGE_KEY);
    return returnTo;
  }, []);

  const logout = useCallback(() => {
    sessionStorage.removeItem(AUTH_RETURN_TO_STORAGE_KEY);
    localStorage.removeItem(AUTH_SESSION_HINT_KEY);
    setApiAccessToken(undefined);
    setApiHttpRequest(undefined);
    setPlatformTokenProvider(undefined);
    setPlatformTokenRefresher(undefined);
    setToken(undefined);
    setUser(undefined);
    (signOut as SignOutFn)();
  }, [signOut]);

  const isLoading = isSdkLoading || (isAuthenticated && isSessionLoading);
  const hasReadySession = isAuthenticated && !!token;
  const status = isLoading
    ? 'loading'
    : sessionLoadFailed
      ? 'unauthenticated'
      : hasReadySession
        ? 'authenticated'
        : 'unauthenticated';

  const value = useMemo<AuthState>(
    () => ({
      mode: 'asgardeo',
      error: loginError,
      status,
      isLoading,
      isAuthenticated: hasReadySession,
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
      hasReadySession,
      isLoading,
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
