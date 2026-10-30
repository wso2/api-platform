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
 * Pure helpers behind registering an API Portal and wiring its shared key.
 *
 * The control plane authenticates to a portal with a shared key it stores; the
 * portal holds only the key's SHA-256 hash. So the console generates the key in
 * the browser, sends it once, and shows the operator nothing but the hash to
 * put in the portal's configuration.
 */

/** Matches the spec's `handle` limit; the handle becomes the portal's id. */
export const HANDLE_MAX_LENGTH = 40;

export const HANDLE_PATTERN = /^[a-z0-9-]+$/;

/** Shown in the config snippet until a key exists in this session. */
export const HASH_PLACEHOLDER = '<shared-key-hash>';

/** Name to URL-friendly handle, as the gateways page derives its own. */
export const toHandle = (name: string): string =>
  name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, HANDLE_MAX_LENGTH)
    .replace(/-+$/, '');

/** The spec only accepts https portal URLs. */
export const isHttpsUrl = (value: string): boolean => {
  try {
    return new URL(value).protocol === 'https:';
  } catch {
    return false;
  }
};

const toHex = (bytes: Uint8Array): string =>
  Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('');

/** A fresh 256-bit key as 64 hex characters, the format the spec requires. */
export const generateSharedKey = (): string => {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);
  return toHex(bytes);
};

/**
 * SHA-256 of the key's text, hex encoded: the same value the quickstart's
 * `setup.sh` writes for the portal (`printf '%s' "$raw" | openssl dgst -sha256`).
 */
export const sharedKeyHash = async (sharedKey: string): Promise<string> => {
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(sharedKey));
  return toHex(new Uint8Array(digest));
};

/** The portal's `config.toml` entry that carries the hash. */
export const internalAuthConfig = (hash: string): string =>
  `[api_portal.internal_auth]\nhash = "${hash}"`;
