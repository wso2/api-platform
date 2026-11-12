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
 * Checks that keep a console request pointed where the console says it goes.
 *
 * The console builds every request from three trusted-by-construction pieces —
 * the gateway's invoke URL, an operation's path template, and parameter values
 * that are percent-encoded as single segments. These checks are the proof that
 * nothing in a hostile OpenAPI document, and nothing a user pastes into a field,
 * can turn that into a request to another host, another path prefix, or one
 * with an injected header. They run in the console *and* again in each
 * transport, so a transport is safe even if it is reused elsewhere.
 */

/** Characters allowed in an HTTP header name (RFC 9110 `token`). */
const HEADER_NAME = /^[!#$%&'*+.^_`|~0-9A-Za-z-]+$/;

/**
 * Request headers the browser owns, or that would let a caller steer the
 * request away from the API under test. Refused in both transport modes: the
 * browser drops most of these silently, and the relay must never be asked to
 * forward them.
 */
const FORBIDDEN_HEADER_NAMES = new Set([
  'accept-charset',
  'accept-encoding',
  'access-control-request-headers',
  'access-control-request-method',
  'connection',
  'content-length',
  'cookie',
  'cookie2',
  'date',
  'dnt',
  'expect',
  'host',
  'keep-alive',
  'origin',
  'referer',
  'te',
  'trailer',
  'transfer-encoding',
  'upgrade',
  'via',
  'x-forwarded-for',
  'x-forwarded-host',
  'x-forwarded-proto',
  'x-http-method-override',
  'x-http-method',
  'x-method-override',
]);

export type HeaderProblem = 'name' | 'forbidden' | 'value';

/** `undefined` when the header may be sent, otherwise why not. */
export const headerProblem = (name: string, value: string): HeaderProblem | undefined => {
  const lower = name.trim().toLowerCase();
  if (!HEADER_NAME.test(name.trim())) return 'name';
  if (
    FORBIDDEN_HEADER_NAMES.has(lower) ||
    lower.startsWith('proxy-') ||
    lower.startsWith('sec-')
  ) {
    return 'forbidden';
  }
  // CR, LF and NUL are how a header value becomes a second header.
  if (/[\r\n\0]/.test(value)) return 'value';
  return undefined;
};

/** Percent-encodes one path segment, including the characters `encodeURIComponent` leaves. */
export const encodePathValue = (value: string): string =>
  encodeURIComponent(value).replace(
    /[!'()*]/g,
    (char) => `%${char.charCodeAt(0).toString(16).toUpperCase()}`,
  );

/**
 * Whether an operation's path template may be appended to the gateway URL.
 *
 * Rejects what could change the request's destination or confuse a proxy's
 * path matching: an absolute or protocol-relative form, dot segments (raw or
 * percent-encoded), backslashes, control characters, and a query or fragment
 * smuggled into the path.
 */
export const isSafePath = (path: string): boolean => {
  if (!path.startsWith('/') || path.startsWith('//')) return false;
  // eslint-disable-next-line no-control-regex
  if (/[\\?#\u0000-\u001f\u007f]/.test(path)) return false;
  return path.split('/').every((segment) => {
    const decoded = segment.replace(/%2e/gi, '.');
    return decoded !== '.' && decoded !== '..';
  });
};

/** Joins the gateway URL and a path, refusing anything that would leave the gateway. */
export const resolveTarget = (baseUrl: string, path: string): URL | undefined => {
  if (!isSafePath(path)) return undefined;
  let base: URL;
  try {
    base = new URL(baseUrl);
  } catch {
    return undefined;
  }
  if (base.protocol !== 'http:' && base.protocol !== 'https:') return undefined;
  // A URL with credentials would put them in the clear in every request line.
  if (base.username !== '' || base.password !== '') return undefined;

  const prefix = base.pathname.replace(/\/+$/, '');
  const target = new URL(`${base.origin}${prefix}${path}`);

  // The final proof, whatever the pieces were: same origin, still under the base path.
  if (target.origin !== base.origin) return undefined;
  if (prefix !== '' && target.pathname !== prefix && !target.pathname.startsWith(`${prefix}/`)) {
    return undefined;
  }
  return target;
};
