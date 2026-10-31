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

import { cloneElement, isValidElement, type ReactElement, type ReactNode } from 'react';
import { Tooltip } from '@wso2/oxygen-ui';

import { useResourceLimits, type LimitedComponent } from '../hooks/useResourceLimits';

import { Can, type DeniedBehaviour } from './Can';
import type { PermissionOperation } from './evaluate';

export type CanCreateProps = {
  /** The OpenAPI `operationId` the user needs. */
  do: PermissionOperation | readonly PermissionOperation[];
  /**
   * Which product component this creates, for the limit check. Omit for a
   * create action nothing caps, and this behaves exactly like `Can`.
   */
  component?: LimitedComponent;
  children: ReactNode;
  /**
   * How to render when the user lacks **permission**. Passed straight through
   * to `Can`, defaulting to `hide` as `Can` does.
   *
   * Deliberately not hardcoded: surfaces differ today — the API list hides its
   * create button on denial while the gateways and subscription-plan settings
   * pages disable theirs — so fixing a default here would silently change two
   * of them. A limit, by contrast, always disables: hiding a control because a
   * quota is full tells the user nothing about why it vanished.
   */
  denied?: DeniedBehaviour;
  /** Rendered instead of `children` when permission is denied and `denied` is `hide`. */
  fallback?: ReactNode;
};

/**
 * A create control gated on all three of permission, write access and quota.
 *
 *     <CanCreate do="CreateRESTAPI" component="restApis">
 *       <Button variant="contained">Create API</Button>
 *     </CanCreate>
 *
 * **Precedence**, outermost first:
 *
 * | Condition        | Behaviour                                  |
 * |------------------|--------------------------------------------|
 * | No permission    | per `denied` — `hide` by default           |
 * | Read-only org    | disabled, with the supplier's explanation  |
 * | At the limit     | disabled, with the supplier's explanation  |
 * | Loading / none   | **enabled**                                |
 *
 * Permission outranks the rest because there is no point explaining a quota to
 * someone who could not create anyway. Read-only outranks quota, which is how
 * `canCreate` already encodes it.
 *
 * The loading row matters: an unresolved decision renders **enabled**, matching
 * what `useActionPermission` already does for permissions "rather than flashing
 * disabled". A wrong "allowed" costs a failed submit; a wrong "denied" would
 * lock a user out of their own product. The control plane refuses an
 * over-limit create regardless, so this is a courtesy and never the guard.
 *
 * Nesting two wrappers would not work: an outer `Can denied="disable"` clones
 * the inner wrapper rather than the button, so the `disabled` prop never
 * reaches the control. Hence one component evaluating all three.
 */
export function CanCreate({
  do: operations,
  component,
  children,
  denied,
  fallback,
}: CanCreateProps) {
  const { canCreate, limitMessage, readOnly } = useResourceLimits();

  // Read-only is `Can`'s job — it already disables every write operation with
  // the organization-wide explanation, and `CreateX` is always a write. Leaving
  // it to `Can` avoids disabling the same control twice and nesting a second
  // tooltip span inside the first.
  //
  // What remains here is the quota: this organization may write, but has no
  // room for another one of these.
  const blocked = component ? !readOnly && !canCreate(component) : false;

  return (
    <Can do={operations} denied={denied} fallback={fallback}>
      {blocked ? (
        <Tooltip title={component ? limitMessage(component) : ''}>
          {/*
            A disabled MUI control sets `pointer-events: none` and so never
            fires the enter/leave events a Tooltip listens for. Wrapping in a
            span that stays interactive is the documented way around it, and
            `inline-flex` keeps the control's own box intact inside a Stack or
            button group — the same workaround `Can` uses.
          */}
          <span style={{ display: 'inline-flex' }}>{disable(children)}</span>
        </Tooltip>
      ) : (
        children
      )}
    </Can>
  );
}

/** Clones a single element child with `disabled`, as `Can` does for its own denial. */
function disable(children: ReactNode): ReactNode {
  if (!isValidElement(children)) {
    throw new Error(
      'CanCreate needs a single element child that accepts a `disabled` prop ' +
        'so it can be disabled when a limit is reached.',
    );
  }
  return cloneElement(children as ReactElement<{ disabled?: boolean }>, { disabled: true });
}
