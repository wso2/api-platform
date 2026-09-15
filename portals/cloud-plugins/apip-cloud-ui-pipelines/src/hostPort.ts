/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
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
 * the API base (e.g. `/pipelines`); the host prepends its proxy base, attaches
 * the bearer/CSRF as its transport requires, and resolves JSON — or `undefined`
 * for a 204/205 or an empty successful body — rejecting with an `Error` on
 * failure. Feature code never sees a token or a base URL. The resolved type
 * includes `undefined` so callers narrow before dereferencing an empty response.
 */
export type ApiFetch = <T = unknown>(
  method: string,
  path: string,
  body?: unknown
) => Promise<T | undefined>;

export type AIWorkspaceHostPort = {
  orgHandle: string;
  projectHandle?: string;
  navigate: (path: string) => void;
  notify: (message: string, severity?: NotifySeverity) => void;
  apiFetch: ApiFetch;
};
