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

import { describe, expect, it, vi } from 'vitest';

import { ApiError } from './errors';
import {
  createQueryClient,
  HANDLED_LOCALLY,
  retryDelay,
  shouldRetry,
  staleTimes,
} from './queryClient';

/**
 * The retry policy is where a wrong answer is expensive in both directions:
 * retrying an unretryable failure delays the message the user needs, and
 * retrying in lockstep across many queries can keep a recovering backend down.
 */

const httpError = (status: number) => new ApiError('failed', { kind: 'http', status });

describe('shouldRetry — which failures are worth attempting again', () => {
  it.each([
    ['a server error', httpError(503)],
    ['a network failure', new ApiError('offline', { kind: 'network' })],
    ['a timeout', new ApiError('slow', { kind: 'timeout' })],
    ['rate limiting', httpError(429)],
  ])('retries %s', (_label, error) => {
    expect(shouldRetry(0, error)).toBe(true);
  });

  it.each([
    ['unauthorized', httpError(401)],
    ['forbidden', httpError(403)],
    ['not found', httpError(404)],
    ['a conflict', httpError(409)],
  ])('does not retry %s — the previous layer retried these three times over', (_label, error) => {
    expect(shouldRetry(0, error)).toBe(false);
  });

  it('stops after three attempts even for a retryable failure', () => {
    const error = httpError(503);

    expect(shouldRetry(0, error)).toBe(true);
    expect(shouldRetry(2, error)).toBe(true);
    expect(shouldRetry(3, error)).toBe(false);
    expect(shouldRetry(9, error)).toBe(false);
  });

  it('does not retry an error that never came from the transport', () => {
    // A bug thrown inside a queryFn or a `select` is not a transport failure,
    // and running it three more times will not fix it.
    expect(shouldRetry(0, new TypeError('cannot read property of undefined'))).toBe(false);
  });
});

describe('retryDelay — backoff that does not synchronise across queries', () => {
  /** The un-jittered ceiling the implementation backs off toward. */
  const ceiling = (attempt: number) => Math.min(1_000 * 2 ** attempt, 30_000);

  it.each([0, 1, 2, 3, 5, 10])(
    'stays within half of the ceiling and the ceiling itself on attempt %i',
    (attempt) => {
      // Sampled repeatedly because the delay is randomised on every call.
      for (let i = 0; i < 200; i += 1) {
        const delay = retryDelay(attempt);

        expect(delay).toBeGreaterThanOrEqual(ceiling(attempt) * 0.5 - 1);
        expect(delay).toBeLessThanOrEqual(ceiling(attempt));
      }
    }
  );

  it('grows with each successive attempt', () => {
    // Any individual pair of samples may not be ordered, so compare the
    // observed minimum of each attempt over many samples.
    const minimum = (attempt: number) =>
      Math.min(...Array.from({ length: 200 }, () => retryDelay(attempt)));
    expect(minimum(0)).toBeLessThan(minimum(1));
    expect(minimum(1)).toBeLessThan(minimum(2));
  });

  it('caps at thirty seconds, so a long outage does not produce absurd waits', () => {
    expect(retryDelay(20)).toBeLessThanOrEqual(30_000);
  });

  it('produces different delays for concurrent queries, spreading a recovery herd', () => {
    // Without jitter, a dashboard firing eight queries retries all of them at
    // exactly 1s, 2s and 4s — hitting a struggling backend in lockstep.
    const samples = new Set(Array.from({ length: 50 }, () => retryDelay(3)));

    expect(samples.size).toBeGreaterThan(1);
  });
});

