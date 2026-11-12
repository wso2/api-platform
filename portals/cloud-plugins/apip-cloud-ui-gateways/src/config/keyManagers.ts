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

import type { GatewayKeyManager } from '../types';

/** The platform's limits, checked again here so the dialog says so before a round trip. */
export const MAX_KEY_MANAGERS = 10;
const NAME_PATTERN = /^[A-Za-z0-9][A-Za-z0-9_-]{0,62}$/;
const MAX_ISSUER = 512;
const MAX_JWKS_URI = 2048;

export type KeyManagerErrors = Partial<
  Record<'name' | 'issuer' | 'jwksUri' | 'publicKey', string>
>;

/** An absolute http or https URL. */
export const isHttpUrl = (value: string): boolean => {
  try {
    const url = new URL(value);
    return (url.protocol === 'https:' || url.protocol === 'http:') && url.hostname !== '';
  } catch {
    return false;
  }
};

/** Where an issuer's OpenID discovery document lives by default. */
export const wellKnownUrlOf = (issuer: string): string =>
  `${issuer.trim().replace(/\/+$/, '')}/.well-known/openid-configuration`;

/** A well-known URL as the dialog checks it: required, and a URL. */
export const wellKnownUrlError = (value: string): string | undefined => {
  const url = value.trim();
  if (!url) return 'Required.';
  if (url.length > MAX_JWKS_URI) return `At most ${MAX_JWKS_URI} characters.`;
  return isHttpUrl(url) ? undefined : 'Must be a URL.';
};

/** Whether two lists are the same entries in the same order. */
export const sameKeyManagers = (a: GatewayKeyManager[], b: GatewayKeyManager[]): boolean =>
  a.length === b.length &&
  a.every((km, i) => {
    const other = b[i];
    return (
      km.name === other.name &&
      km.issuer === other.issuer &&
      (km.jwksUri ?? '') === (other.jwksUri ?? '') &&
      (km.publicKey ?? '') === (other.publicKey ?? '')
    );
  });

/**
 * One entry against the rest of the list. `others` are the entries it must not
 * collide with; `knownUris` are JWKS URLs the gateway already trusts, which keep
 * their scheme (the environment's ThunderID is http on a local cluster) — the
 * platform makes the same exception for its own.
 */
export function validateKeyManager(
  km: GatewayKeyManager,
  others: GatewayKeyManager[],
  knownUris: string[]
): KeyManagerErrors {
  const errors: KeyManagerErrors = {};
  const name = km.name.trim();
  const issuer = km.issuer.trim();
  if (!NAME_PATTERN.test(name)) {
    errors.name =
      'Letters, digits, hyphens and underscores, starting with a letter or digit, at most 63 characters.';
  } else if (others.some((other) => other.name === name)) {
    errors.name = 'Another key manager has this name.';
  }
  if (!issuer) errors.issuer = 'Required.';
  else if (issuer.length > MAX_ISSUER) errors.issuer = `At most ${MAX_ISSUER} characters.`;
  else if (!isHttpUrl(issuer)) errors.issuer = 'Must be a URL.';
  else if (others.some((other) => other.issuer === issuer)) {
    errors.issuer = 'Another key manager has this issuer.';
  }
  const uri = km.jwksUri?.trim() ?? '';
  const publicKey = km.publicKey?.trim() ?? '';
  if (km.jwksUri !== undefined) {
    if (!uri) errors.jwksUri = 'Required.';
    else if (uri.length > MAX_JWKS_URI) errors.jwksUri = `At most ${MAX_JWKS_URI} characters.`;
    else if (!knownUris.includes(uri) && !(isHttpUrl(uri) && uri.startsWith('https://'))) {
      errors.jwksUri = 'Must be an https URL.';
    }
  } else if (!publicKey) {
    errors.publicKey = 'Required.';
  } else if (!/^-----BEGIN (PUBLIC KEY|CERTIFICATE)-----/.test(publicKey)) {
    errors.publicKey = 'Must be a PEM public key or certificate.';
  }
  return errors;
}

/** An entry as it is sent: trimmed, and with only the key source it uses. */
export const normalizeKeyManager = (km: GatewayKeyManager): GatewayKeyManager => {
  const out: GatewayKeyManager = { name: km.name.trim(), issuer: km.issuer.trim() };
  if (km.jwksUri !== undefined) out.jwksUri = km.jwksUri.trim();
  else out.publicKey = km.publicKey?.trim() ?? '';
  return out;
};
