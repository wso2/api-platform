import axios, { AxiosError, type Method } from 'axios';

import { runtimeConfig } from '../config/runtime';
import { ApiError } from './types/errors';

/**
 * Per-request org/project context for the GraphQL/REST (axios) transport. Passed
 * explicitly into `postGraphql`/`getJson` (no hidden global state) and sent as
 * `x-org-handle` / `x-project-handler` headers when present.
 *
 */
export type ApiRequestContext = {
  orgHandle?: string;
  projectHandler?: string;
};

type AuthHttpRequest = (config: {
  data?: unknown;
  headers?: Record<string, string>;
  method: Method;
  timeout?: number;
  url: string;
}) => Promise<{ data?: unknown }>;

const REQUEST_TIMEOUT_MS = 15000;

const contextHeaders = (context?: ApiRequestContext): Record<string, string> => {
  const headers: Record<string, string> = {};
  if (context?.orgHandle) headers['x-org-handle'] = context.orgHandle;
  if (context?.projectHandler) {
    headers['x-project-handler'] = context.projectHandler;
  }
  return headers;
};

// Optional transport override for this legacy GraphQL/REST client (distinct
// from platformClient.ts, which is what actually calls the BFF's same-origin
// proxy). Lets tests substitute a fake HTTP layer without mocking axios
// directly; unset in production, where `apiClient` below is used as-is.
let authHttpRequest: AuthHttpRequest | undefined;

export const apiClient = axios.create({
  baseURL: runtimeConfig.apiBaseUrl,
  timeout: REQUEST_TIMEOUT_MS,
});

export const setApiAccessToken = (token?: string) => {
  if (token) {
    apiClient.defaults.headers.common.Authorization = `Bearer ${token}`;
  } else {
    delete apiClient.defaults.headers.common.Authorization;
  }
};

export const setApiHttpRequest = (httpRequest?: AuthHttpRequest) => {
  authHttpRequest = httpRequest;
};

// Token seam for the platform (BML/REST) client. Auth adapters register a
// provider (the current bearer) and an optional refresher (invoked once on a
// 401). This keeps `platformClient` decoupled from any specific IdP SDK so it
// works identically under Asgardeo, Thunder, or local-file auth.
type PlatformTokenFn = () => Promise<string | undefined>;
let platformTokenProvider: PlatformTokenFn | undefined;
let platformTokenRefresher: PlatformTokenFn | undefined;

export const setPlatformTokenProvider = (provider?: PlatformTokenFn) => {
  platformTokenProvider = provider;
};

export const setPlatformTokenRefresher = (refresher?: PlatformTokenFn) => {
  platformTokenRefresher = refresher;
};

export const getPlatformToken = (): Promise<string | undefined> =>
  platformTokenProvider ? platformTokenProvider() : Promise.resolve(undefined);

/** Returns a fresh token on success, or undefined when no refresher is set. */
export const refreshPlatformToken = (): Promise<string | undefined> =>
  platformTokenRefresher ? platformTokenRefresher() : Promise.resolve(undefined);

export const normalizeApiError = (error: unknown): ApiError => {
  if (error instanceof ApiError) {
    return error;
  }

  if (!axios.isAxiosError(error)) {
    return new ApiError('Unexpected error', 'UNKNOWN');
  }

  const axiosError = error as AxiosError<{ message?: string }>;
  const status = axiosError.response?.status;
  const message =
    axiosError.response?.data?.message ||
    axiosError.message ||
    'Request failed';

  if (status === 401) return new ApiError(message, 'UNAUTHORIZED', status);
  if (status === 403) return new ApiError(message, 'FORBIDDEN', status);
  if (status === 404) return new ApiError(message, 'NOT_FOUND', status);
  if (status && status >= 500) {
    return new ApiError(message, 'SERVER_ERROR', status);
  }
  if (!status) return new ApiError(message, 'NETWORK_ERROR');

  return new ApiError(message, 'UNKNOWN', status);
};

const request = async <T>(config: {
  data?: unknown;
  method: Method;
  url: string;
  headers?: Record<string, string>;
}) => {
  const headers = {
    Accept: 'application/json',
    'Content-Type': 'application/json',
    'x-api-platform-console': 'oxygen',
    ...config.headers,
  };

  if (authHttpRequest) {
    const response = await authHttpRequest({
      ...config,
      headers,
      timeout: REQUEST_TIMEOUT_MS,
    });
    return response.data as T;
  }

  const response = await apiClient.request({
    ...config,
    headers,
    timeout: REQUEST_TIMEOUT_MS,
  });
  return response.data as T;
};

export async function postGraphql<T>(
  query: string,
  variables?: object,
  context?: ApiRequestContext
) {
  try {
    const response = await request<{
      data?: T;
      errors?: { message?: string }[];
    }>({
      data: {
        query,
        variables,
      },
      method: 'POST',
      url: runtimeConfig.projectApiBaseUrl || '/project-api/graphql',
      headers: contextHeaders(context),
    });
    const errors = response.errors;
    if (errors?.length) {
      throw new ApiError(errors[0].message || 'GraphQL request failed', 'UNKNOWN');
    }
    if (response.data == null) {
      throw new ApiError('GraphQL response contained no data', 'UNKNOWN');
    }
    return response.data as T;
  } catch (error) {
    throw normalizeApiError(error);
  }
}

export async function getJson<T>(url: string, context?: ApiRequestContext) {
  try {
    return await request<T>({
      method: 'GET',
      url,
      headers: contextHeaders(context),
    });
  } catch (error) {
    throw normalizeApiError(error);
  }
}
