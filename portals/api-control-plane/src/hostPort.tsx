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
 *
 * The "Port": the small set of host capabilities an extension's `render`
 * function receives, so a feature component never imports this portal's
 * own hooks (`useConsoleScope`, `useNotifications`) directly — that's what
 * would make it impossible to reuse the same component in another host.
 *
 * This is a real React context, but — unlike `slots/index.tsx` — it is NOT
 * meant to be imported across the api-platform/apim-saas repo boundary.
 * apim-saas's host file (e.g. `hosts/api-control-plane.tsx`) hand-mirrors
 * this exact `CloudHostPort` type (same convention as `ApiControlPlaneExtension`)
 * and receives a value of that shape as a plain function argument via
 * `extension.render(port)` — never by importing `PortContext`/`usePort`
 * itself. `PortProvider` is built and mounted once, here in core, from real
 * hooks; only the small type crosses the boundary, never the context object.
 */

import { createContext, useContext, type ReactNode } from 'react';

import { runtimeConfig } from './config/runtime';
import { CSRF_HEADER, CSRF_HEADER_VALUE } from './contexts/auth/authConstants';

export type NotifySeverity = 'success' | 'info' | 'warning' | 'error';

/**
 * A same-origin, host-authenticated call to platform-api that an extension can
 * make without knowing this portal's transport. `path` is relative to the API
 * base (e.g. `/pipelines`). Resolves parsed JSON (or `undefined` for 204) and
 * rejects with an `Error` carrying the API's message on failure.
 */
export type ApiFetch = <T = unknown>(
  method: string,
  path: string,
  body?: unknown
) => Promise<T>;

export type CloudHostPort = {
  orgHandle: string;
  projectHandle?: string;
  navigate: (path: string) => void;
  notify: (message: string, severity?: NotifySeverity) => void;
  apiFetch: ApiFetch;
};

const MUTATING_METHODS = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);

/**
 * The Port's `apiFetch`: a same-origin fetch through the BFF proxy, which
 * injects the bearer token from the session cookie (the browser never holds a
 * token) and validates the CSRF header on mutations. The org is resolved by
 * platform-api from that token, so no `X-Org-Id` is sent here. Kept as a small
 * self-contained wrapper rather than reusing the api layer's transport so it
 * stays on the UI side of the api-layer import boundary.
 */
export const extensionApiFetch: ApiFetch = async <T = unknown>(
  method: string,
  path: string,
  body?: unknown
): Promise<T> => {
  const base = `${runtimeConfig.platformApiBaseUrl}/api/${runtimeConfig.platformApiVersion}`;
  const verb = method.toUpperCase();
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  if (MUTATING_METHODS.has(verb)) headers[CSRF_HEADER] = CSRF_HEADER_VALUE;
  const response = await fetch(`${base}${path}`, {
    method: verb,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (!response.ok) {
    let message = `Request failed (${response.status})`;
    try {
      const parsed = (await response.json()) as { message?: string } | null;
      if (parsed?.message) message = parsed.message;
    } catch {
      /* non-JSON error body — keep the status-based default */
    }
    throw new Error(message);
  }
  if (response.status === 204 || response.status === 205) return undefined as T;
  const text = await response.text();
  return (text ? JSON.parse(text) : undefined) as T;
};

const PortContext = createContext<CloudHostPort | null>(null);

export function PortProvider({
  value,
  children,
}: {
  value: CloudHostPort;
  children: ReactNode;
}) {
  return <PortContext.Provider value={value}>{children}</PortContext.Provider>;
}

export function usePort(): CloudHostPort {
  const port = useContext(PortContext);
  if (!port) {
    throw new Error('usePort must be used within a PortProvider');
  }
  return port;
}
