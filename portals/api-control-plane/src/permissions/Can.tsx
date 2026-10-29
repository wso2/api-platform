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

import { cloneElement, isValidElement, type ReactNode } from 'react';
import { Tooltip } from '@wso2/oxygen-ui';

import { useResourceLimits } from '../hooks/useResourceLimits';

import { useActionPermission, useCanAny } from './useCan';
import { isWriteOperation, type PermissionOperation } from './evaluate';

/**
 * How to render a denied control.
 *
 * `hide` — render `fallback`, or nothing.
 * `disable` — render the control disabled with a tooltip.
 */
export type DeniedBehaviour = 'hide' | 'disable';

export type CanProps = {
  /**
   * The OpenAPI `operationId` to check. An array allows any matching operation.
   */
  do: PermissionOperation | readonly PermissionOperation[];
  children: ReactNode;
  /** See `DeniedBehaviour`. Defaults to `hide`. */
  denied?: DeniedBehaviour;
  /** Rendered instead of `children` when denied and `denied` is `hide`. */
  fallback?: ReactNode;
};

/**
 * Renders `children` only when the user is permitted to perform the operation.
 *
 *     <Can do="CreateProject"><Button …>New project</Button></Can>
 *     <Can do="DeleteProject" denied="disable"><Button …>Delete</Button></Can>
 *     <Can do="ListProjects" fallback={<ForbiddenState />}><Body /></Can>
 *
 * Use `denied="hide"` to omit the control, or `denied="disable"` to render
 * it disabled with a tooltip. For more complex denial states, use
 * `useActionPermission` directly.
 *
 * It also honours **read-only mode**: when something has reported that the
 * organization may not write at all, every *write* operation renders disabled
 * with that explanation, whatever the user's scopes say. Reads are untouched,
 * so a read-only organization still browses normally. Which operations write is
 * taken from the spec's own scope map (`isWriteOperation`), not from their
 * names, so a new write surface is covered the day it is generated rather than
 * when someone remembers to wrap it.
 *
 * That is deliberately here rather than at ~26 call sites: read-only is an
 * organization-wide fact, and a rule applied by hand to every control is one
 * that silently misses the twenty-seventh. Nothing reports read-only unless a
 * supplier is mounted, so this is inert in a standalone build.
 *
 * This component provides only a UI guard; server-side authorization remains
 * required.
 */
export function Can({ do: operations, children, denied = 'hide', fallback }: CanProps) {
  const list = Array.isArray(operations) ? operations : [operations];
  // Both hooks run unconditionally; `useCanAny` gates multi-op checks and
  // `useActionPermission` supplies the single-op tooltip.
  const anyAllowed = useCanAny(list);
  const action = useActionPermission(list[0]);
  const allowed = list.length > 1 ? anyAllowed : action.allowed;

  // Permission outranks read-only: there is no point explaining that an
  // organization cannot write to someone who could not perform the operation
  // anyway, and `denied="hide"` must keep hiding rather than start disabling.
  const { readOnly, readOnlyMessage } = useResourceLimits();
  const blockedByReadOnly = allowed && readOnly && list.some(isWriteOperation);

  if (allowed && !blockedByReadOnly) return <>{children}</>;
  if (!allowed && denied === 'hide') return <>{fallback ?? null}</>;

  // `disable` needs a single child so we can clone it with `disabled`.
  // `isValidElement` gives a clearer error than `Children.only`.
  if (!isValidElement(children)) {
    throw new Error(
      'Can needs a single element child that accepts a `disabled` prop in order to ' +
        'disable it. Wrap plain content in a control, or use denied="hide".',
    );
  }
  const child = children;

  return (
    <Tooltip title={blockedByReadOnly ? readOnlyMessage : (action.tooltip ?? '')}>
      {/*
        A disabled MUI control sets `pointer-events: none`, so it never fires
        the enter/leave events a Tooltip listens for, the explanation would
        simply never appear. Wrapping in a span that stays interactive is the
        documented way around it; `inline-flex` keeps the control's own box
        intact inside a Stack or button group.
      */}
      <span style={{ display: 'inline-flex' }}>
        {cloneElement(child as React.ReactElement<{ disabled?: boolean }>, {
          disabled: true,
        })}
      </span>
    </Tooltip>
  );
}
