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

import { http as mswHttp, HttpResponse, type HttpHandler } from 'msw';

import type { Schema } from '../../api/core/spec';
import { apiUrl } from './apiBase';

/**
 * Handler builders for platform-api endpoints.
 *
 * There are deliberately **no default handlers**: `onUnhandledRequest: 'error'`
 * means a test must declare the endpoints it exercises, which keeps each test
 * readable on its own and stops one shared fake backend from quietly deciding
 * what every test asserts against. These builders exist so declaring them costs
 * a line rather than fifteen.
 */

type Method = 'get' | 'post' | 'put' | 'patch' | 'delete';

/** The spec's paginated collection envelope, which every list endpoint returns. */
export type ListEnvelope<T> = {
  count: number;
  list: T[];
  pagination: Schema<'Pagination'>;
};

/** Wraps items in the collection envelope. */
export const listEnvelope = <T>(
  items: T[],
  { offset = 0, limit = 20, total = items.length } = {}
): ListEnvelope<T> => ({
  count: items.length,
  list: items,
  pagination: { total, offset, limit },
});

/* -------------------------------------------------------------------------- */
/* Recording what reached the wire                                            */
/* -------------------------------------------------------------------------- */

export type RecordedRequest = {
  method: string;
  url: URL;
  /** Raw body text. Empty string when the request had none. */
  body: string;
  headers: Headers;
  /** Query parameters, for asserting paging and filters. */
  params: URLSearchParams;
};

export type Recorder = {
  calls: RecordedRequest[];
  /** The most recent matching request, or undefined if none was made. */
  last: () => RecordedRequest | undefined;
  /** Number of matching requests so far. */
  count: () => number;
  /** Internal: used by the builders below. */
  capture: (request: Request) => Promise<void>;
};

/**
 * Collects the requests a handler received.
 *
 * Assertions about *what was sent* — org header, paging parameters, request
 * body — need the request itself, and MSW gives no history of its own. Create
 * one per test and pass it to any builder below.
 */
export const recorder = (): Recorder => {
  const calls: RecordedRequest[] = [];

  return {
    calls,
    count: () => calls.length,
    last: () => calls.at(-1),
    capture: async (request: Request) => {
      const url = new URL(request.url);
      calls.push({
        body: await request.clone().text(),
        headers: request.headers,
        method: request.method,
        params: url.searchParams,
        url,
      });
    },
  };
};

type WithRecorder = { record?: Recorder };

/* -------------------------------------------------------------------------- */
/* Success handlers                                                           */
/* -------------------------------------------------------------------------- */

/**
 * A paginated collection that honours `limit`, `offset` and `query` the way the
 * real endpoints do.
 *
 * Serving the whole array regardless of parameters is the trap here: a page that
 * forgets to send `limit` would still render correctly, so the test would pass
 * while server-side paging was quietly broken.
 *
 * `matches` decides what `query` filters on; it defaults to a case-insensitive
 * substring match against `displayName`, mirroring the spec's description.
 */
export const collection = <T extends { displayName?: string }>(
  path: string,
  items: T[],
  options: WithRecorder & { matches?: (item: T, term: string) => boolean } = {}
): HttpHandler =>
  mswHttp.get(apiUrl(path), async ({ request }) => {
    await options.record?.capture(request);

    const params = new URL(request.url).searchParams;
    const term = params.get('query')?.toLowerCase();
    const matches =
      options.matches ??
      ((item: T, value: string) =>
        (item.displayName ?? '').toLowerCase().includes(value));

    const filtered = term ? items.filter((item) => matches(item, term)) : items;
    const offset = Number(params.get('offset') ?? 0);
    const limit = Number(params.get('limit') ?? 20);

    return HttpResponse.json(
      listEnvelope(filtered.slice(offset, offset + limit), {
        limit,
        offset,
        total: filtered.length,
      }) as never
    );
  });

/** A single resource. Use `:param` segments in `path` to match any id. */
export const resource = <T>(
  path: string,
  body: T,
  options: WithRecorder & { method?: Method; status?: number } = {}
): HttpHandler =>
  mswHttp[options.method ?? 'get'](apiUrl(path), async ({ request }) => {
    await options.record?.capture(request);
    return HttpResponse.json(body as never, { status: options.status ?? 200 });
  });

/** A write that echoes back the created or updated resource. */
export const accepts = <T>(
  method: Exclude<Method, 'get'>,
  path: string,
  body: T,
  options: WithRecorder & { status?: number } = {}
): HttpHandler =>
  resource(path, body, {
    ...options,
    method,
    status: options.status ?? (method === 'post' ? 201 : 200),
  });

/** A bodiless success, as deletes and undeploys return. */
export const noContent = (
  method: Exclude<Method, 'get'>,
  path: string,
  options: WithRecorder = {}
): HttpHandler =>
  mswHttp[method](apiUrl(path), async ({ request }) => {
    await options.record?.capture(request);
    return new HttpResponse(null, { status: 204 });
  });

/* -------------------------------------------------------------------------- */
/* Failure handlers                                                           */
/* -------------------------------------------------------------------------- */

/**
 * A failure in the spec's error envelope, so the client parses a real `code`
 * and the test exercises the same path production does.
 */
export const failure = (
  method: Method,
  path: string,
  status: number,
  code: string,
  options: WithRecorder & {
    message?: string;
    errors?: Schema<'FieldError'>[];
    details?: Record<string, unknown>;
    trackingId?: string;
  } = {}
): HttpHandler =>
  mswHttp[method](apiUrl(path), async ({ request }) => {
    await options.record?.capture(request);
    return HttpResponse.json(
      {
        status: 'error',
        code,
        message: options.message ?? 'The request could not be completed.',
        ...(options.errors ? { errors: options.errors } : {}),
        ...(options.details ? { details: options.details } : {}),
        ...(options.trackingId ? { trackingId: options.trackingId } : {}),
      } as never,
      { status }
    );
  });

/**
 * A response that is *not* the spec's envelope — an HTML page from an
 * intermediary. Covers the branch where the client must degrade safely rather
 * than render markup.
 */
export const malformedFailure = (
  method: Method,
  path: string,
  status = 502
): HttpHandler =>
  mswHttp[method](apiUrl(path), () =>
    new HttpResponse(`<html><body>${status}</body></html>`, {
      headers: { 'Content-Type': 'text/html' },
      status,
    })
  );

/** A request that never reaches the server at all. */
export const networkError = (method: Method, path: string): HttpHandler =>
  mswHttp[method](apiUrl(path), () => HttpResponse.error());

/**
 * A response that never arrives, for exercising timeouts and cancellation.
 * The promise is never resolved, so the request stays in flight.
 */
export const neverResponds = (method: Method, path: string): HttpHandler =>
  mswHttp[method](apiUrl(path), () => new Promise<never>(() => {}));
