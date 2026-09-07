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

import { MutationCache, QueryCache, QueryClient } from '@tanstack/react-query';

import { ApiError, isApiError } from './errors';

/**
 * Freshness tiers. Naming the tiers rather than sprinkling magic numbers keeps
 * ~200 operations consistent, and makes the intent reviewable at a glance.
 */
export const staleTimes = {
  /** Live state the user is watching change — deployments, gateway status. */
  realtime: 0,
  /** Normal resources the user edits: APIs, projects, applications. */
  standard: 30_000,
  /** Rarely-changing org structure: organizations, projects list, gateways. */
  stable: 5 * 60_000,
  /** Effectively immutable per session: policy catalog, provider templates. */
  static: 30 * 60_000,
} as const;

/**
 * How long an unused entry survives in memory. Longer than `staleTime` on
 * purpose: it is what makes back-navigation instant (cached data renders while
 * a background refetch runs) instead of flashing a spinner.
 */
export const GC_TIME_MS = 10 * 60_000;

const MAX_RETRIES = 3;

/**
 * Retry only what can plausibly succeed on a second attempt. `ApiError.isRetryable`
 * is the single source of truth: 5xx, timeouts, network failures, 408 and 429 —
 * never 401/403/404/409/422, which fail identically every time and only delay
 * the message the user needs.
 *
 * The old client used a blanket `retry: 2`, which turned a 403 into three
 * requests and a three-second wait before the user saw "forbidden".
 */
export const shouldRetry = (failureCount: number, error: unknown): boolean => {
  if (failureCount >= MAX_RETRIES) return false;
  if (!isApiError(error)) return false;
  return error.isRetryable;
};

/**
 * Exponential backoff with full jitter. Jitter matters here specifically
 * because a console dashboard fires many queries at once: without it, a
 * recovering backend gets all of them retrying in lockstep at exactly 1s, 2s,
 * 4s: a self-inflicted thundering herd against a service that is already
 * struggling.
 */
export const retryDelay = (attemptIndex: number): number => {
  const ceiling = Math.min(1_000 * 2 ** attemptIndex, 30_000);
  return Math.round(ceiling * (0.5 + Math.random() * 0.5));
};

/**
 * Marks a mutation whose failures its own caller already puts on screen —
 * bound onto form fields, shown in a panel. The global snackbar skips these,
 * because a failure that has a home on screen reported *twice* reads worse
 * than either report alone.
 *
 * Opt-in on purpose: a mutation that says nothing still gets the snackbar, so
 * forgetting to handle an error can never mean swallowing it.
 */
export const HANDLED_LOCALLY = { handlesErrors: true } as const;

export type QueryClientHandlers = {
  /** Called for every unhandled *mutation* error — wire to the snackbar. */
  onMutationError?: (error: ApiError) => void;
  /** Called for background query errors that already have data on screen. */
  onBackgroundError?: (error: ApiError) => void;
};

export const createQueryClient = (handlers: QueryClientHandlers = {}) =>
  new QueryClient({
    /**
     * Global handlers belong on the caches, not in `defaultOptions`.
     *
     * A mutation's own `onError` *replaces* `defaultOptions.mutations.onError`
     * rather than running alongside it; so putting the global handler there
     * would silence it for exactly the mutations that define one, which is
     * every mutation doing an optimistic rollback. `MutationCache.onError`
     * fires for all of them regardless — except the ones that opted out with
     * `HANDLED_LOCALLY`, which surface the failure themselves.
     */
    mutationCache: new MutationCache({
      onError: (error, _variables, _onMutateResult, mutation) => {
        if (mutation.meta?.handlesErrors === true) return;
        if (isApiError(error)) handlers.onMutationError?.(error);
      },
    }),

    /**
     * Query errors reach here only when the query already has data on screen.
     * A background refetch that failed. A first-load failure is surfaced by the
     * component through `isError`, or thrown to an error boundary; reporting
     * both here would double up.
     */
    queryCache: new QueryCache({
      onError: (error, query) => {
        if (isApiError(error) && query.state.data !== undefined) {
          handlers.onBackgroundError?.(error);
        }
      },
    }),

    defaultOptions: {
      queries: {
        staleTime: staleTimes.standard,
        gcTime: GC_TIME_MS,
        retry: shouldRetry,
        retryDelay,
        // Refetch when the user comes back to a tab that has been idle, but
        // only if the data is actually stale; with `staleTime` set per tier,
        // this costs nothing for stable resources and keeps volatile ones live.
        refetchOnWindowFocus: true,
        // Reconnect refetch is the cheap fix for the laptop-lid case: data
        // fetched before a network drop is silently wrong until refreshed.
        refetchOnReconnect: true,
        refetchOnMount: true,
        // No global `placeholderData`; use it only for pagination-specific queries
        // so route changes do not show stale data from the previous key.
      },
      mutations: {
        // A mutation is not idempotent. A timeout means the request may have
        // reached the server and succeeded, so retrying it risks a duplicate
        // resource. Only a request that provably never left is retried, and
        // only once.
        retry: (failureCount, error) =>
          failureCount < 1 &&
          isApiError(error) &&
          (error.kind === 'network'),
        retryDelay,
      },
    },
  });

export type { QueryClient };
