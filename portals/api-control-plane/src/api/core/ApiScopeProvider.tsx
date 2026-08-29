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

import { type ReactNode, useEffect, useMemo, useRef } from 'react';
import { useQueryClient } from '@tanstack/react-query';

import { orgScope, scopeKey } from './queryKeys';
import { ApiScopeContext, type ApiScope } from './scope';

/**
 * Publishes the active organization and project to the data hooks, and evicts
 * the previous organization's cache when the user switches away from it.
 *
 * Deliberately **props-driven rather than route-reading**: the API layer stays
 * independent of the router, so the same provider works under a different
 * routing setup, in a test, or in Storybook. Whatever owns route params passes
 * them in.
 *
 * Mounting this is not optional. Every scoped query is gated on
 * `enabled: Boolean(org)`, so without a provider above them the context default
 * (`{}`) leaves `org` undefined and **no query ever runs**.
 */
export function ApiScopeProvider({
  orgId,
  projectId,
  children,
}: ApiScope & { children: ReactNode }) {
  const queryClient = useQueryClient();

  // Tracks the organization the cache currently holds data for. A ref rather
  // than state because changing it must not itself trigger a render; this is
  // bookkeeping about the cache, not something the UI reads.
  const cachedOrgRef = useRef<string | undefined>(orgId);

  useEffect(() => {
    const previous = cachedOrgRef.current;
    cachedOrgRef.current = orgId;

    // Only a genuine switch between two organizations should evict anything.
    // The first render (no previous) and a transient undefined during
    // navigation must not drop a cache the user is about to return to.
    if (!previous || !orgId || previous === orgId) return;

    const previousScope = orgScope(previous);
    if (!previousScope) return;

    // Every key for an organization sits under its scope prefix, so one removal
    // drops that tenant's entire cache and nothing else. Global entries (the
    // organization list backing the switcher itself) are filed outside this
    // prefix and survive deliberately.
    queryClient.removeQueries({ queryKey: scopeKey(previousScope) });
  }, [orgId, queryClient]);

  const value = useMemo<ApiScope>(() => ({ orgId, projectId }), [orgId, projectId]);

  return (
    <ApiScopeContext.Provider value={value}>{children}</ApiScopeContext.Provider>
  );
}
