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

// ============================================================================
// Platform API Client
// ----------------------------------------------------------------------------
// Native-fetch client for the Platform API, routed same-origin through the BFF.
//
// Auth model (BFF):
//   Every request rides the HttpOnly `_ai_workspace_session` cookie (credentials:
//   'include'), which now carries the JWT itself; the BFF reads that token
//   straight from the cookie and injects it as the bearer when proxying to the
//   Platform API. State-mutating requests carry a custom CSRF header that
//   cross-site attackers cannot set.
// ============================================================================

import { PLATFORM_API_BASE_URL, CSRF_HEADER, CSRF_VALUE } from '../config.env';
import { logger } from '../utils/logger';
import { HttpMethod, ApiRequestConfig, GQLResponse } from '../utils/types';
import { buildApiError } from '../utils/apiError';

export type { HttpMethod, ApiRequestConfig, GQLResponse };
export type { ApiError, FieldError } from '../utils/apiError';

// ---------------------------------------------------------------------------
// Token shims — tokens now live server-side in the BFF session, never in the
// browser. These are kept as no-ops so existing call-sites keep compiling.
// ---------------------------------------------------------------------------
export const getStoredToken = (): string => '';
export const getOrgToken = (): string => '';
export const setStoredToken = (_token: string) => { /* tokens are BFF-side only */ };
export const clearStoredToken = () => { /* tokens are BFF-side only */ };

// eslint-disable-next-line @typescript-eslint/no-unused-vars
export const setHttpRequestFn = (_fn: unknown) => { /* no-op in platform mode */ };

/**
 * Default headers. No Authorization header — the BFF injects the bearer token.
 * The CSRF header is always sent (harmless on GET, required on mutations).
 */
const buildHeaders = (extra?: Record<string, string>): Record<string, string> => {
  return {
    Accept: 'application/json',
    'Content-Type': 'application/json',
    [CSRF_HEADER]: CSRF_VALUE,
    ...extra,
  };
};

/**
 * Build the full URL, appending query params if provided.
 */
const buildUrl = (
  path: string,
  baseUrl: string,
  params?: Record<string, unknown>,
): string => {
  const full = path.startsWith('http') ? path : `${baseUrl}${path}`;
  if (!params || Object.keys(params).length === 0) return full;

  const url = new URL(full);
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== null) url.searchParams.append(k, String(v));
  }
  return url.toString();
};

/**
 * Core fetch wrapper — all HTTP methods.
 */
export const request = async <T>(config: ApiRequestConfig): Promise<T> => {
  const { path, method, data, params, headers, baseUrl } = config;
  const resolvedBase = baseUrl || PLATFORM_API_BASE_URL;
  const url = buildUrl(path, resolvedBase, params);

  logger.info(`[platformApiClient] ${method} ${url}`);

  const res = await fetch(url, {
    method,
    credentials: 'include',
    headers: buildHeaders(headers),
    body: data && ['POST', 'PUT', 'PATCH'].includes(method)
      ? JSON.stringify(data)
      : undefined,
  });

  if (!res.ok) {
    let data: unknown;
    try {
      data = await res.json();
    } catch { /* body not JSON */ }
    const err = buildApiError(res.status, data, `HTTP ${res.status}`);
    logger.error(
      `[platformApiClient] ${method} ${url} → ${res.status} [${err.code ?? 'UNKNOWN'}]: ${err.message}`
      + (err.trackingId ? ` (trackingId: ${err.trackingId})` : ''),
    );
    throw err;
  }

  // 204 No Content
  if (res.status === 204) return undefined as T;

  return res.json() as Promise<T>;
};

export const get = <T>(
  path: string,
  params?: Record<string, unknown>,
  baseUrl?: string,
): Promise<T> => request<T>({ path, method: 'GET', params, baseUrl });

export const post = <T>(
  path: string,
  data?: unknown,
  baseUrl?: string,
): Promise<T> => request<T>({ path, method: 'POST', data, baseUrl });

/**
 * POST with multipart/form-data body (e.g. for the secrets endpoint).
 * Omits Content-Type so the browser sets it with the correct boundary.
 */
const sendForm = async <T>(
  method: 'POST' | 'PUT',
  path: string,
  form: FormData,
  baseUrl?: string,
): Promise<T> => {
  const resolvedBase = baseUrl || PLATFORM_API_BASE_URL;
  const url = buildUrl(path, resolvedBase);
  // No Content-Type — the browser sets the multipart boundary. The BFF injects
  // the bearer token; we only add the CSRF header for this state-mutating call.
  const headers: Record<string, string> = { Accept: 'application/json', [CSRF_HEADER]: CSRF_VALUE };

  const res = await fetch(url, { method, credentials: 'include', headers, body: form });

  if (!res.ok) {
    let data: unknown;
    try {
      data = await res.json();
    } catch { /* body not JSON */ }
    const err = buildApiError(res.status, data, `HTTP ${res.status}`);
    logger.error(
      `[platformApiClient] ${method} ${url} → ${res.status} [${err.code ?? 'UNKNOWN'}]: ${err.message}`
      + (err.trackingId ? ` (trackingId: ${err.trackingId})` : ''),
    );
    throw err;
  }

  return res.json() as Promise<T>;
};

export const postForm = <T>(path: string, form: FormData, baseUrl?: string): Promise<T> =>
  sendForm<T>('POST', path, form, baseUrl);

export const putForm = <T>(path: string, form: FormData, baseUrl?: string): Promise<T> =>
  sendForm<T>('PUT', path, form, baseUrl);

export const put = <T>(
  path: string,
  data?: unknown,
  baseUrl?: string,
): Promise<T> => request<T>({ path, method: 'PUT', data, baseUrl });

export const del = <T>(
  path: string,
  params?: Record<string, unknown>,
  baseUrl?: string,
): Promise<T> => request<T>({ path, method: 'DELETE', params, baseUrl });

export const patch = <T>(
  path: string,
  data?: unknown,
  baseUrl?: string,
): Promise<T> => request<T>({ path, method: 'PATCH', data, baseUrl });

// ---------------------------------------------------------------------------
// GraphQL shim — the platform API uses REST, but a few call-sites still
// call graphqlQuery(). We forward the request and return an empty data set
// so the UI degrades gracefully rather than crashing.
// ---------------------------------------------------------------------------
export const graphqlQuery = async <T>(
  _query: string,
): Promise<GQLResponse<T>> => {
  logger.warn('[platformApiClient] graphqlQuery called — platform API is REST-only. Returning empty data.');
  return { data: {} as T };
};

/** Backwards-compat default export */
const choreoApiClient = { request, get, post, put, del, patch, graphqlQuery };
export default choreoApiClient;
