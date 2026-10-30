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

import type { BillingOrganization } from './billing/organization';
import type {
  LimitedComponent,
  ResourceLimitSet,
} from './hooks/useResourceLimits';
import { runtimeConfig, type GatewayVersionEntry } from './config/runtime';
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

/**
 * The organization's billing record as an extension reads it. The host owns the
 * call (see `billing/organization`) because it is not a plain read — it performs
 * first-login subscription activation as a side effect — so an extension asks
 * for the record rather than fetching it, and every caller shares one request.
 *
 * Resolves `null` when the BFF has no billing upstream, so an extension needs no
 * deployment config of its own to know billing is unavailable.
 */
export type BillingPort = {
  organization: () => Promise<BillingOrganization | null>;
};

/**
 * The organization's component creation limits, as an extension reads and
 * writes them.
 *
 * The host holds the limits and knows nothing about where they come from; a
 * cloud extension mounted on `APP_GATE_SLOT` is what supplies them, via `set`.
 * `canCreate`/`limitMessage` are here for extensions that read rather than
 * write — a plugin-owned page gating its own create button.
 *
 * Every field is inert in a deployment with no supplier: `canCreate` answers
 * `true`, `readOnly` is `false`, and the setters do nothing. See
 * `hooks/useResourceLimits`, which owns the shape and that default.
 */
export type ResourceLimitsPort = {
  canCreate: (component: LimitedComponent) => boolean;
  limitMessage: (component: LimitedComponent) => string;
  set: (limits: ResourceLimitSet | null) => void;
  /**
   * True when the organization may not create or update anything at all.
   * `canCreate` already answers `false` for every component while this holds;
   * read this one to gate an edit or an update, which no count covers.
   */
  readOnly: boolean;
  /** Why the organization is read-only; '' when it is not. */
  readOnlyMessage: string;
  /**
   * The banner's content when the supplier provides one — its own markup, so
   * the wording and any link stay on that side.
   */
  readOnlyBanner: ReactNode;
  /** Supply the read-only verdict. The supplying extension calls this. */
  setReadOnly: (readOnly: boolean, reason?: string, banner?: ReactNode) => void;
  /**
   * Ask the supplier to re-read the limits, after something that may have
   * changed a count. A no-op when no supplier is mounted.
   */
  refresh: () => void;
  /**
   * Bumped by `refresh`; the supplier watches it to know a re-read was asked
   * for. Readers of `canCreate` have no use for it.
   */
  refreshSignal: number;
};

/**
 * The inert `resourceLimits` a deployment with no supplier holds: nothing
 * capped, nothing read-only, every setter a no-op. Exported so a test (or any
 * caller assembling a Port outside the shell) states the standalone default
 * rather than re-spelling it, and so that default lives in one place.
 */
export const NO_RESOURCE_LIMITS: ResourceLimitsPort = {
  canCreate: () => true,
  limitMessage: () => '',
  set: () => {},
  readOnly: false,
  readOnlyMessage: '',
  readOnlyBanner: null,
  setReadOnly: () => {},
  refresh: () => {},
  refreshSignal: 0,
};

/**
 * Resource kinds an extension can report as changed. Names, not query keys:
 * the host owns the keys, so the cache layout can change without touching a
 * plugin.
 */
export type InvalidatableResource = 'gateways' | 'apiPortals';

/**
 * How an extension reports that it changed data the host also caches. The
 * host's own mutations invalidate React Query; one writing through `apiFetch`
 * bypasses that, leaving stale counts (an Overview card) until the entry
 * expires on its own.
 */
export type ResourcesPort = {
  /** Drop every cached list, detail and child of `resource` for this org. */
  invalidate: (resource: InvalidatableResource) => void;
};

export type CloudHostPort = {
  orgHandle: string;
  projectHandle?: string;
  /** The API in scope, on the API-scoped routes. Absent elsewhere. */
  apiHandle?: string;
  navigate: (path: string) => void;
  notify: (message: string, severity?: NotifySeverity) => void;
  apiFetch: ApiFetch;
  billing: BillingPort;
  /** Gateway releases from the BFF's `[[gateway_versions]]` config, newest first per type. */
  gatewayVersions?: GatewayVersionEntry[];
  /** `host:port` a self-hosted gateway's controller dials, from the console's runtime config. */
  gatewayControlPlaneHost?: string;
  resourceLimits: ResourceLimitsPort;
  resources: ResourcesPort;
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
  // A form body is passed through as it is: multipart is the only shape some
  // platform endpoints accept (creating a secret, for one), and the browser has to
  // set Content-Type itself so the multipart boundary it generates is declared.
  const isForm = typeof FormData !== 'undefined' && body instanceof FormData;
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (body !== undefined && !isForm) headers['Content-Type'] = 'application/json';
  if (MUTATING_METHODS.has(verb)) headers[CSRF_HEADER] = CSRF_HEADER_VALUE;
  const response = await fetch(`${base}${path}`, {
    method: verb,
    headers,
    body: body === undefined ? undefined : isForm ? (body as FormData) : JSON.stringify(body),
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
