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

import { queryOptions } from '@tanstack/react-query';

import { staleTimes } from '../../core/queryClient';
import { createResourceKeys, type OrgScope } from '../../core/queryKeys';
import { getApiThumbnail } from './apiThumbnail.endpoints';

/**
 * Thumbnails are a singleton per artifact — one row keyed on the API's
 * (type, handle). Caching the fetched Blob under this key means `<img>` tags
 * on the API detail page and the API listing share one network call per API,
 * and a PUT/DELETE invalidation refreshes every live `<img>` at once.
 */
export const apiThumbnailKeys = createResourceKeys('apiThumbnail');

export const apiThumbnailParentId = (apiType: string, apiId: string): string =>
  `${apiType}/${apiId}`;

export const apiThumbnailQueries = {
  /** The stored thumbnail Blob + sniffed content type. 204 No Content is the normal "not set" state. */
  blob: (org: OrgScope, apiType: string, apiId: string) =>
    queryOptions({
      queryKey: apiThumbnailKeys.child(org, apiThumbnailParentId(apiType, apiId), 'blob'),
      queryFn: ({ signal }) => getApiThumbnail(apiType, apiId, { orgId: org, signal }),
      staleTime: staleTimes.standard,
      // A missing thumbnail is a steady state, not a transient failure — one
      // 204 doesn't mean the next request will succeed.
      retry: false,
    }),
};
