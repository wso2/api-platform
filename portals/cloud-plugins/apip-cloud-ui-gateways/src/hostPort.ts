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
 * Hand-mirrors `AIWorkspaceHostPort` from api-platform's
 * `portals/ai-workspace/src/hostPort.tsx` — api-platform and apim-saas are
 * separate git repos, so this type is duplicated by hand (small, stable,
 * rarely-changing) rather than imported. This package only ever receives a
 * value of this shape as a plain prop; never a shared React Context.
 */
export type NotifySeverity = 'success' | 'info' | 'warning' | 'error';

/**
 * A same-origin, host-authenticated call to platform-api. `path` is relative to
 * the API base the host prepends (`/proxy/api/v0.9` in the console), so no base
 * URL, token, organization header or CSRF header belongs in this package — the
 * Port owns all four. Rejects with an `Error` carrying the API's own message.
 */
export type ApiFetch = <T = unknown>(
  method: string,
  path: string,
  body?: unknown
) => Promise<T>;

export type AIWorkspaceHostPort = {
  orgHandle: string;
  projectHandle?: string;
  navigate: (path: string) => void;
  notify: (message: string, severity?: NotifySeverity) => void;
  /** Optional: the console's Port carries it, the AI Workspace's does not yet. */
  apiFetch?: ApiFetch;
};
