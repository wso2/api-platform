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

import { QueryClient } from '@tanstack/react-query';

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

export type QueryClientHandlers = {
  /** Called for every unhandled *mutation* error — wire to the snackbar. */
  onMutationError?: (error: ApiError) => void;
  /** Called for background query errors that already have data on screen. */
  onBackgroundError?: (error: ApiError) => void;
};

export const createQueryClient = (handlers: QueryClientHandlers = {}) =>
  new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: staleTimes.standard,
        gcTime: GC_TIME_MS,
        retry: shouldRetry,
        retryDelay,
        // Refetch when the user comes back to a tab that has been idle, but
        // only if the data is actually stale — with `staleTime` set per tier,
        // this costs nothing for stable resources and keeps volatile ones live.
        refetchOnWindowFocus: true,
        // Reconnect refetch is the cheap fix for the laptop-lid case: data
        // fetched before a network drop is silently wrong until refreshed.
        refetchOnReconnect: true,
        refetchOnMount: true,
        // Keep the previous page's rows on screen while the next page loads,
        // instead of collapsing the table to a spinner on every page change.
        placeholderData: (previous: unknown) => previous,
      },
      mutations: {
        // A mutation is not idempotent; retrying a failed POST risks creating
        // two resources. Retry only genuine transport failures, and only once.
        retry: (failureCount, error) =>
          failureCount < 1 &&
          isApiError(error) &&
          (error.kind === 'network' || error.kind === 'timeout'),
        retryDelay,
        onError: (error) => {
          if (isApiError(error)) handlers.onMutationError?.(error);
        },
      },
    },
  });

export type { QueryClient };
