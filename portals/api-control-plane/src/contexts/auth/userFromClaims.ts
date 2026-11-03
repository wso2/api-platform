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

import type { ClaimMappings } from '../../config/authContract';
import type { AuthUser } from './authTypes';

/**
 * Resolves the display user from the signed-in user's claims, for a session
 * endpoint that answers with claims rather than a ready user (the `claims` and
 * `sdk` session formats; in `sdk` the provider then takes the scopes from the
 * session's own `scope` field instead).
 *
 * This is the BFF's `UserFromClaims` (bff/internal/session/claims.go) in
 * scope mode, rule for rule, so the same claims and settings give the same
 * user on both sides. Both are run against the shared cases in
 * `testdata/claim-mapping-cases.json`; change the two together. The BFF's
 * role mode, which turns roles into scopes through a mapping file, is not
 * supported here: scopes come from the scope claim.
 */

type Claims = Record<string, unknown>;

const isObject = (value: unknown): value is Claims =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

/** Walks a dotted path ("realm_access.roles") through nested claim objects. */
const resolveClaimPath = (claims: Claims, path: string): unknown => {
  if (!path) return undefined;
  let current: unknown = claims;
  for (const segment of path.split('.')) {
    if (!isObject(current) || !Object.prototype.hasOwnProperty.call(current, segment)) {
      return undefined;
    }
    current = current[segment];
  }
  return current;
};

const strClaim = (claims: Claims, path: string) => {
  const value = resolveClaimPath(claims, path);
  return typeof value === 'string' ? value : '';
};

const splitFields = (value: string) => value.split(/\s+/).filter(Boolean);

/** A claim that may be one string, a space-delimited string, or an array of strings. */
const strSliceClaim = (claims: Claims, path: string): string[] => {
  const value = resolveClaimPath(claims, path);
  if (typeof value === 'string') return splitFields(value);
  if (Array.isArray(value)) {
    return value.filter((item): item is string => typeof item === 'string' && item !== '');
  }
  return [];
};

/**
 * Whether the claims carry a scope claim at all: at the configured path, or
 * the flat "scp" claim. An id_token, for one, has none.
 */
export const hasScopeClaim = (claims: Claims, path: string): boolean => {
  const raw = resolveClaimPath(claims, path) ?? claims.scp;
  return raw !== undefined && raw !== null;
};

/** The scope claim at the configured path, falling back to the flat "scp" claim. */
const scopes = (claims: Claims, path: string): string[] => {
  let raw = resolveClaimPath(claims, path);
  if (raw === undefined || raw === null) raw = claims.scp;
  if (typeof raw === 'string') return splitFields(raw);
  if (Array.isArray(raw)) return raw.filter((item): item is string => typeof item === 'string');
  return [];
};

const first = (...values: string[]) => values.find((value) => value !== '') ?? '';

export function userFromClaims(claims: Claims, mappings: ClaimMappings): AuthUser {
  const roleList = strSliceClaim(claims, mappings.roles);
  const user: AuthUser = {
    name: first(
      strClaim(claims, mappings.username),
      strClaim(claims, mappings.email),
      strClaim(claims, 'sub'),
    ),
    email: strClaim(claims, mappings.email),
    scopes: scopes(claims, mappings.scope),
  };

  const role = roleList.join(' ');
  if (role) user.role = role;

  const picture = strClaim(claims, mappings.picture);
  if (picture) user.picture = picture;

  const orgId = strClaim(claims, mappings.organization);
  const orgName = strClaim(claims, mappings.org_name);
  const orgHandle = strClaim(claims, mappings.org_handle);
  if (orgId || orgHandle) {
    user.org = { id: orgId, name: orgName || orgHandle, handle: orgHandle };
  }
  return user;
}
