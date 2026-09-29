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

import { useMemo } from 'react';
import { useIntl } from 'react-intl';

import { permissionMessages } from './messages';
import { usePermissions } from './PermissionContext';
import type { ApScope, Decision, PermissionOperation } from './evaluate';

/**
 * Whether the user may be offered `operation`, named by its OpenAPI
 * `operationId` — the same string the resource's `endpoints.ts` already passes
 * to the transport.
 *
 *     const canCreate = useCan('CreateProject');
 *
 * Ask for the operation, never for a scope string: the generated map supplies
 * the whole accepted list, including the broader `:manage` scope, so a caller
 * cannot accidentally lock out an org admin by naming only `ap:project:create`.
 */
export const useCan = (operation: PermissionOperation): boolean => usePermissions().can(operation);

/**
 * Whether *any* of `operations` may be offered.
 *
 * The question an overflow menu asks before rendering its trigger — a menu that
 * opens onto four disabled rows is noise, so hide the trigger when nothing in
 * it is reachable.
 *
 * Not memoized: an inline array literal changes identity every render, so a
 * `useMemo` would need a stringified key, and the work being avoided is a
 * handful of set lookups over two or three operations. The expensive part (the
 * granted-scope set) is already memoized once in the provider.
 */
export const useCanAny = (operations: readonly PermissionOperation[]): boolean =>
  usePermissions().canAny(operations);

/**
 * Whether the user holds one specific scope.
 *
 * For the ownership overrides that do not correspond to a single operation —
 * `ap:api_key:all:manage` widens *which rows* of a creator-scoped resource the
 * caller may act on, not which endpoint they may call. Combine it with the row
 * itself in the owning resource module; this hook answers only half the
 * question.
 */
export const useHasScope = (scope: ApScope | (string & {})): boolean =>
  usePermissions().hasScope(scope);

export type ActionPermission = Decision & {
  /**
   * Ready-to-render explanation, or `undefined` when allowed — so it can be
   * handed straight to a `Tooltip` that then renders nothing.
   */
  tooltip?: string;
};

/**
 * A decision plus the copy that explains it, for a control that stays visible
 * but disabled.
 *
 *     const { allowed, tooltip } = useActionPermission('DeleteProject');
 *
 * The copy lives here rather than at each call site for two reasons: it must be
 * translated (so it needs a stable message id, not a literal), and it must read
 * identically on every one of the roughly fifty controls that will use it. A
 * per-page string is how a console ends up explaining the same rule four
 * different ways.
 *
 * Note that a `loading` decision is `allowed` with no tooltip, so a control
 * renders enabled while the session resolves rather than flashing disabled.
 */
export const useActionPermission = (operation: PermissionOperation): ActionPermission => {
  const { decide } = usePermissions();
  const intl = useIntl();
  const decision = decide(operation);

  return useMemo(
    () => ({
      ...decision,
      tooltip: decision.allowed ? undefined : intl.formatMessage(permissionMessages.denied),
    }),
    [decision, intl],
  );
};
