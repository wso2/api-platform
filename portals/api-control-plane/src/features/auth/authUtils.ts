import { runtimeConfig } from '../../config/runtime';
import {
  AUTH_RETURN_TO_STORAGE_KEY,
  AUTH_SESSION_STORAGE_KEY,
  DEFAULT_SESSION_DURATION_MS,
  SESSION_LOAD_TIMEOUT_MS,
} from './authConstants';
import type {
  AuthStatus,
  AuthUser,
  LocalFileAuthSession,
  LoginProvider,
  StoredAuthSession,
} from './authTypes';

export const getConfiguredLoginProviders = (): LoginProvider[] =>
  [
    runtimeConfig.enableEmailLogin ||
    runtimeConfig.enableLocalAuthFallback ||
    runtimeConfig.authMode === 'local-file'
      ? { id: runtimeConfig.fidpEmail, label: 'Email' }
      : undefined,
    runtimeConfig.fidpGoogle
      ? { id: runtimeConfig.fidpGoogle, label: 'Google' }
      : undefined,
    runtimeConfig.fidpGithub
      ? { id: runtimeConfig.fidpGithub, label: 'GitHub' }
      : undefined,
    runtimeConfig.enableMicrosoftLogin && runtimeConfig.fidpMicrosoft
      ? { id: runtimeConfig.fidpMicrosoft, label: 'Microsoft' }
      : undefined,
    !runtimeConfig.disableEnterpriseLogin && runtimeConfig.fidpEnterprise
      ? { id: runtimeConfig.fidpEnterprise, label: 'Enterprise Login' }
      : undefined,
  ].filter(Boolean) as LoginProvider[];

export const buildEncodedState = (fidp: string, returnTo = '/') =>
  btoa(
    JSON.stringify({
      fidpId: fidp,
      returnToUrl: returnTo,
      returnToOrg: '',
    })
  );

export const generateTokenBindingId = () =>
  crypto.randomUUID?.() || `${Date.now()}-${Math.random().toString(36).slice(2)}`;

export const withTimeout = async <T,>(
  promise: Promise<T>,
  message: string,
  timeoutMs = SESSION_LOAD_TIMEOUT_MS
) => {
  let timeoutId: ReturnType<typeof setTimeout> | undefined;
  const timeout = new Promise<never>((_, reject) => {
    timeoutId = setTimeout(() => reject(new Error(message)), timeoutMs);
  });

  try {
    return await Promise.race([promise, timeout]);
  } finally {
    if (timeoutId) clearTimeout(timeoutId);
  }
};

export const normalizeUser = (basicUserInfo?: Record<string, unknown>): AuthUser => {
  const email =
    String(
      basicUserInfo?.email ||
        basicUserInfo?.username ||
        basicUserInfo?.sub ||
        'user@wso2.com'
    ) || 'user@wso2.com';
  const name =
    String(
      basicUserInfo?.displayName ||
        basicUserInfo?.name ||
        basicUserInfo?.username ||
        email
    ) || email;

  return { email, name };
};

export const makeLocalSession = (
  token: string,
  user: AuthUser = {
    name: 'Oxygen Developer',
    email: 'developer@wso2.com',
  },
  expiresAt = Date.now() + DEFAULT_SESSION_DURATION_MS
): StoredAuthSession => ({
  token,
  expiresAt,
  user,
});

export const normalizeLocalFileSession = (
  value: LocalFileAuthSession
): StoredAuthSession =>
  makeLocalSession(
    value.token || value.accessToken || 'local-development-token',
    {
      email: value.user?.email || value.email || 'developer@wso2.com',
      name: value.user?.name || value.name || value.email || 'Oxygen Developer',
    },
    value.expiresAt || Date.now() + DEFAULT_SESSION_DURATION_MS
  );

export const readStoredSession = (): {
  session: StoredAuthSession | null;
  status: AuthStatus;
} => {
  const raw = localStorage.getItem(AUTH_SESSION_STORAGE_KEY);
  if (!raw) return { session: null, status: 'unauthenticated' };
  try {
    const session = JSON.parse(raw) as StoredAuthSession;
    if (!session.expiresAt || session.expiresAt <= Date.now()) {
      localStorage.removeItem(AUTH_SESSION_STORAGE_KEY);
      return { session: null, status: 'expired' };
    }
    return { session, status: 'authenticated' };
  } catch {
    localStorage.removeItem(AUTH_SESSION_STORAGE_KEY);
    return { session: null, status: 'unauthenticated' };
  }
};

export const persistSession = (session: StoredAuthSession) => {
  localStorage.setItem(AUTH_SESSION_STORAGE_KEY, JSON.stringify(session));
};

export const clearStoredAuth = () => {
  localStorage.removeItem(AUTH_SESSION_STORAGE_KEY);
  sessionStorage.removeItem(AUTH_RETURN_TO_STORAGE_KEY);
};

export const readRedirectParams = () => {
  const query = new URLSearchParams(window.location.search);
  const hash = new URLSearchParams(window.location.hash.replace(/^#/, ''));
  return {
    code: query.get('code'),
    error: query.get('error') || hash.get('error'),
    state: query.get('state') || hash.get('state'),
    token: query.get('token') || hash.get('access_token') || hash.get('token'),
  };
};
