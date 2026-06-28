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
//   The browser holds NO token. Every request rides the HttpOnly `_bff_session`
//   cookie (credentials: 'include'); the BFF looks up the session and injects the
//   bearer token when proxying to the Platform API. State-mutating requests carry
//   a custom CSRF header that cross-site attackers cannot set.
// ============================================================================

import { PLATFORM_API_BASE_URL, CSRF_HEADER, CSRF_VALUE } from '../config.env';
import { logger } from '../utils/logger';
import { HttpMethod, ApiRequestConfig, GQLResponse } from '../utils/types';

export type { HttpMethod, ApiRequestConfig, GQLResponse };

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
    let message = `HTTP ${res.status}`;
    try {
      const body = await res.json();
      message = body?.description ?? body?.message ?? body?.error ?? message;
    } catch { /* body not JSON */ }
    logger.error(`[platformApiClient] ${method} ${url} → ${res.status}: ${message}`);
    // Attach the HTTP status so callers (e.g. ErrorAlert) can react to it —
    // notably surfacing a logout action on 401 instead of a futile retry.
    const err = new Error(message) as Error & { status?: number };
    err.status = res.status;
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
export const postForm = async <T>(
  path: string,
  form: FormData,
  baseUrl?: string,
): Promise<T> => {
  const resolvedBase = baseUrl || PLATFORM_API_BASE_URL;
  const url = buildUrl(path, resolvedBase);
  // No Content-Type — the browser sets the multipart boundary. The BFF injects
  // the bearer token; we only add the CSRF header for this state-mutating call.
  const headers: Record<string, string> = { Accept: 'application/json', [CSRF_HEADER]: CSRF_VALUE };

  const res = await fetch(url, { method: 'POST', credentials: 'include', headers, body: form });

  if (!res.ok) {
    let message = `HTTP ${res.status}`;
    try {
      const body = await res.json();
      message = body?.description ?? body?.message ?? body?.error ?? message;
    } catch { /* body not JSON */ }
    throw new Error(message);
  }

  return res.json() as Promise<T>;
};

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
