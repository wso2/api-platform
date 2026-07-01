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

import type { ReactNode } from 'react';
import type { MappedAPIKey, UserAPIKey } from '../../../../../utils/types';

export const ROWS_PER_PAGE_OPTIONS = [5, 10, 25];

export type DrawerEntity = {
  id: string;
  displayName: string;
  description?: string;
};

export type SelectionDrawerItemMeta = {
  chip?: ReactNode;
  emptyKeysText: string;
};

export function getInitials(name: string): string {
  const words = name.trim().split(/\s+/);
  if (words.length === 0) return '';
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
  return `${words[0][0]}${words[1][0]}`.toUpperCase();
}

export function formatDate(value?: string): string {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '—';
  return date.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });
}

export function getKeyStatusColor(
  status?: string
): 'success' | 'warning' | 'error' | 'default' {
  const normalizedStatus = (status ?? '').toLowerCase();
  if (normalizedStatus === 'active') return 'success';
  if (normalizedStatus === 'pending') return 'warning';
  if (
    normalizedStatus === 'inactive' ||
    normalizedStatus === 'expired' ||
    normalizedStatus === 'revoked'
  ) {
    return 'error';
  }
  return 'default';
}

export function dedupeMappedKeys(keys: MappedAPIKey[]): MappedAPIKey[] {
  const seen = new Set<string>();

  return keys.filter((key) => {
    const keyId = key.keyId || '';
    if (!keyId || seen.has(keyId)) return false;
    seen.add(keyId);
    return true;
  });
}

export function resolveMappedKeyId(key: MappedAPIKey): string {
  const candidate = key as MappedAPIKey & { mappedKeyId?: string };
  return candidate.mappedKeyId || key.keyId;
}

export function getVisibleKeys(
  entityKeys: UserAPIKey[],
  mappedKeys: MappedAPIKey[],
  includeMappedOnlyKeys: boolean
): UserAPIKey[] {
  if (!includeMappedOnlyKeys) return entityKeys;

  const mappedOnlyKeys: UserAPIKey[] = mappedKeys
    .filter(
      (mappedKey) =>
        !entityKeys.some(
          (entityKey) => (entityKey.name ?? '') === mappedKey.keyId
        )
    )
    .map((mappedKey) => ({
      name: mappedKey.keyId,
      status: mappedKey.status,
    }));

  return [...entityKeys, ...mappedOnlyKeys];
}
