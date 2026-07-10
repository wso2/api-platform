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

import type { ApiListResponse } from './types';

/**
 * Largest `limit` the platform API accepts on a collection GET. Requests above
 * this are clamped server-side rather than rejected, so asking for more only
 * wastes the round trip.
 */
export const MAX_PAGE_LIMIT = 100;

/**
 * Upper bound on pages walked by {@link fetchAllPages}, so a server that keeps
 * reporting a `total` it never delivers cannot spin this loop forever.
 */
const MAX_PAGES = 100;

/**
 * Collect every page of a paginated collection endpoint into a single response.
 *
 * Collection GETs default to `limit=20` when the caller omits it, so a plain
 * unpaged call silently truncates. Use this for lists the UI renders in full
 * (with no pager of its own) and that are known to be small and bounded.
 *
 * The returned `pagination.total` is the server's total; `count` and `limit`
 * describe the merged result.
 */
export async function fetchAllPages<T>(
  fetchPage: (limit: number, offset: number) => Promise<ApiListResponse<T>>
): Promise<ApiListResponse<T>> {
  const all: T[] = [];
  let offset = 0;
  let total = 0;

  for (let page = 0; page < MAX_PAGES; page++) {
    const response = await fetchPage(MAX_PAGE_LIMIT, offset);
    const items = response?.list ?? [];
    total = response?.pagination?.total ?? items.length;

    all.push(...items);

    // A short page means the server has nothing further to give, regardless of
    // what `total` claims — this is the loop's real termination condition.
    if (items.length < MAX_PAGE_LIMIT || all.length >= total) {
      break;
    }
    offset += items.length;
  }

  return {
    count: all.length,
    list: all,
    pagination: { total, offset: 0, limit: all.length },
  };
}
