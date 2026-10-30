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
 * Hand-mirrors `CloudHostPort` from api-platform's
 * `portals/api-control-plane/src/hostPort.tsx` — api-platform and apim-saas are
 * separate git repos, so this type is duplicated by hand (small, stable,
 * rarely-changing) rather than imported. This package only ever receives a
 * value of this shape as a plain prop; never a shared React Context.
 */
export type NotifySeverity = 'success' | 'info' | 'warning' | 'error';

/**
 * A same-origin, host-authenticated call to the platform API. `path` is relative
 * to the API base, so this package never knows the host's transport, base URL or
 * auth.
 */
// Resolves to undefined when the response carries no body — a 204, say. The other
// cloud plugins declare it the same way, which is what lets one feature be hosted by
// both the console and the AI Workspace.
export type ApiFetch = <T = unknown>(
  method: string,
  path: string,
  body?: unknown
) => Promise<T | undefined>;

/**
 * One gateway release, keyed by `type`; the first entry of a type is its latest.
 * Hand-mirrors the host's `GatewayVersionEntry` (`config/runtime.ts`).
 */
export type GatewayVersionEntry = {
  type: 'ai' | 'regular' | 'event';
  version: string;
  /** Release tag, e.g. `v1.2.0`. Defaults to `v${version}` when omitted. */
  latestVersion?: string;
  /** Zip/folder version when it differs from `version`. */
  distVersion?: string;
  channel: 'STS' | 'LTS';
};

export type CloudHostPort = {
  orgHandle: string;
  projectHandle?: string;
  /** The API being deployed. Deploying needs one, so the page says so when it is absent. */
  apiHandle?: string;
  navigate: (path: string) => void;
  notify: (message: string, severity?: NotifySeverity) => void;
  apiFetch: ApiFetch;
  /** Gateway releases from the host's config; the first `regular` entry is the latest. */
  gatewayVersions?: GatewayVersionEntry[];
  /** `host:port` a self-hosted gateway's controller dials; falls back to this page's own host. */
  gatewayControlPlaneHost?: string;
};
