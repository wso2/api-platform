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

import { http, type RequestOptions } from '../../core/http';
import type { ResponseOf, Schema } from '../../core/spec';

/**
 * Transport layer for the `/api-portals` org-level list. Kept as its own package
 * (rather than folded into apiPublications) because the two answer different
 * questions: publications is an API-scoped rollup annotated per portal, while
 * this is a plain list of the org's registered portals - used, e.g., for the
 * portal count on the org overview card.
 */

export type ApiPortal = Schema<'ApiPortal'>;
export type ListApiPortalsResponse = ResponseOf<'ListApiPortals'>;

const API_PORTALS_BASE = '/api-portals';

export const listApiPortals = async (
  options?: RequestOptions,
): Promise<ListApiPortalsResponse> =>
  http.get<ListApiPortalsResponse>(API_PORTALS_BASE, {
    ...options,
    operationName: 'ListApiPortals',
  });
