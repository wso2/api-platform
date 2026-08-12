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
 * Query-key construction.
 *
 * Two rules make the whole cache predictable, and both are enforced by
 * construction here rather than by convention:
 *
 * 1. **Every key is prefixed by the scope that authorizes it.** A key is
 *    `['platform', orgId, ...resource]`. Because TanStack Query invalidation is
 *    prefix-based, `invalidateQueries({ queryKey: scopeKey(orgId) })` drops
 *    exactly one organization's cached data and nothing else. It also makes
 *    cross-tenant collision structurally impossible: two orgs can never share a
 *    cache entry, because `orgId` sits above the resource in the key.
 *
 * 2. **A key is never built from a possibly-empty string.** The old layer wrote
 *    `queryKeys.projects(orgHandle || '')`, so every hook that ran before its
 *    org resolved wrote into the *same* `['projects', '']` bucket; one
 *    organization's list could be served to another. Here, scope is a branded
 *    type that can only be produced from a non-empty id, so that key cannot be
 *    expressed at all.
 */

/** A validated, non-empty organization id. */
export type OrgScope = string & { readonly __brand: 'OrgScope' };

/**
 * Narrows an id into an `OrgScope`, or `undefined` when it is not yet known.
 * Hooks pass the result straight to `enabled`, so a query with no scope simply
 * does not run — rather than running against a degenerate key.
 */
export const orgScope = (orgId: string | undefined | null): OrgScope | undefined =>
  orgId ? (orgId as OrgScope) : undefined;

/** Root of every cached entry. */
export const ROOT_KEY = 'platform' as const;

/** All queries for one organization. Invalidate this on org switch. */
export const scopeKey = (org: OrgScope) => [ROOT_KEY, org] as const;

/**
 * Normalizes a filter/param object into a stable key segment.
 *
 * TanStack Query hashes keys deterministically for object *values*, but two
 * filters that differ only by an explicit `undefined` (`{ q: undefined }` vs
 * `{}`) still hash differently — which silently doubles cache entries and
 * re-fetches. Stripping empty values first means `?q=` and no `q` at all share
 * one entry, matching what the server actually does.
 */
export const normalizeParams = <T extends Record<string, unknown>>(
  params: T | undefined
): Record<string, unknown> | undefined => {
  if (!params) return undefined;
  const entries = Object.entries(params)
    .filter(([, value]) => value !== undefined && value !== null && value !== '')
    .sort(([a], [b]) => a.localeCompare(b));
  return entries.length ? Object.fromEntries(entries) : undefined;
};

/**
 * Builds a resource's key factory. Each resource module calls this once, which
 * guarantees all ~30 resources share one key shape and one invalidation story.
 *
 *   const restApiKeys = createResourceKeys('restApis');
 *   restApiKeys.all(org)            // ['platform', org, 'restApis']
 *   restApiKeys.list(org, filters)  // ['platform', org, 'restApis', 'list', {…}]
 *   restApiKeys.detail(org, id)     // ['platform', org, 'restApis', 'detail', id]
 *   restApiKeys.child(org, id, 'deployments')
 *
 * Invalidating `all(org)` covers every list, detail and child of the resource —
 * the correct default after a mutation, because a create/delete can change
 * counts and pagination on pages you have not visited yet.
 */
export const createResourceKeys = <const Name extends string>(name: Name) => ({
  name,

  /** Everything cached for this resource in this org. */
  all: (org: OrgScope) => [...scopeKey(org), name] as const,

  /** Every list variant, regardless of filters. */
  lists: (org: OrgScope) => [...scopeKey(org), name, 'list'] as const,

  /** One list variant, identified by its normalized filters. */
  list: (org: OrgScope, params?: Record<string, unknown>) =>
    [...scopeKey(org), name, 'list', normalizeParams(params)] as const,

  /** Every detail entry for this resource. */
  details: (org: OrgScope) => [...scopeKey(org), name, 'detail'] as const,

  /** One resource instance. */
  detail: (org: OrgScope, id: string) =>
    [...scopeKey(org), name, 'detail', id] as const,

  /**
   * A sub-resource of one instance (`/rest-apis/{id}/deployments`). Keying it
   * *under* the parent detail means deleting the parent invalidates its
   * children in one call, with no bookkeeping.
   */
  child: (org: OrgScope, id: string, child: string, params?: Record<string, unknown>) =>
    [...scopeKey(org), name, 'detail', id, child, normalizeParams(params)] as const,
});

export type ResourceKeys = ReturnType<typeof createResourceKeys>;
