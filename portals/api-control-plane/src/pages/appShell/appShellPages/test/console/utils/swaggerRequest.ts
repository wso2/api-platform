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

import {
  normalizeMethod,
  rawFormatFor,
  type ConsoleRequest,
  type KeyValueRow,
} from '../../utils/types';

/**
 * Converts a Swagger UI request object into the console's `ConsoleRequest`
 * format. This one-way conversion keeps the cURL view synchronized with the
 * values entered in the Console view's try-out form.
 *
 * The implementation is intentionally defensive because Swagger UI's request
 * object is an Immutable.js structure that is not part of its public API. Its
 * shape may change between minor versions, so every field is treated as
 * optional. The function returns `undefined` when a complete request cannot be
 * constructed, leaving the previously displayed command unchanged rather
 * than showing an inaccurate one.
 *
 * The conversion is pure and depends on Immutable.js only through guarded
 * calls to `.get` and `.toJS`. This also allows it to be unit-tested with
 * ordinary JavaScript objects.
 */

/** The shape we hope for, without asserting swagger actually provides it. */
type ImmutableLike = {
  get?: (key: string) => unknown;
  toJS?: () => unknown;
};

const isImmutableLike = (value: unknown): value is ImmutableLike =>
  typeof value === 'object' && value !== null && typeof (value as ImmutableLike).get === 'function';

/** Reads a key from either an Immutable Map or a plain object. */
const read = (source: unknown, key: string): unknown => {
  if (isImmutableLike(source)) return source.get?.(key);
  if (typeof source === 'object' && source !== null) {
    return (source as Record<string, unknown>)[key];
  }
  return undefined;
};

/** Flattens an Immutable Map or plain object of headers into string pairs. */
const readHeaders = (source: unknown): Record<string, string> => {
  const raw = isImmutableLike(source) && typeof source.toJS === 'function' ? source.toJS() : source;

  if (typeof raw !== 'object' || raw === null) return {};

  const headers: Record<string, string> = {};
  Object.entries(raw as Record<string, unknown>).forEach(([name, value]) => {
    // Swagger stores absent headers as null/undefined rather than removing the
    // key, so an unfiltered pass would emit `-H 'Accept: undefined'`.
    if (typeof value === 'string' && name.trim() !== '') headers[name] = value;
  });
  return headers;
};

/** Gets a header value by name, case-insensitively. */
const headerValue = (headers: Record<string, string>, name: string): string | undefined =>
  Object.entries(headers).find(([key]) => key.toLowerCase() === name.toLowerCase())?.[1];

/** The body as text, whatever swagger happened to store it as. */
const readBody = (source: unknown): string => {
  const body = read(source, 'body');
  if (typeof body === 'string') return body;
  if (body === undefined || body === null) return '';
  // A structured body (swagger keeps request-body forms as objects) is printed
  // rather than dropped, so the cURL view shows what would actually be sent.
  try {
    return JSON.stringify(body, null, 2);
  } catch {
    return '';
  }
};

let rowSequence = 0;
/**
 * Row ids for synced rows.
 *
 * Monotonic rather than content-derived: two headers can share a name while the
 * user is mid-edit, and duplicate React keys would make one of them
 * un-editable.
 */
const nextRowId = (prefix: string): string => {
  rowSequence += 1;
  return `${prefix}-${rowSequence}`;
};

const toRows = (entries: [string, string][], prefix: string, secretName?: string): KeyValueRow[] =>
  entries.map(([name, value]) => {
    // Mark credentials case-insensitively so the cURL panel masks them by
    // default, whether they appear as headers or query parameters.
    const isCredential = Boolean(secretName && name.toLowerCase() === secretName.toLowerCase());

    return {
      auto: isCredential,
      enabled: true,
      id: nextRowId(prefix),
      name,
      secret: isCredential,
      value,
    };
  });

/**
 * Splits a full URL into the path and query relative to a gateway base.
 *
 * Returns `undefined` when the URL is unparseable or points somewhere other
 * than the base — a request aimed at another host is not this API's request,
 * and syncing it would put a foreign URL in the user's clipboard.
 */
export const splitAgainstBase = (
  url: string,
  baseUrl: string,
): { path: string; query: [string, string][] } | undefined => {
  const base = baseUrl.trim().replace(/\/+$/, '');
  let parsed: URL;
  let parsedBase: URL;
  try {
    parsed = new URL(url);
    parsedBase = new URL(base);
  } catch {
    return undefined;
  }

  if (parsed.origin !== parsedBase.origin) return undefined;

  const basePath = parsedBase.pathname.replace(/\/+$/, '');
  if (
    basePath !== '' &&
    parsed.pathname !== basePath &&
    !parsed.pathname.startsWith(`${basePath}/`)
  )
    return undefined;

  const path = parsed.pathname.slice(basePath.length) || '/';
  const query: [string, string][] = [];
  parsed.searchParams.forEach((value, name) => query.push([name, value]));

  return { path, query };
};

/**
 * Builds a `ConsoleRequest` from a swagger request object.
 *
 * `secretName` marks which header *or query parameter* carries the test key, so
 * the cURL panel knows what to mask — swagger has no concept of a credential,
 * so this is the only place that knowledge can be attached. One name covers
 * both lists because a given API's key travels in one place, never both.
 */
export const fromSwaggerRequest = (
  raw: unknown,
  baseUrl: string,
  secretName?: string,
): ConsoleRequest | undefined => {
  const url = read(raw, 'url');
  if (typeof url !== 'string' || url.trim() === '') return undefined;

  const split = splitAgainstBase(url, baseUrl);
  if (!split) return undefined;

  // A verb the console does not offer is not a request it can describe, the
  // same as a URL pointing somewhere other than the gateway above.
  const method = normalizeMethod(read(raw, 'method') as string | undefined);
  if (!method) return undefined;

  const headers = readHeaders(read(raw, 'headers'));
  const body = readBody(raw);

  return {
    method,
    baseUrl: baseUrl.trim().replace(/\/+$/, ''),
    path: split.path,
    queryParams: toRows(split.query, 'q', secretName),
    headers: toRows(Object.entries(headers), 'h', secretName),
    bodyMode: body.trim() === '' ? 'none' : 'raw',
    // Use the sent Content-Type, not the operation's declared format.
    rawFormat: rawFormatFor(headerValue(headers, 'content-type')),
    body,
    formFields: [],
  };
};
