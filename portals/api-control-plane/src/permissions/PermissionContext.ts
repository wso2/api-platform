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

import { createContext, useContext } from 'react';

import type { Decision, PermissionInput, PermissionMode, PermissionOperation } from './evaluate';

/**
 * Permission state for the current session.
 * Predicates are plain functions so they can be used in `useMemo`.
 */
export type PermissionState = {
  /** How an unknown scope set is treated. See `PermissionMode`. */
  mode: PermissionMode;
  /** True until `GET /api/session` resolves; while true, predicates return `true`. */
  isLoading: boolean;
  /** Token scopes, or `undefined` when no scope claim was present. */
  grantedScopes?: ReadonlySet<string>;
  /** The assembled evaluator input, for passing to `evaluate`'s pure helpers. */
  input: PermissionInput;

  can: (operation: PermissionOperation) => boolean;
  canAny: (operations: readonly PermissionOperation[]) => boolean;
  hasScope: (scope: string) => boolean;

  /** The full decision, when the reason matters (copy, tests, telemetry). */
  decide: (operation: PermissionOperation) => Decision;
  decideAny: (operations: readonly PermissionOperation[]) => Decision;
  decideScope: (scope: string) => Decision;
};

/**
 * In its own module so a test can inject a stub state through
 * `PermissionContext.Provider` without standing up `AuthProvider` — the same
 * arrangement, and for the same reason, as `AuthStateContext` and
 * `ConsoleScopeContext`.
 */
export const PermissionContext = createContext<PermissionState | null>(null);

/**
 * The whole permission state. Most components want `useCan` instead; reach for
 * this when you need `mode`, `isLoading`, or a predicate to call inside a
 * `useMemo` over a list.
 */
export const usePermissions = (): PermissionState => {
  const context = useContext(PermissionContext);
  if (!context) {
    throw new Error('usePermissions must be used within PermissionProvider');
  }
  return context;
};
