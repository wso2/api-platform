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

import { useCallback, useEffect, useMemo, type ReactNode } from 'react';

import { onForbidden } from '../api/core/sessionEvents';
import { runtimeConfig } from '../config/runtime';
import { useAuth } from '../contexts/auth/AuthProvider';
import { PermissionContext, type PermissionState } from './PermissionContext';
import {
  decideAnyOperation,
  decideOperation,
  decideScope,
  reportForbiddenDrift,
  toGrantedSet,
  type PermissionInput,
  type PermissionMode,
  type PermissionOperation,
} from './evaluate';

/**
 * Feature flag controlling permission enforcement.
 *
 * Default (off) = permissive: unknown scopes are allowed and the server
 * enforces access. Turn on to enforce UI-side scope checks.
 */
export const PERMISSION_ENFORCEMENT_FLAG = 'ui-permissions';

const configuredMode: PermissionMode = runtimeConfig.featureFlags.includes(
  PERMISSION_ENFORCEMENT_FLAG,
)
  ? 'enforce'
  : 'permissive';

export type PermissionProviderProps = {
  children: ReactNode;
  /**
   * Overrides the flag-derived mode. For tests and Storybook, where flipping a
   * module-scope config value is not practical — not for app code, which should
   * let the deployment decide.
   */
  mode?: PermissionMode;
};

/**
 * Publishes the signed-in user's permissions.
 *
 * Reads scopes from `GET /api/session` via `AuthProvider`, so it keeps no
 * local state or extra fetches.
 */
export function PermissionProvider({ children, mode = configuredMode }: PermissionProviderProps) {
  const { isLoading, user } = useAuth();

  /**
   * The scope claim as a single string used for memoization.
   * AuthProvider recreates user/scopes on each hydrate, so joining
   * prevents unnecessary re-renders. Splitting is safe because OAuth2
   * scopes cannot contain spaces; undefined distinguishes "no claim"
   * from "no scopes granted".
   */
  const scopeClaim = user?.scopes ? user.scopes.join(' ') : undefined;

  const grantedScopes = useMemo(
    () => (scopeClaim === undefined ? undefined : toGrantedSet(scopeClaim.split(' '))),
    [scopeClaim],
  );

  const input = useMemo<PermissionInput>(
    () => ({
      status: isLoading ? 'loading' : 'ready',
      granted: grantedScopes,
      mode,
    }),
    [grantedScopes, isLoading, mode],
  );

  const decide = useCallback(
    (operation: PermissionOperation) => decideOperation(operation, input),
    [input],
  );
  const decideAny = useCallback(
    (operations: readonly PermissionOperation[]) => decideAnyOperation(operations, input),
    [input],
  );
  const decideOne = useCallback((scope: string) => decideScope(scope, input), [input]);

  /**
   * Watches for the server disagreeing with us.
   *
   * The transport publishes every 403 with the operation that drew it; if this
   * provider had predicted that operation was allowed, the generated scope map
   * and the deployed backend have diverged. Reported in dev only — see
   * `reportForbiddenDrift`.
   *
   * Note what this deliberately does *not* do: re-fetch the session. The BFF
   * derives scopes by decoding the token in the session cookie, so asking again
   * with the same cookie returns the same scopes. Only a token refresh can
   * change them, and that path already produces a fresh `user` through
   * `AuthProvider`.
   */
  useEffect(
    () =>
      onForbidden(({ operation }) => {
        if (!operation) return;
        reportForbiddenDrift(operation, decideOperation(operation, input));
      }),
    [input],
  );

  const value = useMemo<PermissionState>(
    () => ({
      mode,
      isLoading,
      grantedScopes,
      input,
      can: (operation) => decide(operation).allowed,
      canAny: (operations) => decideAny(operations).allowed,
      hasScope: (scope) => decideOne(scope).allowed,
      decide,
      decideAny,
      decideScope: decideOne,
    }),
    [decide, decideAny, decideOne, grantedScopes, input, isLoading, mode],
  );

  return <PermissionContext.Provider value={value}>{children}</PermissionContext.Provider>;
}

// Re-exported so a consumer needs one import path.
export { PermissionContext, usePermissions, type PermissionState } from './PermissionContext';
