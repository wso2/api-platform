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

import { useActionPermission, useCanAny } from './useCan';
import type { PermissionOperation } from './evaluate';

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

  if (allowed) return <>{children}</>;
  if (denied === 'hide') return <>{fallback ?? null}</>;

  // `disable` needs a single child so we can clone it with `disabled`.
  // `isValidElement` gives a clearer error than `Children.only`.
  if (!isValidElement(children)) {
    throw new Error(
      'Can with denied="disable" needs a single element child that accepts a `disabled` prop. ' +
        'Wrap plain content in a control, or use denied="hide".',
    );
  }
  const child = children;

  return (
    <Tooltip title={action.tooltip ?? ''}>
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
