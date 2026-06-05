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

export function readOrganizationFlagMap(
  storageKey: string
): Record<string, boolean> {
  try {
    const stored = localStorage.getItem(storageKey);
    if (!stored) return {};

    const parsed = JSON.parse(stored) as unknown;
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return {};
    }

    return parsed as Record<string, boolean>;
  } catch {
    return {};
  }
}

export function writeOrganizationFlag(
  storageKey: string,
  organizationId: string
): void {
  if (!organizationId) return;
  try {
    const map = readOrganizationFlagMap(storageKey);
    map[organizationId] = true;
    localStorage.setItem(storageKey, JSON.stringify(map));
  } catch {
    // Ignore localStorage write failures.
  }
}

export function getLatestTimestamp(...values: Array<string | undefined>): number {
  for (const value of values) {
    if (!value) continue;
    const timestamp = new Date(value).getTime();
    if (Number.isFinite(timestamp)) return timestamp;
  }
  return 0;
}

export function sortByLatest<T>(
  items: T[],
  dateExtractor: (item: T) => Array<string | undefined>
): T[] {
  return items
    .map((item, index) => ({ item, index }))
    .sort((a, b) => {
      const timeDifference =
        getLatestTimestamp(...dateExtractor(b.item)) -
        getLatestTimestamp(...dateExtractor(a.item));
      if (timeDifference !== 0) return timeDifference;
      return a.index - b.index;
    })
    .map(({ item }) => item);
}

