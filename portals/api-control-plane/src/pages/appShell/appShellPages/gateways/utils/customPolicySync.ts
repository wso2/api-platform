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
 * Sync status of a gateway's custom policy against the organization, which
 * holds one record per name and major version (as `SyncCustomPolicy` enforces).
 */
export type CustomPolicySyncStatus =
  /** No record of this major. */
  | { state: 'not-synced' }
  /** The organization holds a lower minor of this major. */
  | { state: 'update-available'; organizationVersion: string }
  /** The organization holds this minor or a higher one. */
  | { state: 'synced'; organizationVersion: string };

type PolicyVersion = { major: number; minor: number; patch: number };

type PolicyRef = { name: string; version: string };

/** Parses `[v]MAJOR.MINOR.PATCH`, the only form the server syncs. */
export const parsePolicyVersion = (version: string): PolicyVersion | undefined => {
  const match = /^v?(\d+)\.(\d+)\.(\d+)$/i.exec(version.trim());
  if (!match) return undefined;
  return { major: Number(match[1]), minor: Number(match[2]), patch: Number(match[3]) };
};

const normalizedName = (name: string) => name.trim().toLowerCase();

export const customPolicySyncStatus = (
  reported: PolicyRef,
  organizationPolicies: readonly PolicyRef[],
): CustomPolicySyncStatus => {
  const name = normalizedName(reported.name);
  const sameName = organizationPolicies.filter(
    (candidate) => normalizedName(candidate.name) === name,
  );

  const version = parsePolicyVersion(reported.version);
  if (!version) {
    const identical = sameName.find((candidate) => candidate.version === reported.version);
    return identical
      ? { state: 'synced', organizationVersion: identical.version }
      : { state: 'not-synced' };
  }

  const held = sameName
    .map((candidate) => ({ candidate, parsed: parsePolicyVersion(candidate.version) }))
    .find(({ parsed }) => parsed?.major === version.major);
  if (!held?.parsed) return { state: 'not-synced' };

  return version.minor > held.parsed.minor
    ? { state: 'update-available', organizationVersion: held.candidate.version }
    : { state: 'synced', organizationVersion: held.candidate.version };
};
