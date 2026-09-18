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

import type { ApiKeyLocation } from './apiKeyAuth';

/**
 * Represents the request shared by the test console and cURL builder.
 *
 * Maintaining a single request model keeps the generated cURL command
 * consistent with the request executed by the console. The console populates
 * this model from Swagger UI's filled-in form through its `mutatedRequestFor`
 * selector, while the cURL view allows direct editing. In both cases,
 * `toCurl` renders this model, ensuring that the command accurately reflects
 * the request executed by the console.
 */

/**
 * Methods the console offers. Ordered as the method picker lists them.
 * */
export const HTTP_METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS'] as const;

export type HttpMethod = (typeof HTTP_METHODS)[number];

/**
 * Normalizes a method string from a spec, a URL or a swagger selector.
 *
 * OpenAPI path-item keys are lowercase, while downstream comparisons, map keys,
 * and matchers use uppercase.
 *
 * Unsupported verbs return `undefined` rather than defaulting to `GET`. This
 * prevents unsupported operations, such as `connect`, from being displayed or
 * sent as `GET` and keeps the cURL output consistent with the executed request.
 *
 * An absent or blank value defaults to `GET`.
 */
export const normalizeMethod = (raw: string | undefined): HttpMethod | undefined => {
  const upper = (raw ?? '').trim().toUpperCase();
  if (upper === '') return 'GET';
  return (HTTP_METHODS as readonly string[]).includes(upper) ? (upper as HttpMethod) : undefined;
};

/**
 * One row of the query-parameter or header table.
 *
 * `id` exists because rows are reorderable and independently editable: keying
 * React rows by name would make two blank rows collide, and renaming a header
 * would remount its inputs and drop focus mid-keystroke.
 */
export type KeyValueRow = {
  id: string;
  name: string;
  value: string;
  /** Unchecked rows are excluded from both the request and the command. */
  enabled: boolean;
  /**
   * Rows the console supplied rather than the user, the test key header.
   * Shown with an "Auto" badge; still editable, since the API's real key header
   * name is not discoverable from the spec and only the user knows it.
   */
  auto?: boolean;
  /**
   * The value is a live credential. Masked in every display surface, and only
   * ever rendered in full on an explicit reveal or copy.
   */
  secret?: boolean;
};

/**
 * How the request carries its body.
 *
 * `form-data` is `multipart/form-data` and `url-encoded` is
 * `application/x-www-form-urlencoded`; both are edited as key/value pairs
 * rather than as text, because that is what they are on the wire.
 */
export const BODY_MODES = ['none', 'raw', 'form-data', 'url-encoded'] as const;

export type BodyMode = (typeof BODY_MODES)[number];

/** Wire format of a raw body. Decides the Content-Type and the validation. */
export const RAW_FORMATS = ['json', 'xml', 'text'] as const;

export type RawFormat = (typeof RAW_FORMATS)[number];

/**
 * A request the console can execute or print.
 *
 * `baseUrl` and `path` are separate rather than one URL string because they
 * have different owners: the base comes from the selected gateway's endpoint
 * and is not the user's to edit, while the path is the operation being tested.
 * Keeping them apart is what lets the URL field grey out the base and
 * emphasise the path, and what stops a user's edit from silently retargeting
 * the request at another host.
 */
export type ConsoleRequest = {
  method: HttpMethod;
  /** The selected gateway's invoke URL. Read-only, no trailing slash. */
  baseUrl: string;
  /** Path within the API, leading slash included, e.g. `/payments`. */
  path: string;
  queryParams: KeyValueRow[];
  headers: KeyValueRow[];
  bodyMode: BodyMode;
  /** Format of `body` while `bodyMode` is `raw`. Ignored otherwise. */
  rawFormat: RawFormat;
  /** Raw body text. Kept as typed, so invalid JSON survives for correction. */
  body: string;
  /**
   * Fields of a `form-data` or `url-encoded` body.
   *
   * Held separately from `body` rather than serialised into it, so switching
   * between the two encodings — or back to raw and forward again — does not
   * destroy what the user typed. The serialisation happens at the point of use.
   */
  formFields: KeyValueRow[];
};

/** Rows that actually travel — enabled, and with a name worth sending. */
export const activeRows = (rows: readonly KeyValueRow[]): KeyValueRow[] =>
  rows.filter((row) => row.enabled && row.name.trim() !== '');

