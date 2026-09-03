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
 * Hand-mirrors the host Port from a portal's own `src/hostPort.tsx`
 * (`CloudHostPort` / `AIWorkspaceHostPort` — the same shape). This package only
 * ever receives a value of this shape as a plain prop, never a shared React
 * Context, which is what lets more than one host render it without this package
 * depending on either one's internals.
 */
export type NotifySeverity = 'success' | 'info' | 'warning' | 'error';

/**
 * A same-origin, host-authenticated call to the platform API. `path` is relative
 * to the API base (e.g. `/projects/x/apis/y/deployments`); the host prepends its
 * proxy base, attaches the bearer/CSRF as its transport requires, and resolves
 * JSON — or `undefined` for a 204/205 or an empty successful body — rejecting
 * with an `Error` on failure. Feature code never sees a token or a base URL. The
 * resolved type includes `undefined` so callers narrow before dereferencing an
 * empty response.
 */
export type ApiFetch = <T = unknown>(
  method: string,
  path: string,
  body?: unknown
) => Promise<T | undefined>;

export type CloudHostPort = {
  orgHandle: string;
  projectHandle?: string;
  /**
   * Handle of the API in scope. Undefined when a host renders this feature
   * without an API in scope, which the feature reports rather than guessing at.
   */
  apiHandle?: string;
  navigate: (path: string) => void;
  notify: (message: string, severity?: NotifySeverity) => void;
  apiFetch: ApiFetch;
};

/** Kept under its original name so an existing host registration still compiles. */
export type AIWorkspaceHostPort = CloudHostPort;
