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

import type { Path } from './types';

const ACRONYMS = new Set([
  'id',
  'ids',
  'url',
  'urls',
  'uri',
  'uris',
  'tls',
  'ca',
  'aws',
  'arn',
  'jwt',
  'jwks',
  'ttl',
  'http',
  'https',
  'api',
  'ip',
  'sts',
  'iam',
  'json',
  'xml',
  'cel',
  'pii',
  'ssn',
  'mcp',
  'llm',
  'hmac',
  'opa',
  'sql',
  'cors',
  'sse',
  'ssl',
  'db',
  'pem',
  'kid',
  'iss',
  'aud',
  'sub',
  'rpm',
  'tpm',
  'mtls',
  'sni',
  'dns',
  'cidr',
  'ms',
  'oidc',
  'saml',
  'crl',
  'ocsp',
  'gcp',
  'uuid',
  'regex',
  'nlp',
  'ai',
  'sdk',
  'cpu',
  'tcp',
  'udp',
]);
const SPECIAL: Record<string, string> = {
  oauth2: 'OAuth2',
  oauth: 'OAuth',
  openai: 'OpenAI',
  websub: 'WebSub',
  redis: 'Redis',
  regex: 'regex',
};

/** "tokenEndpoint" → "Token endpoint", "awsAccessKeyID" → "AWS access key ID", "proxyURL" → "Proxy URL" */
export function humanize(key: string): string {
  if (!key) return '';
  const words = key
    .replace(/[_\-.]+/g, ' ')
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .replace(/([A-Z]+)([A-Z][a-z])/g, '$1 $2')
    .trim()
    .split(/\s+/);
  return words
    .map((w, i) => {
      const lower = w.toLowerCase();
      if (SPECIAL[lower] && lower !== 'regex') return SPECIAL[lower];
      if (ACRONYMS.has(lower) && lower !== 'regex')
        return lower === 'ids' ? 'IDs' : lower.toUpperCase();
      return i === 0 ? lower.charAt(0).toUpperCase() + lower.slice(1) : lower;
    })
    .join(' ');
}

export function pathKey(path: Path): string {
  let out = '';
  for (const p of path) {
    if (typeof p === 'number') out += `[${p}]`;
    else out += out ? `.${p}` : p;
  }
  return out;
}

/**
 * A collision-free key for a config path. `pathKey` joins segments with `.`, so the map
 * entries `a` -> `b` and `a.b` both read `m.a.b`; encoding the segments keeps them apart.
 */
export function configKey(path: Path): string {
  return JSON.stringify(path);
}

/**
 * Sets an own property even for a key like `__proto__`, which a plain assignment would turn
 * into a prototype change and so drop from the object.
 */
export function setOwn(target: Record<string, unknown>, key: string, value: unknown): void {
  Object.defineProperty(target, key, {
    configurable: true,
    enumerable: true,
    value,
    writable: true,
  });
}

export function typeOf(v: unknown): string {
  if (v === null) return 'null';
  if (Array.isArray(v)) return 'array';
  if (typeof v === 'number') return Number.isInteger(v) ? 'integer' : 'number';
  return typeof v;
}

export function deepEqual(a: unknown, b: unknown): boolean {
  if (a === b) return true;
  if (typeof a !== typeof b || a === null || b === null || typeof a !== 'object') return false;
  if (Array.isArray(a) !== Array.isArray(b)) return false;
  if (Array.isArray(a)) {
    const bb = b as unknown[];
    return a.length === bb.length && a.every((x, i) => deepEqual(x, bb[i]));
  }
  const ka = Object.keys(a as object);
  const kb = Object.keys(b as object);
  return (
    ka.length === kb.length &&
    ka.every((k) => deepEqual((a as Record<string, unknown>)[k], (b as Record<string, unknown>)[k]))
  );
}

export function clone<T>(v: T): T {
  return v === undefined ? v : JSON.parse(JSON.stringify(v));
}

export function isEmpty(v: unknown): boolean {
  return v === undefined || v === null || (typeof v === 'string' && v.trim() === '');
}

export function formatValue(v: unknown): string {
  if (typeof v === 'string') return v;
  return JSON.stringify(v);
}

type Container = Record<string | number, unknown>;

/** Immutable set: creates arrays for numeric segments and objects for string segments. */
export function setIn(root: unknown, path: Path, value: unknown): unknown {
  if (path.length === 0) return value;
  const [head, ...rest] = path;
  const container = root ?? (typeof head === 'number' ? [] : {});
  const copy = (
    Array.isArray(container) ? container.slice() : { ...(container as Container) }
  ) as Container;
  const next = setIn(copy[head], rest, value);
  if (next === undefined && !Array.isArray(copy)) delete copy[head];
  else copy[head] = next;
  return copy;
}

export function getIn(root: unknown, path: Path): unknown {
  let cur = root;
  for (const p of path) {
    if (cur == null) return undefined;
    cur = (cur as Container)[p];
  }
  return cur;
}

/** Keys that look like credentials. Matches the end of the key so "tokenEndpoint" is not a secret. */
export const SECRET_KEY =
  /(secret|password|passwd|bearertoken|apikey|privatekey|secretaccesskey|clientsecret|accesstoken|sessiontoken)$/i;
/** Keys whose text is usually long or multi-line. */
export const MULTILINE_KEY =
  /(template|prompt|instructions|schema|inline|cert|certificate|pem|expression|body|policy|systemprompt|payload|script|rego)$/i;
