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

/**
 * Hand-mirrors the host Port from the portal's `src/hostPort.tsx`
 * (`AIWorkspaceHostPort` / `CloudHostPort`, same shape). Duplicated rather than
 * imported because the portals live in a separate repo; it arrives as a plain
 * prop, never a shared React Context.
 */
export type NotifySeverity = 'success' | 'info' | 'warning' | 'error';

/**
 * A same-origin, host-authenticated call to platform-api. `path` is relative to
 * the API base (e.g. `/logs`); the host prepends its proxy base, attaches
 * bearer/CSRF, and resolves JSON — or `undefined` for a 204/205 or empty body —
 * rejecting on failure. Feature code never sees a token or a base URL.
 */
export type ApiFetch = <T = unknown>(
  method: string,
  path: string,
  body?: unknown
) => Promise<T | undefined>;

/**
 * The signed-in user's permissions, as the host answers them. `canAny` takes an
 * operation's accepted-scope list — the cloud spec's any-of list, `:manage`
 * scopes included — and says whether the user may be offered it. Mirrors the
 * host's `PermissionsPort`. Not a security boundary: platform-api decides.
 */
export type PermissionsPort = {
  canAny: (scopes: readonly string[]) => boolean;
};

export type AIWorkspaceHostPort = {
  orgHandle: string;
  projectHandle?: string;
  navigate: (path: string) => void;
  notify: (message: string, severity?: NotifySeverity) => void;
  apiFetch: ApiFetch;
  /**
   * Optional: a host whose Port carries no permission check omits it and
   * nothing is gated — every control stays enabled exactly as before, and
   * platform-api still refuses what the user may not do. See `permissionsOf`
   * in `permissions.ts`.
   */
  permissions?: PermissionsPort;
};
