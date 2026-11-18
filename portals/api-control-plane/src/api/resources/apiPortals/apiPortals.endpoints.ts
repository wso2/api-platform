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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { http, type RequestOptions } from '../../core/http';
import type { BodyOf, PathOf, QueryOf, ResponseOf, Schema } from '../../core/spec';

/**
 * Transport layer for `/api-portals`, the org's registry of API Portals. Kept
 * as its own package (rather than folded into apiPublications) because the two
 * answer different questions: publications is an API-scoped rollup annotated
 * per portal, while this is the registry itself - the list, plus registering,
 * editing and removing a portal.
 *
 * The shared key platform-api sends to a portal is write-only: it goes out on
 * create and update and never comes back in a response.
 */

export type ApiPortal = Schema<'ApiPortalResponse'>;
export type ApiPortalListItem = Schema<'ApiPortalListItem'>;
export type ListApiPortalsResponse = ResponseOf<'ListApiPortals'>;
export type ListApiPortalsQuery = QueryOf<'ListApiPortals'>;
export type CreateApiPortalBody = BodyOf<'CreateApiPortal'>;
export type UpdateApiPortalBody = BodyOf<'UpdateApiPortal'>;

const API_PORTALS_BASE = '/api-portals';

/** URL-encoded path for one portal. Handles are user-supplied - always encode. */
const resourcePath = (apiPortalId: PathOf<'GetApiPortal'>['apiPortalId']): string =>
  `${API_PORTALS_BASE}/${encodeURIComponent(apiPortalId)}`;

/**
 * GETs `/api-portals` for the caller's org. Paging, sort and search flow
 * through `options.query` (typed at the queries layer via `ListApiPortalsQuery`);
 * without them, the server returns its default first page.
 */
export const listApiPortals = async (
  options?: RequestOptions,
): Promise<ListApiPortalsResponse> =>
  http.get<ListApiPortalsResponse>(API_PORTALS_BASE, {
    ...options,
    operationName: 'ListApiPortals',
  });

export const getApiPortal = async (
  apiPortalId: string,
  options?: RequestOptions,
): Promise<ApiPortal> =>
  http.get<ApiPortal>(resourcePath(apiPortalId), {
    ...options,
    operationName: 'GetApiPortal',
  });

export const createApiPortal = async (
  body: CreateApiPortalBody,
  options?: RequestOptions,
): Promise<ApiPortal> =>
  http.post<ApiPortal>(API_PORTALS_BASE, body, {
    ...options,
    operationName: 'CreateApiPortal',
  });

/**
 * PUTs a partial update. A `sharedKey` in the body replaces the stored key, so
 * the same call rotates it; omitting the field leaves the key unchanged.
 */
export const updateApiPortal = async (
  apiPortalId: string,
  body: UpdateApiPortalBody,
  options?: RequestOptions,
): Promise<ApiPortal> =>
  http.put<ApiPortal>(resourcePath(apiPortalId), body, {
    ...options,
    operationName: 'UpdateApiPortal',
  });

export const deleteApiPortal = async (
  apiPortalId: string,
  options?: RequestOptions,
): Promise<void> => {
  await http.delete<void>(resourcePath(apiPortalId), {
    ...options,
    operationName: 'DeleteApiPortal',
  });
};
