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

import { createContext, useContext, useMemo } from 'react';

import { ApiError, ClientErrorCode } from './errors';
import { orgScope, type OrgScope } from './queryKeys';

/**
 * The org/project context every scoped hook resolves against.
 *
 * This is deliberately a *small, data-only* context, separate from the rich
 * `ConsoleScopeContext` (which also holds the loaded organization, project and
 * API objects). The separation matters: `ConsoleScopeProvider` builds its value
 * by calling data hooks, so if those hooks read the rich context back, the
 * layer becomes circular; which is why the current code has to reach into a
 * nullable raw context and thread explicit override arguments through every
 * single hook signature.
 *
 * Here, `ApiScopeProvider` derives scope from the *route* alone and depends on
 * nothing. Data hooks read it; the rich console scope is built on top of both
 * and is never read by the API layer.
 */
export type ApiScope = {
  /** Organization id/handle. Undefined until the route resolves it. */
  orgId?: string;
  /** Project id, when the route is inside a project. */
  projectId?: string;
};

export const ApiScopeContext = createContext<ApiScope>({});

/**
 * Resolves the active scope for a hook. An explicit argument always wins over
 * the route-derived value, which lets a page render a resource from another
 * project (a picker, a cross-project link) without breaking the default case.
 *
 * The returned `org` is branded and can be `undefined`; passing it to
 * `enabled: !!org` is the one gate every scoped query needs.
 */
export const useApiScope = (
  overrides: ApiScope = {}
): { org?: OrgScope; orgId?: string; projectId?: string } => {
  const scope = useContext(ApiScopeContext);
  const orgId = overrides.orgId ?? scope.orgId;
  const projectId = overrides.projectId ?? scope.projectId;

  return useMemo(
    () => ({ org: orgScope(orgId), orgId, projectId }),
    [orgId, projectId]
  );
};

/**
 * Asserts scope inside a `queryFn`. By the time a `queryFn` runs, `enabled`
 * has already guaranteed scope exists — this narrows the type for TypeScript
 * and fails loudly if a hook ever forgets its `enabled` gate, instead of
 * silently issuing an unscoped request the server would reject.
 */
export const requireScope = (orgId: string | undefined, operation: string): string => {
  if (!orgId) {
    throw new Error(
      `${operation} requires an organization scope but none was resolved. ` +
        'This indicates a missing `enabled` guard on the query.'
    );
  }
  return orgId;
};

/**
 * Asserts scope inside a `mutationFn`, returning the org id so it can be passed
 * straight to the endpoint.
 *
 * The mutation counterpart to `requireScope`, and deliberately a different kind
 * of failure. A query has `enabled` to hold it back, so a missing scope there is
 * a programming error and throws a bare `Error`. A mutation has no such gate:
 * the user can click a button while the route is still resolving, which is
 * reachable at runtime rather than a bug. So this throws the same `ApiError`
 * every other failure in this layer produces — components keep one error type,
 * and the global mutation handler can surface it — instead of letting an
 * unscoped write reach the server with no `X-Org-Id` at all.
 */
export const requireOrgScope = (
  orgId: string | undefined,
  operation: string
): string => {
  if (!orgId) {
    throw new ApiError('No organization is selected.', {
      kind: 'http',
      status: 0,
      code: ClientErrorCode.CLIENT_MISSING_ORG_SCOPE,
      operation,
    });
  }
  return orgId;
};