/**
 * Whether the request would actually carry a body.
 *
 * The mode alone is not enough: an empty raw body must not produce a `-d ''`
 * flag that turns a GET into a request with a zero-length entity, and a
 * key/value body with nothing filled in yet is not a body either.
 */
export const hasBody = (request: ConsoleRequest): boolean => {
  if (request.bodyMode === 'raw') return request.body.trim() !== '';
  if (request.bodyMode === 'form-data' || request.bodyMode === 'url-encoded') {
    return activeRows(request.formFields).length > 0;
  }
  return false;
};

/** Content-Type implied by each raw format. */
const RAW_CONTENT_TYPES: Record<RawFormat, string> = {
  json: 'application/json',
  text: 'text/plain',
  xml: 'application/xml',
};

/** Maps a Content-Type to a known raw format, defaulting to `json`. */
export const rawFormatFor = (mediaType: string | undefined): RawFormat => {
  const base = (mediaType ?? '').split(';')[0].trim().toLowerCase();
  if (base === '') return 'json';
  if (base === 'application/xml' || base === 'text/xml' || base.endsWith('+xml')) return 'xml';
  if (base === 'application/json' || base.endsWith('+json')) return 'json';
  if (base.startsWith('text/')) return 'text';
  return 'json';
};

/**
 * The Content-Type the body implies, or `undefined` when nothing should be set.
 *
 * `form-data` deliberately returns `undefined`: curl generates
 * `multipart/form-data` *with a boundary parameter* itself for `-F`, and a
 * Content-Type we set by hand would have no boundary, the server would then
 * fail to split the body it was handed. Declaring less is what makes that
 * request work.
 *
 * A body with no content also returns `undefined`, so switching to a mode and
 * typing nothing does not announce a payload that is not there.
 */
export const contentTypeFor = (request: ConsoleRequest): string | undefined => {
  if (!hasBody(request)) return undefined;
  if (request.bodyMode === 'raw') return RAW_CONTENT_TYPES[request.rawFormat];
  if (request.bodyMode === 'url-encoded') return 'application/x-www-form-urlencoded';
  return undefined;
};

/**
 * How the body should be described to the user.
 *
 * A discriminator rather than a label: the wording is translated at the point
 * of display, and returning English here would bake it into this layer.
 */
export const bodyKindOf = (request: ConsoleRequest): 'none' | RawFormat | BodyMode =>
  !hasBody(request) ? 'none' : request.bodyMode === 'raw' ? request.rawFormat : request.bodyMode;

/**
 * Re-points a request at the current gateway and credential.
 *
 * Derived rather than synced. An earlier version kept the request in state and
 * corrected it from an effect whenever the gateway or key changed; that effect
 * ended up fighting the Console view's own sync, each one retriggering the
 * other in an unbounded render loop. Computing the target on read makes the
 * conflict impossible to express: there is one owner for the base URL and the
 * credential row, and it is whatever the page currently holds.
 *
 * `placement` decides which list the credential lands in, because the
 * `api-key-auth` policy can ask for the key in a header *or* a query parameter.
 * Auto rows are stripped from **both** lists before the new one is added, so
 * switching placement — or moving to an API that needs no key at all — leaves
 * no stale credential behind in the list it used to occupy. The user's own rows
 * are untouched either way.
 */
export const withTarget = (
  request: ConsoleRequest,
  baseUrl: string,
  autoRows: readonly KeyValueRow[],
  placement: ApiKeyLocation = 'header',
): ConsoleRequest => ({
  ...request,
  baseUrl: baseUrl.trim().replace(/\/+$/, ''),
  headers: [
    ...request.headers.filter((row) => !row.auto),
    ...(placement === 'header' ? autoRows : []),
  ],
  queryParams: [
    ...request.queryParams.filter((row) => !row.auto),
    ...(placement === 'query' ? autoRows : []),
  ],
});

/** An empty request, for a console with no operation selected yet. */
export const emptyRequest = (baseUrl: string): ConsoleRequest => ({
  baseUrl: baseUrl.trim().replace(/\/+$/, ''),
  body: '',
  bodyMode: 'none',
  formFields: [],
  headers: [],
  method: 'GET',
  path: '/',
  queryParams: [],
  rawFormat: 'json',
});
