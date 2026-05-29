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
// Replaces the Asgardeo-backed httpRequest with native fetch.
// All calls go to PLATFORM_API_BASE_URL (default: https://localhost:9243/api/v1).
//
// Token storage:
//   sessionStorage.setItem('platform_auth_token', '<your-token>')
// Run the STS service (sts/README.md) to obtain a token.
// ============================================================================

import { PLATFORM_API_BASE_URL } from '../config.env';
import { logger } from '../utils/logger';
import { HttpMethod, ApiRequestConfig, GQLResponse } from '../utils/types';

export type { HttpMethod, ApiRequestConfig, GQLResponse };

const TOKEN_KEY     = 'platform_auth_token';
const ORG_TOKEN_KEY = 'platform_org_token';

// TODO: [REMOVE BEFORE PRODUCTION] Hardcoded dev token — swap for real auth flow.
// Used for ALL platform API calls (GET /organizations, GET /projects, proxies, gateways, …).
// JWT `organization` claim: 770331ed-90ea-477a-b08f-a30f0a03853f
const DEV_GET_ORG_TOKEN =
  'eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9' +
  '.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE3Njk0OTExNDUsImV4cCI6MTgwMTAyNzE0NSwiYXVkIjoid3d3LmV4YW1wbGUuY29tIiwic3ViIjoiamJvY2tldEBleGFtcGxlLmNvbSIsIm9yZ2FuaXphdGlvbiI6Ijc3MDMzMWVkLTkwZWEtNDc3YS1iMDhmLWEzMGYwYTAzODUzZiJ9' +
  '.ylWxQypB6N0-6sFAG6axtfaqT-mZKum_0q_CjYDAHHw';

/**
 * Token for all org-scoped workspace API calls (projects, proxies, gateways, …).
 * Falls back to DEV_GET_ORG_TOKEN so every call carries the correct `organization`
 * JWT claim that the platform API uses to scope results.
 * Priority: sessionStorage('platform_auth_token') → DEV_GET_ORG_TOKEN.
 * TODO: [REMOVE BEFORE PRODUCTION] Remove DEV_GET_ORG_TOKEN fallback.
 */
export const getStoredToken = (): string =>
  sessionStorage.getItem(TOKEN_KEY) ?? DEV_GET_ORG_TOKEN;

/**
 * Token specifically for GET /organizations — must have the registered org UUID
 * in the JWT `organization` claim so the platform API resolves the right org.
 * Priority: sessionStorage('platform_org_token') → DEV_GET_ORG_TOKEN.
 * TODO: [REMOVE BEFORE PRODUCTION] Remove DEV_GET_ORG_TOKEN fallback.
 */
export const getOrgToken = (): string =>
  sessionStorage.getItem(ORG_TOKEN_KEY) ?? DEV_GET_ORG_TOKEN;

/** Persist a bearer token for subsequent requests (overrides the dev fallback). */
export const setStoredToken = (token: string) =>
  sessionStorage.setItem(TOKEN_KEY, token);

/** Clear the stored bearer token (falls back to DEV_FALLBACK_TOKEN until removed). */
export const clearStoredToken = () =>
  sessionStorage.removeItem(TOKEN_KEY);

// ---------------------------------------------------------------------------
// No-op shim so existing call-sites that call setHttpRequestFn(httpRequest)
// continue to compile without changes.
// ---------------------------------------------------------------------------
// eslint-disable-next-line @typescript-eslint/no-unused-vars
export const setHttpRequestFn = (_fn: unknown) => { /* no-op in platform mode */ };

/**
 * Default headers — adds Authorization if a token is stored.
 */
const buildHeaders = (extra?: Record<string, string>): Record<string, string> => {
  const h: Record<string, string> = {
    Accept: 'application/json',
    'Content-Type': 'application/json',
    ...extra,
  };
  const token = getStoredToken();
  if (token) {
    h.Authorization = `Bearer ${token}`;
  }
  return h;
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
    throw new Error(message);
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