describe('mutation retry policy', () => {
  const mutationRetry = () => {
    const options = createQueryClient().getDefaultOptions().mutations;
    return options?.retry as (count: number, error: unknown) => boolean;
  };

  it('retries a network failure once, since the request never reached the server', () => {
    const error = new ApiError('offline', { kind: 'network' });

    expect(mutationRetry()(0, error)).toBe(true);
    expect(mutationRetry()(1, error)).toBe(false);
  });

  it('never retries a timeout, because the request may have arrived and succeeded', () => {
    // The distinction from a network failure: nothing left the client there, so
    // a retry is free. A timeout only proves no response came back in time —
    // the write may already have been applied, and retrying duplicates it.
    expect(mutationRetry()(0, new ApiError('slow', { kind: 'timeout' }))).toBe(false);
  });

  it('never retries a server error, because a POST is not idempotent', () => {
    // A 500 may still have created the resource; a blind retry risks two.
    expect(mutationRetry()(0, httpError(500))).toBe(false);
  });

  it('never retries a validation failure or a conflict', () => {
    expect(mutationRetry()(0, httpError(400))).toBe(false);
    expect(mutationRetry()(0, httpError(409))).toBe(false);
  });
});

describe('query defaults', () => {
  it('reports every mutation failure to the handler, so none can fail silently', async () => {
    // The previous layer had no onError at all: a failed mutation showed
    // nothing unless the calling component happened to handle it.
    const seen: ApiError[] = [];
    const client = createQueryClient({ onMutationError: (error) => seen.push(error) });

    await client
      .getMutationCache()
      .build(client, { mutationFn: () => Promise.reject(httpError(500)), retry: false })
      .execute(undefined)
      .catch(() => undefined);

    expect(seen).toHaveLength(1);
    expect(seen[0].status).toBe(500);
  });

  it('still reports a failure when the mutation defines its own onError', async () => {
    // The reason the handler lives on the MutationCache rather than in
    // defaultOptions: a mutation's own onError *replaces* the default one, and
    // every mutation doing an optimistic rollback defines one. Putting the
    // global handler in defaultOptions would silence it for exactly those.
    const seen: ApiError[] = [];
    const rollback = vi.fn();
    const client = createQueryClient({ onMutationError: (error) => seen.push(error) });

    await client
      .getMutationCache()
      .build(client, {
        mutationFn: () => Promise.reject(httpError(500)),
        onError: rollback,
        retry: false,
      })
      .execute(undefined)
      .catch(() => undefined);

    expect(rollback).toHaveBeenCalledTimes(1);
    expect(seen).toHaveLength(1);
  });

  it('stays quiet for a mutation that reports its own failures', async () => {
    // The creation wizard binds a rejection onto the form fields that caused
    // it. Without the opt-out the same failure would also fly past as a
    // snackbar, which reads as two unrelated problems rather than one.
    const seen: ApiError[] = [];
    const client = createQueryClient({ onMutationError: (error) => seen.push(error) });

    await client
      .getMutationCache()
      .build(client, {
        meta: HANDLED_LOCALLY,
        mutationFn: () => Promise.reject(httpError(409)),
        retry: false,
      })
      .execute(undefined)
      .catch(() => undefined);

    expect(seen).toHaveLength(0);
  });

  it('keeps unused data in memory longer than it stays fresh, so back-navigation is instant', () => {
    // The gap between staleTime and gcTime is what lets a revisited page render
    // from cache while it refetches in the background.
    const defaults = createQueryClient().getDefaultOptions().queries;

    expect(defaults?.gcTime).toBeGreaterThan(staleTimes.standard);
  });

  it('orders the freshness tiers from most to least volatile', () => {
    expect(staleTimes.realtime).toBeLessThan(staleTimes.standard);
    expect(staleTimes.standard).toBeLessThan(staleTimes.stable);
    expect(staleTimes.stable).toBeLessThan(staleTimes.static);
  });

  it('refetches on reconnect, so data fetched before a network drop is corrected', () => {
    const defaults = createQueryClient().getDefaultOptions().queries;

    expect(defaults?.refetchOnReconnect).toBe(true);
  });

  it('sets no global placeholderData, so no query renders another key’s data', () => {
    // Keep paging local to the query; a global default can show the previous tenant's data after a route change.
    const defaults = createQueryClient().getDefaultOptions().queries;

    expect(defaults?.placeholderData).toBeUndefined();
  });
});
