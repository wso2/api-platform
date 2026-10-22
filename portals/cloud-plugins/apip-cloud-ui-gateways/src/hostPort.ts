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
 * Hand-mirrors the host Port from api-platform's portal `src/hostPort.tsx`
 * (`AIWorkspaceHostPort` / `CloudHostPort` — same shape) — api-platform and
 * apim-saas are separate git repos, so this type is duplicated by hand (small,
 * stable, rarely-changing) rather than imported. This package only ever
 * receives a value of this shape as a plain prop; never a shared React Context.
 */
export type NotifySeverity = 'success' | 'info' | 'warning' | 'error';

/**
 * A same-origin, host-authenticated call to platform-api. `path` is relative to
 * the API base (e.g. `/managed-gateways`); the host prepends its proxy base,
 * attaches the bearer/CSRF as its transport requires, and resolves JSON — or
 * `undefined` for a 204/205 or an empty successful body — rejecting with an
 * `Error` on failure. Feature code never sees a token or a base URL. The
 * resolved type includes `undefined` so callers narrow before dereferencing an
 * empty response.
 */
export type ApiFetch = <T = unknown>(
  method: string,
  path: string,
  body?: unknown
) => Promise<T | undefined>;

/**
 * One gateway release the Quick Start step can offer a download for, keyed by
 * `type` since the AI gateway and the regular/event gateway ship as separate
 * artifacts. Mirrors AI Workspace's own `GatewayVersionEntry`
 * (`src/config.env.ts`) so the two hosts can feed this from whatever each
 * already has — AI Workspace from its existing `PLATFORM_GATEWAY_VERSIONS`,
 * API Control Plane from its own runtime config — without this package caring
 * which.
 */
export type GatewayVersionEntry = {
  type: 'ai' | 'regular' | 'event';
  version: string;
  /** Release tag, e.g. `v1.2.0`. Defaults to `v${version}` when omitted. */
  latestVersion?: string;
  /** Zip/folder version when it differs from `version` (rare). */
  distVersion?: string;
  channel: 'STS' | 'LTS';
};

export type AIWorkspaceHostPort = {
  orgHandle: string;
  projectHandle?: string;
  navigate: (path: string) => void;
  notify: (message: string, severity?: NotifySeverity) => void;
  apiFetch: ApiFetch;
  /** Available release versions per gateway type, for the self-hosted Quick Start panel. */
  gatewayVersions: GatewayVersionEntry[];
};
